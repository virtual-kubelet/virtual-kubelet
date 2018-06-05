// Copyright 2017 VMware, Inc. All Rights Reserved.
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

package common

import (
	"encoding"
	"fmt"
	"net"
	"strings"

	"gopkg.in/urfave/cli.v1"

	"github.com/vmware/vic/lib/config/executor"
	"github.com/vmware/vic/pkg/ip"
	"github.com/vmware/vic/pkg/trace"
)

// CNetworks holds user input from container network flags
type CNetworks struct {
	ContainerNetworks         cli.StringSlice `arg:"container-network"`
	ContainerNetworksGateway  cli.StringSlice `arg:"container-network-gateway"`
	ContainerNetworksIPRanges cli.StringSlice `arg:"container-network-ip-range"`
	ContainerNetworksDNS      cli.StringSlice `arg:"container-network-dns"`
	ContainerNetworksFirewall cli.StringSlice `arg:"container-network-firewall"`
	IsSet                     bool
}

// ContainerNetworks holds container network data after processing
type ContainerNetworks struct {
	MappedNetworks          map[string]string              `cmd:"parent" label:"key-value"`
	MappedNetworksGateways  map[string]net.IPNet           `cmd:"gateway" label:"key-value"`
	MappedNetworksIPRanges  map[string][]ip.Range          `cmd:"ip-range" label:"key-value"`
	MappedNetworksDNS       map[string][]net.IP            `cmd:"dns" label:"key-value"`
	MappedNetworksFirewalls map[string]executor.TrustLevel `cmd:"firewall" label:"key-value"`
}

func (c *ContainerNetworks) IsSet() bool {
	return len(c.MappedNetworks) > 0 ||
		len(c.MappedNetworksGateways) > 0 ||
		len(c.MappedNetworksIPRanges) > 0 ||
		len(c.MappedNetworksDNS) > 0 ||
		len(c.MappedNetworksFirewalls) > 0
}

func (c *CNetworks) CNetworkFlags(hidden bool) []cli.Flag {
	return []cli.Flag{
		cli.StringSliceFlag{
			Name:  "container-network, cn",
			Value: &c.ContainerNetworks,
			Usage: "vSphere network list that containers can use directly with labels, e.g. vsphere-net:backend. Defaults to DCHP - see advanced help (-x).",
		},
		cli.StringSliceFlag{
			Name:   "container-network-gateway, cng",
			Value:  &c.ContainerNetworksGateway,
			Usage:  "Gateway for the container network's subnet in CONTAINER-NETWORK:SUBNET format, e.g. vsphere-net:172.16.0.1/16",
			Hidden: hidden,
		},
		cli.StringSliceFlag{
			Name:   "container-network-ip-range, cnr",
			Value:  &c.ContainerNetworksIPRanges,
			Usage:  "IP range for the container network in CONTAINER-NETWORK:IP-RANGE format, e.g. vsphere-net:172.16.0.0/24, vsphere-net:172.16.0.10-172.16.0.20",
			Hidden: hidden,
		},
		cli.StringSliceFlag{
			Name:   "container-network-dns, cnd",
			Value:  &c.ContainerNetworksDNS,
			Usage:  "DNS servers for the container network in CONTAINER-NETWORK:DNS format, e.g. vsphere-net:8.8.8.8. Ignored if no static IP assigned.",
			Hidden: hidden,
		},
		cli.StringSliceFlag{
			Name:   "container-network-firewall, cnf",
			Value:  &c.ContainerNetworksFirewall,
			Usage:  "Container network trust level in CONTAINER-NETWORK:LEVEL format. Options: Closed, Outbound, Peers, Published, Open.",
			Hidden: hidden,
		},
	}
}

func parseContainerNetworkGateways(cgs []string) (map[string]net.IPNet, error) {
	gws := make(map[string]net.IPNet)
	for _, cg := range cgs {
		m := &ipNetUnmarshaler{}
		vnet, err := parseVnetParam(cg, m)
		if err != nil {
			return nil, err
		}

		if _, ok := gws[vnet]; ok {
			return nil, fmt.Errorf("Duplicate gateway specified for container network %s", vnet)
		}

		gws[vnet] = net.IPNet{IP: m.ip, Mask: m.ipnet.Mask}
	}

	return gws, nil
}

func parseContainerNetworkIPRanges(cps []string) (map[string][]ip.Range, error) {
	pools := make(map[string][]ip.Range)
	for _, cp := range cps {
		ipr := &ip.Range{}
		vnet, err := parseVnetParam(cp, ipr)
		if err != nil {
			return nil, err
		}

		pools[vnet] = append(pools[vnet], *ipr)
	}

	return pools, nil
}

func parseContainerNetworkDNS(cds []string) (map[string][]net.IP, error) {
	dns := make(map[string][]net.IP)
	for _, cd := range cds {
		var ip net.IP
		vnet, err := parseVnetParam(cd, &ip)
		if err != nil {
			return nil, err
		}

		if ip == nil {
			return nil, fmt.Errorf("DNS IP not specified for container network %s", vnet)
		}

		dns[vnet] = append(dns[vnet], ip)
	}

	return dns, nil
}

func parseContainerNetworkFirewalls(cfs []string) (map[string]executor.TrustLevel, error) {
	firewalls := make(map[string]executor.TrustLevel)
	for _, cf := range cfs {
		vnet, value, err := splitVnetParam(cf)
		if err != nil {
			return nil, fmt.Errorf("Error parsing container network parameter %s: %s", cf, err)
		}
		trust, err := executor.ParseTrustLevel(value)
		if err != nil {
			return nil, err
		}
		firewalls[vnet] = trust
	}
	return firewalls, nil
}

func splitVnetParam(p string) (vnet string, value string, err error) {
	mapped := strings.Split(p, ":")
	if len(mapped) == 0 || len(mapped) > 2 {
		err = fmt.Errorf("Invalid value for parameter %s", p)
		return
	}

	vnet = mapped[0]
	if vnet == "" {
		err = fmt.Errorf("Container network not specified in parameter %s", p)
		return
	}

	// If the supplied vSphere network contains spaces then the user must supply a network alias. Guest info won't receive a name with spaces.
	if strings.Contains(vnet, " ") && (len(mapped) == 1 || (len(mapped) == 2 && len(mapped[1]) == 0)) {
		err = fmt.Errorf("A network alias must be supplied when network name %q contains spaces.", p)
		return
	}

	if len(mapped) > 1 {
		// Make sure the alias does not contain spaces
		if strings.Contains(mapped[1], " ") {
			err = fmt.Errorf("The network alias supplied in %q cannot contain spaces.", p)
			return
		}
		value = mapped[1]
	}

	return
}

func parseVnetParam(p string, m encoding.TextUnmarshaler) (vnet string, err error) {
	vnet, v, err := splitVnetParam(p)
	if err != nil {
		return "", fmt.Errorf("Error parsing container network parameter %s: %s", p, err)
	}

	if err = m.UnmarshalText([]byte(v)); err != nil {
		return "", fmt.Errorf("Error parsing container network parameter %s: %s", p, err)
	}

	return vnet, nil
}

type ipNetUnmarshaler struct {
	ipnet *net.IPNet
	ip    net.IP
}

func (m *ipNetUnmarshaler) UnmarshalText(text []byte) error {
	s := string(text)
	ip, ipnet, err := net.ParseCIDR(s)
	if err != nil {
		return err
	}

	m.ipnet = ipnet
	m.ip = ip
	return nil
}

// ProcessContainerNetworks parses container network settings and returns a
// struct containing all processed container network fields on success.
func (c *CNetworks) ProcessContainerNetworks(op trace.Operation) (ContainerNetworks, error) {
	cNetworks := ContainerNetworks{
		MappedNetworks:          make(map[string]string),
		MappedNetworksGateways:  make(map[string]net.IPNet),
		MappedNetworksIPRanges:  make(map[string][]ip.Range),
		MappedNetworksDNS:       make(map[string][]net.IP),
		MappedNetworksFirewalls: make(map[string]executor.TrustLevel),
	}

	if c.ContainerNetworks != nil || c.ContainerNetworksGateway != nil ||
		c.ContainerNetworksIPRanges != nil || c.ContainerNetworksDNS != nil {
		c.IsSet = true
	}

	gws, err := parseContainerNetworkGateways([]string(c.ContainerNetworksGateway))
	if err != nil {
		return cNetworks, cli.NewExitError(err.Error(), 1)
	}

	pools, err := parseContainerNetworkIPRanges([]string(c.ContainerNetworksIPRanges))
	if err != nil {
		return cNetworks, cli.NewExitError(err.Error(), 1)
	}

	dns, err := parseContainerNetworkDNS([]string(c.ContainerNetworksDNS))
	if err != nil {
		return cNetworks, cli.NewExitError(err.Error(), 1)
	}

	firewalls, err := parseContainerNetworkFirewalls([]string(c.ContainerNetworksFirewall))
	if err != nil {
		return cNetworks, cli.NewExitError(err.Error(), 1)
	}

	// Parse container networks
	for _, cn := range c.ContainerNetworks {
		vnet, v, err := splitVnetParam(cn)
		if err != nil {
			return cNetworks, cli.NewExitError(err.Error(), 1)
		}

		alias := vnet
		if v != "" {
			alias = v
		}

		cNetworks.MappedNetworks[alias] = vnet
		cNetworks.MappedNetworksGateways[alias] = gws[vnet]
		cNetworks.MappedNetworksIPRanges[alias] = pools[vnet]
		cNetworks.MappedNetworksDNS[alias] = dns[vnet]
		cNetworks.MappedNetworksFirewalls[alias] = firewalls[vnet]

		delete(gws, vnet)
		delete(pools, vnet)
		delete(dns, vnet)
		delete(firewalls, vnet)
	}

	var hasError bool
	fmtMsg := "The following container network %s is set, but CONTAINER-NETWORK cannot be found. Please check the --container-network and %s settings"
	if len(gws) > 0 {
		op.Errorf(fmtMsg, "gateway", "--container-network-gateway")
		for key, value := range gws {
			mask, _ := value.Mask.Size()
			op.Errorf("\t%s:%s/%d, %q should be a vSphere network name", key, value.IP, mask, key)
		}
		hasError = true
	}
	if len(pools) > 0 {
		op.Errorf(fmtMsg, "ip range", "--container-network-ip-range")
		for key, value := range pools {
			op.Errorf("\t%s:%s, %q should be a vSphere network name", key, value, key)
		}
		hasError = true
	}
	if len(dns) > 0 {
		op.Errorf(fmtMsg, "dns", "--container-network-dns")
		for key, value := range dns {
			op.Errorf("\t%s:%s, %q should be a vSphere network name", key, value, key)
		}
		hasError = true
	}
	if len(firewalls) > 0 {
		op.Errorf(fmtMsg, "firewall", "--container-network-firewall")
		for key := range firewalls {
			op.Errorf("\t%q should be a vSphere network name", key)
		}
		hasError = true
	}
	if hasError {
		return cNetworks, cli.NewExitError("Inconsistent container network configuration.", 1)
	}

	return cNetworks, nil
}
