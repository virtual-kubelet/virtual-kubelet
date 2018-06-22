// Copyright 2016 VMware, Inc. All Rights Reserved.
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

package portmap

import (
	"fmt"
	"net"
	"strconv"
	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/libnetwork/iptables"
	//viccontainer "github.com/vmware/vic/lib/apiservers/engine/backends/container"
)

type Operation int

const (
	Map Operation = iota
	Unmap
)

func (o Operation) String() string {
	switch o {
	case Map:
		return "Map"

	case Unmap:
		return "Unmap"
	}

	return "Unknown"
}

type PortMapper interface {
	MapPort(ip net.IP, port int, proto string, destIP string, destPort int, srcIface, destIface string) error
	UnmapPort(ip net.IP, port int, proto string, destPort int, srcIface, destIface string) error
}

type bindKey struct {
	ip   string
	port int
}

type portMapper struct {
	sync.Mutex

	bindings map[bindKey][][]string
}

func NewPortMapper() PortMapper {
	return &portMapper{bindings: make(map[bindKey][][]string)}
}

func (p *portMapper) isPortAvailable(proto string, ip net.IP, port int) bool {
	addr := ""
	if ip != nil && !ip.IsUnspecified() {
		addr = ip.String()
	}

	if _, ok := p.bindings[bindKey{addr, port}]; ok {
		return false
	}

	c, err := net.Dial(proto, net.JoinHostPort(addr, strconv.Itoa(port)))
	defer func() {
		if c != nil {
			// #nosec: Errors unhandled.
			c.Close()
		}
	}()

	if err != nil {
		return true
	}

	return false
}

func (p *portMapper) MapPort(ip net.IP, port int, proto string, destIP string, destPort int, srcIface, destIface string) error {
	p.Lock()
	defer p.Unlock()

	// check if port is available
	if !p.isPortAvailable(proto, ip, port) {
		return fmt.Errorf("port %d is not available", port)
	}

	if port <= 0 {
		return fmt.Errorf("source port must be specified")
	}

	if destPort <= 0 {
		log.Infof("destination port not specified, using source port %d", port)
		destPort = port
	}

	return p.forward(Map, ip, port, proto, destIP, destPort, srcIface, destIface)
}

func (p *portMapper) UnmapPort(ip net.IP, port int, proto string, destPort int, srcIface, destIface string) error {
	p.Lock()
	defer p.Unlock()

	if port <= 0 {
		return fmt.Errorf("source port must be specified")
	}

	if destPort <= 0 {
		log.Infof("destination port not specified, using source port %d", port)
		destPort = port
	}

	return p.forward(Unmap, ip, port, proto, "", destPort, srcIface, destIface)
}

// iptablesRunAndCheck runs an iptables command with the provided args
func iptablesRunAndCheck(action iptables.Action, args []string) error {
	args = append([]string{string(action)}, args...)
	if output, err := iptables.Raw(args...); err != nil {
		return err
	} else if len(output) != 0 {
		return iptables.ChainError{Chain: "FORWARD", Output: output}
	}
	return nil
}

// iptablesDelete takes the saved args from the Append operation
// and uses them to delete the previously added rules
func iptablesDelete(args [][]string) error {
	var errs []error
	for _, cmd := range args {
		if err := iptablesRunAndCheck(iptables.Delete, cmd); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("Failed to delete iptables rules: %s", errs)
	}
	return nil
}

// adapted from https://github.com/docker/libnetwork/blob/master/iptables/iptables.go
//
// assumes p is locked
func (p *portMapper) forward(op Operation, ip net.IP, port int, proto, destAddr string, destPort int, srcIface, destIface string) error {
	daddr := ip.String()
	if ip == nil || ip.IsUnspecified() {
		// iptables interprets "0.0.0.0" as "0.0.0.0/32", whereas we
		// want "0.0.0.0/0". "0/0" is correctly interpreted as "any
		// value" by both iptables and ip6tables.
		daddr = "0/0"
	}
	ipStr := ""
	if ip != nil && !ip.IsUnspecified() {
		ipStr = ip.String()
	}

	key := bindKey{ip: ipStr, port: port}
	switch op {
	case Unmap:
		// lookup commands to reverse
		if args, ok := p.bindings[key]; ok {
			if err := iptablesDelete(args); err != nil {
				return err
			}
			delete(p.bindings, bindKey{ipStr, port})
			return nil
		}
		return fmt.Errorf("Failed to find unmap data for %s:%d", ipStr, port)

	case Map:
		var savedArgs [][]string

		args := []string{"VIC", "-t", string(iptables.Nat),
			"-i", srcIface,
			"-p", proto,
			"-d", daddr,
			"--dport", strconv.Itoa(port),
			"-j", "DNAT",
			"--to-destination", net.JoinHostPort(destAddr, strconv.Itoa(destPort))}
		if err := iptablesRunAndCheck(iptables.Append, args); err != nil {
			return err
		}
		savedArgs = append(savedArgs, args)
		p.bindings[key] = savedArgs

		// allow traffic from container to container via vch public interface
		args = []string{"VIC", "-t", string(iptables.Nat),
			"-i", destIface,
			"-p", proto,
			"--dport", strconv.Itoa(port),
			"-j", "DNAT",
			"--to-destination", net.JoinHostPort(destAddr, strconv.Itoa(destPort)),
			"-m", "addrtype",
			"--dst-type", "LOCAL"}
		if err := iptablesRunAndCheck(iptables.Append, args); err != nil {
			return err
		}
		savedArgs = append(savedArgs, args)
		p.bindings[key] = savedArgs

		// rule to allow connections from the public interface for
		// the mapped port
		args = []string{"VIC", "-t", string(iptables.Filter),
			"-i", srcIface,
			"-o", destIface,
			"-p", proto,
			"-d", destAddr,
			"--dport", strconv.Itoa(destPort),
			"-j", "ACCEPT"}
		if err := iptablesRunAndCheck(iptables.Append, args); err != nil {
			return err
		}
		savedArgs = append(savedArgs, args)
		p.bindings[key] = savedArgs

		args = []string{"POSTROUTING", "-t", string(iptables.Nat),
			"-p", proto,
			"-d", destAddr,
			"--dport", strconv.Itoa(destPort),
			"-j", "MASQUERADE"}
		if err := iptablesRunAndCheck(iptables.Append, args); err != nil {
			return err
		}
		savedArgs = append(savedArgs, args)
		p.bindings[key] = savedArgs
		return nil

	default:
		log.Warnf("noop for given operation: %s", op)
	}

	return nil
}
