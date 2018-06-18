// Copyright 2016-2018 VMware, Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package network

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	"github.com/docker/libnetwork/iptables"
	"github.com/docker/libnetwork/portallocator"
	"github.com/vishvananda/netlink"

	viccontainer "github.com/vmware/vic/lib/apiservers/engine/backends/container"
	"github.com/vmware/vic/lib/apiservers/engine/backends/portmap"
	"github.com/vmware/vic/lib/apiservers/portlayer/models"
	"github.com/vmware/vic/lib/config/executor"
)

const (
	bridgeIfaceName = "bridge"
)

var (
	publicIfaceName = "public"

	portMapper portmap.PortMapper

	// bridge-to-bridge rules, indexed by mapped port;
	// this map is used to delete the rule once
	// the container stops or is removed
	btbRules map[string][]string

	cbpLock         sync.Mutex
	ContainerByPort map[string]string // port:containerID
)

func init() {
	portMapper = portmap.NewPortMapper()
	btbRules = make(map[string][]string)
	ContainerByPort = make(map[string]string)

	l, err := netlink.LinkByName(publicIfaceName)
	if l == nil {
		l, err = netlink.LinkByAlias(publicIfaceName)
		if err != nil {
			log.Errorf("interface %s not found", publicIfaceName)
			return
		}
	}

	// don't use interface alias for iptables rules
	publicIfaceName = l.Attrs().Name
}

// requestHostPort finds a free port on the host
func requestHostPort(proto string) (int, error) {
	pa := portallocator.Get()
	return pa.RequestPortInRange(nil, proto, 0, 0)
}

type portMapping struct {
	intHostPort int
	strHostPort string
	portProto   nat.Port
}

// unrollPortMap processes config for mapping/unmapping ports e.g. from hostconfig.PortBindings
func unrollPortMap(portMap nat.PortMap) ([]*portMapping, error) {
	var portMaps []*portMapping
	for i, pb := range portMap {

		proto, port := nat.SplitProtoPort(string(i))
		nport, err := nat.NewPort(proto, port)
		if err != nil {
			return nil, err
		}

		// iterate over all the ports in pb []nat.PortBinding
		for i := range pb {
			var hostPort int
			var hPort string
			if pb[i].HostPort == "" {
				// use a random port since no host port is specified
				hostPort, err = requestHostPort(proto)
				if err != nil {
					log.Errorf("could not find available port on host")
					return nil, err
				}
				log.Infof("using port %d on the host for port mapping", hostPort)

				// update the hostconfig
				pb[i].HostPort = strconv.Itoa(hostPort)

			} else {
				hostPort, err = strconv.Atoi(pb[i].HostPort)
				if err != nil {
					return nil, err
				}
			}
			hPort = strconv.Itoa(hostPort)
			portMaps = append(portMaps, &portMapping{
				intHostPort: hostPort,
				strHostPort: hPort,
				portProto:   nport,
			})
		}
	}
	return portMaps, nil
}

// MapPorts maps ports defined in bridge endpoint for containerID
func MapPorts(vc *viccontainer.VicContainer, endpoint *models.EndpointConfig, containerID string) error {
	if endpoint == nil {
		return fmt.Errorf("invalid endpoint")
	}

	var containerIP net.IP
	containerIP = net.ParseIP(endpoint.Address)
	if containerIP == nil {
		return fmt.Errorf("invalid endpoint address %s", endpoint.Address)
	}

	portMap := addIndirectEndpointsToPortMap([]*models.EndpointConfig{endpoint}, nil)
	log.Debugf("Mapping ports of %q on endpoint %s: %v", containerID, endpoint.Name, portMap)
	if len(portMap) == 0 {
		return nil
	}

	mappings, err := unrollPortMap(portMap)
	if err != nil {
		return err
	}

	// cannot occur direct under the lock below because unmap ports take a lock.
	defer func() {
		if err != nil {
			// if we didn't succeed then make sure we clean up
			UnmapPorts(containerID, vc)
		}
	}()

	cbpLock.Lock()
	defer cbpLock.Unlock()
	vc.NATMap = portMap

	for _, p := range mappings {
		// update mapped ports
		if ContainerByPort[p.strHostPort] == containerID {
			log.Debugf("Skipping mapping for already mapped port %s for %s", p.strHostPort, containerID)
			continue
		}

		if err = portMapper.MapPort(nil, p.intHostPort, p.portProto.Proto(), containerIP.String(), p.portProto.Int(), publicIfaceName, bridgeIfaceName); err != nil {
			return err
		}

		// bridge-to-bridge pin hole for traffic from containers for exposed port
		if err = interBridgeTraffic(portmap.Map, p.strHostPort, p.portProto.Proto(), containerIP.String(), p.portProto.Port()); err != nil {
			return err
		}

		// update mapped ports
		ContainerByPort[p.strHostPort] = containerID
		log.Debugf("mapped port %s for container %s", p.strHostPort, containerID)
	}
	return nil
}

// UnmapPorts unmaps ports defined in hostconfig if it's mapped for this container
func UnmapPorts(id string, vc *viccontainer.VicContainer) error {
	portMap := vc.NATMap
	log.Debugf("UnmapPorts for %s: %v", vc.ContainerID, portMap)

	if len(portMap) == 0 {
		return nil
	}

	mappings, err := unrollPortMap(vc.NATMap)
	if err != nil {
		return err
	}

	cbpLock.Lock()
	defer cbpLock.Unlock()
	vc.NATMap = nil

	for _, p := range mappings {
		// check if we should actually unmap based on current mappings
		mappedID, mapped := ContainerByPort[p.strHostPort]
		if !mapped {
			log.Debugf("skipping already unmapped %s", p.strHostPort)
			continue
		}
		if mappedID != id {
			log.Debugf("port is mapped for container %s, not %s, skipping", mappedID, id)
			continue
		}

		if err = portMapper.UnmapPort(nil, p.intHostPort, p.portProto.Proto(), p.portProto.Int(), publicIfaceName, bridgeIfaceName); err != nil {
			log.Warnf("failed to unmap port %s: %s", p.strHostPort, err)
			continue
		}

		// bridge-to-bridge pin hole for traffic from containers for exposed port
		if err = interBridgeTraffic(portmap.Unmap, p.strHostPort, "", "", ""); err != nil {
			log.Warnf("failed to undo bridge-to-bridge pinhole %s: %s", p.strHostPort, err)
			continue
		}

		// update mapped ports
		delete(ContainerByPort, p.strHostPort)
		log.Debugf("unmapped port %s", p.strHostPort)
	}
	return nil
}

// interBridgeTraffic enables traffic for exposed port from one bridge network to another
func interBridgeTraffic(op portmap.Operation, hostPort, proto, containerAddr, containerPort string) error {
	switch op {
	case portmap.Map:
		switch proto {
		case "udp", "tcp":
		default:
			return fmt.Errorf("unknown protocol: %s", proto)
		}

		// rule to allow connections from bridge interface for the
		// specific mapped port. has to inserted at the top of the
		// chain rather than appended to supersede bridge-to-bridge
		// traffic blocking
		baseArgs := []string{"-t", string(iptables.Filter),
			"-i", bridgeIfaceName,
			"-o", bridgeIfaceName,
			"-p", proto,
			"-d", containerAddr,
			"--dport", containerPort,
			"-j", "ACCEPT",
		}

		args := append([]string{string(iptables.Insert), "VIC", "1"}, baseArgs...)
		if _, err := iptables.Raw(args...); err != nil && !os.IsExist(err) {
			return err
		}

		btbRules[hostPort] = baseArgs
	case portmap.Unmap:
		if args, ok := btbRules[hostPort]; ok {
			args = append([]string{string(iptables.Delete), "VIC"}, args...)
			if _, err := iptables.Raw(args...); err != nil && !os.IsNotExist(err) {
				return err
			}

			delete(btbRules, hostPort)
		}
	}

	return nil
}

func PublicIPv4Addrs() ([]string, error) {
	l, err := netlink.LinkByName(publicIfaceName)
	if err != nil {
		return nil, fmt.Errorf("could not look up link from interface name %s: %s", publicIfaceName, err.Error())
	}

	addrs, err := netlink.AddrList(l, netlink.FAMILY_V4)
	if err != nil {
		return nil, fmt.Errorf("could not get addresses from public link: %s", err.Error())
	}

	ips := make([]string, len(addrs))
	for i := range addrs {
		ips[i] = addrs[i].IP.String()
	}

	return ips, nil
}

// portMapFromContainer constructs a docker portmap from the container's
// info as returned by the portlayer and adds nil entries for any exposed ports
// that are unmapped
func PortMapFromContainer(vc *viccontainer.VicContainer, t *models.ContainerInfo) nat.PortMap {
	var mappings nat.PortMap

	if t != nil {
		mappings = addDirectEndpointsToPortMap(t.Endpoints, mappings)
	}
	if vc != nil && vc.Config != nil {
		if vc.NATMap != nil {
			// if there's a NAT map for the container then just use that for the indirect port set
			mappings = mergePortMaps(vc.NATMap, mappings)
		} else {
			// if there's no NAT map then we use the backend data every time
			mappings = addIndirectEndpointsToPortMap(t.Endpoints, mappings)
		}
		mappings = addExposedToPortMap(vc.Config, mappings)
	}

	return mappings
}

func ContainerWithPort(hostPort string) (string, bool) {
	cbpLock.Lock()
	mappedCtr, mapped := ContainerByPort[hostPort]
	cbpLock.Unlock()

	return mappedCtr, mapped
}

// mergePortMaps creates a new map containing the union of the two arguments
func mergePortMaps(map1, map2 nat.PortMap) nat.PortMap {
	resultMap := make(map[nat.Port][]nat.PortBinding)
	for k, v := range map1 {
		resultMap[k] = v
	}

	for k, v := range map2 {
		vr := resultMap[k]
		resultMap[k] = append(vr, v...)
	}

	return resultMap
}

// addIndirectEndpointToPortMap constructs a docker portmap from the container's info as returned by the portlayer for those ports that
// require NAT forward on the endpointVM.
// The portMap provided is modified and returned - the return value should always be used.
func addIndirectEndpointsToPortMap(endpoints []*models.EndpointConfig, portMap nat.PortMap) nat.PortMap {
	if len(endpoints) == 0 {
		return portMap
	}

	// will contain a combined set of port mappings
	if portMap == nil {
		portMap = make(nat.PortMap)
	}

	// add IP address into port spec to allow direct usage of data returned by calls such as docker port
	var ip string
	ips, _ := PublicIPv4Addrs()
	if len(ips) > 0 {
		ip = ips[0]
	}

	// Preserve the existing behaviour if we do not have an IP for some reason.
	if ip == "" {
		ip = "0.0.0.0"
	}

	for _, ep := range endpoints {
		if ep.Direct {
			continue
		}

		for _, port := range ep.Ports {
			mappings, err := nat.ParsePortSpec(port)
			if err != nil {
				log.Error(err)
				// just continue if we do have partial port data
			}

			for i := range mappings {
				p := mappings[i].Port
				b := mappings[i].Binding

				if b.HostIP == "" {
					b.HostIP = ip
				}

				if mappings[i].Binding.HostPort == "" {
					// leave this undefined for dynamic assignment
					// TODO: for port stability over VCH restart we would expect to set the dynamically assigned port
					// recorded in containerVM annotations here, so that the old host->port mapping is preserved.
				}

				log.Debugf("Adding indirect mapping for port %v: %v (%s)", p, b, port)

				current, _ := portMap[p]
				portMap[p] = append(current, b)
			}
		}
	}

	return portMap
}

// addDirectEndpointsToPortMap constructs a docker portmap from the container's info as returned by the portlayer for those
// ports exposed directly from the containerVM via container network
// The portMap provided is modified and returned - the return value should always be used.
func addDirectEndpointsToPortMap(endpoints []*models.EndpointConfig, portMap nat.PortMap) nat.PortMap {
	if len(endpoints) == 0 {
		return portMap
	}

	if portMap == nil {
		portMap = make(nat.PortMap)
	}

	for _, ep := range endpoints {
		if !ep.Direct {
			continue
		}

		// add IP address into the port spec to allow direct usage of data returned by calls such as docker port
		var ip string
		rawIP, _, _ := net.ParseCIDR(ep.Address)
		if rawIP != nil {
			ip = rawIP.String()
		}

		if ip == "" {
			ip = "0.0.0.0"
		}

		for _, port := range ep.Ports {
			mappings, err := nat.ParsePortSpec(port)
			if err != nil {
				log.Error(err)
				// just continue if we do have partial port data
			}

			for i := range mappings {
				if mappings[i].Binding.HostIP == "" {
					mappings[i].Binding.HostIP = ip
				}

				if mappings[i].Binding.HostPort == "" {
					// If there's no explicit host port and it's a direct endpoint, then
					// mirror the actual port. It's a bit misleading but we're trying to
					// pack extended function into an existing structure.
					_, p := nat.SplitProtoPort(string(mappings[i].Port))
					mappings[i].Binding.HostPort = p
				}
			}

			for _, mapping := range mappings {
				p := mapping.Port
				current, _ := portMap[p]
				portMap[p] = append(current, mapping.Binding)
			}
		}
	}

	return portMap
}

// addExposedToPortMap ensures that exposed ports are all present in the port map.
// This means nil entries for any exposed ports that are not mapped.
// The portMap provided is modified and returned - the return value should always be used.
func addExposedToPortMap(config *container.Config, portMap nat.PortMap) nat.PortMap {
	if config == nil || len(config.ExposedPorts) == 0 {
		return portMap
	}

	if portMap == nil {
		portMap = make(nat.PortMap)
	}

	for p := range config.ExposedPorts {
		if _, ok := portMap[p]; ok {
			continue
		}

		portMap[p] = nil
	}

	return portMap
}

func DirectPortInformation(t *models.ContainerInfo) []types.Port {
	var resultPorts []types.Port

	for _, ne := range t.Endpoints {
		trust, _ := executor.ParseTrustLevel(ne.Trust)
		if !ne.Direct || trust == executor.Closed || trust == executor.Outbound || trust == executor.Peers {
			// we don't publish port info for ports that are not directly accessible from outside of the VCH
			continue
		}

		ip := strings.SplitN(ne.Address, "/", 2)[0]

		// if it's an open network then inject an "all ports" entry
		if trust == executor.Open {
			resultPorts = append(resultPorts, types.Port{
				IP:          ip,
				PrivatePort: 0,
				PublicPort:  0,
				Type:        "*",
			})
		}

		for _, p := range ne.Ports {
			port := types.Port{IP: ip}

			portsAndType := strings.SplitN(p, "/", 2)
			port.Type = portsAndType[1]

			mapping := strings.Split(portsAndType[0], ":")
			// if no mapping is supplied then there's only one and that's public. If there is a mapping then the first
			// entry is the public
			public, err := strconv.Atoi(mapping[0])
			if err != nil {
				log.Errorf("Got an error trying to convert public port number \"%s\" to an int: %s", mapping[0], err)
				continue
			}
			port.PublicPort = uint16(public)

			// If port is on container network then a different container could be forwarding the same port via the endpoint
			// so must check for explicit ID match. If a match then it's definitely not accessed directly.
			if ContainerByPort[mapping[0]] == t.ContainerConfig.ContainerID {
				continue
			}

			// did not find a way to have the client not render both ports so setting them the same even if there's not
			// redirect occurring
			port.PrivatePort = port.PublicPort

			// for open networks we don't bother listing direct ports
			if len(mapping) == 1 {
				if trust != executor.Open {
					resultPorts = append(resultPorts, port)
				}
				continue
			}

			private, err := strconv.Atoi(mapping[1])
			if err != nil {
				log.Errorf("Got an error trying to convert private port number \"%s\" to an int: %s", mapping[1], err)
				continue
			}
			port.PrivatePort = uint16(private)
			resultPorts = append(resultPorts, port)
		}
	}

	return resultPorts
}

// returns port bindings as a slice of Docker Ports for return to the client
// returns empty slice on error
//func PortForwardingInformation(t *models.ContainerInfo, ips []string) []types.Port {
func PortForwardingInformation(vc *viccontainer.VicContainer, ips []string) []types.Port {
	//cid := t.ContainerConfig.ContainerID
	//c := cache.ContainerCache().GetContainer(cid)

	if vc == nil {
		log.Errorf("Could not get port forwarding info for container")
		return nil
	}

	portBindings := vc.NATMap
	var resultPorts []types.Port

	// create a port for each IP on the interface (usually only 1, but could be more)
	// (works with both IPv4 and IPv6 addresses)
	for _, ip := range ips {
		port := types.Port{IP: ip}

		for portBindingPrivatePort, hostPortBindings := range portBindings {
			proto, pnum := nat.SplitProtoPort(string(portBindingPrivatePort))
			portNum, err := strconv.Atoi(pnum)
			if err != nil {
				log.Warnf("Unable to convert private port %q to an int", pnum)
				continue
			}
			port.PrivatePort = uint16(portNum)
			port.Type = proto

			for i := 0; i < len(hostPortBindings); i++ {
				// If port is on container network then a different container could be forwarding the same port via the endpoint
				// so must check for explicit ID match. If no match, definitely not forwarded via endpoint.
				//if ContainerByPort[hostPortBindings[i].HostPort] != t.ContainerConfig.ContainerID {
				if ContainerByPort[hostPortBindings[i].HostPort] != vc.ContainerID {
					continue
				}

				newport := port
				publicPort, err := strconv.Atoi(hostPortBindings[i].HostPort)
				if err != nil {
					log.Infof("Got an error trying to convert public port number to an int")
					continue
				}

				newport.PublicPort = uint16(publicPort)
				// sanity check -- sometimes these come back as 0 when no binding actually exists
				// that doesn't make sense, so in that case we don't want to report these bindings
				if newport.PublicPort != 0 && newport.PrivatePort != 0 {
					resultPorts = append(resultPorts, newport)
				}
			}
		}
	}
	return resultPorts
}
