// Copyright 2016-2017 VMware, Inc. All Rights Reserved.
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

package ip

import (
	"bytes"
	"fmt"
	"math"
	"net"
	"strconv"
	"strings"
)

type Range struct {
	FirstIP net.IP `vic:"0.1" scope:"read-only" key:"first"`
	LastIP  net.IP `vic:"0.1" scope:"read-only" key:"last"`
}

func NewRange(first, last net.IP) *Range {
	return &Range{FirstIP: first, LastIP: last}
}

func (i *Range) Overlaps(other Range) bool {
	if (bytes.Compare(i.FirstIP, other.FirstIP) <= 0 && bytes.Compare(other.FirstIP, i.LastIP) <= 0) ||
		(bytes.Compare(i.FirstIP, other.LastIP) <= 0 && bytes.Compare(other.FirstIP, i.LastIP) <= 0) {
		return true
	}

	return false
}

func (i *Range) String() string {
	n := i.Network()
	if n == nil {
		return fmt.Sprintf("%s-%s", i.FirstIP, i.LastIP)
	}

	return n.String()
}

func (i *Range) Equal(other *Range) bool {
	return i.FirstIP.Equal(other.FirstIP) && i.LastIP.Equal(other.LastIP)
}

// Network returns the network that this range represents, if any
func (i *Range) Network() *net.IPNet {
	// only works for ipv4
	first := i.FirstIP.To4()
	last := i.LastIP.To4()
	diff := net.IPv4(0, 0, 0, 0).To4()
	for j := 0; j < net.IPv4len; j++ {
		diff[j] = first[j] ^ last[j]
	}

	var m uint
	for j := net.IPv4len - 1; j >= 0; j-- {
		var k uint
		for ; k < 8; k++ {
			if diff[j]>>k == 0 {
				break
			}
		}

		m += k
		if k < 8 {
			break
		}
	}

	if m == 0 {
		return nil
	}

	mask := net.CIDRMask(32-int(m), 32)
	for j, f := range first {
		l := f | ^mask[j]
		if l != last[j] {
			return nil
		}
	}

	return &net.IPNet{IP: first, Mask: mask}
}

func ParseRange(r string) *Range {
	var first, last net.IP
	// check if its a CIDR
	// #nosec: Errors unhandled
	_, ipnet, _ := net.ParseCIDR(r)
	if ipnet != nil {
		first = ipnet.IP
		last := make(net.IP, len(first))
		for i, f := range first {
			last[i] = f | ^ipnet.Mask[i]
		}

		return &Range{
			FirstIP: first,
			LastIP:  last,
		}
	}

	comps := strings.Split(r, "-")
	if len(comps) != 2 {
		return nil
	}

	first = net.ParseIP(comps[0])
	if first == nil {
		return nil
	}

	last = net.ParseIP(comps[1])
	if last == nil {
		var end int
		end, err := strconv.Atoi(comps[1])
		if err != nil || end <= int(first[15]) || end > math.MaxUint8 {
			return nil
		}

		last = net.IPv4(first[12], first[13], first[14], byte(end))
	}

	if bytes.Compare(first, last) > 0 {
		return nil
	}

	return &Range{
		FirstIP: first,
		LastIP:  last,
	}
}

// MarshalText implements the encoding.TextMarshaler interface
func (i *Range) MarshalText() ([]byte, error) {
	return []byte(i.String()), nil
}

// UmarshalText implements the encoding.TextUnmarshaler interface
func (i *Range) UnmarshalText(text []byte) error {
	s := string(text)
	r := ParseRange(s)
	if r == nil {
		return fmt.Errorf("parse error: %s", s)
	}

	*i = *r
	return nil
}

// ParseIPandMask parses a CIDR format address (e.g. 1.1.1.1/8)
func ParseIPandMask(s string) (net.IPNet, error) {
	var i net.IPNet
	ip, ipnet, err := net.ParseCIDR(s)
	if err != nil {
		return i, err
	}

	i.IP = ip
	i.Mask = ipnet.Mask
	return i, nil
}

// Empty determines if net.IPNet is empty
func Empty(i net.IPNet) bool {
	return i.IP == nil && i.Mask == nil
}

func IsUnspecifiedIP(ip net.IP) bool {
	return len(ip) == 0 || ip.IsUnspecified()
}

func IsUnspecifiedSubnet(n *net.IPNet) bool {
	if n == nil || IsUnspecifiedIP(n.IP) {
		return true
	}

	ones, bits := n.Mask.Size()
	return bits == 0 || ones == 0
}

// AllZerosAddr returns the all-zeros address for a subnet
func AllZerosAddr(subnet *net.IPNet) net.IP {
	return subnet.IP.Mask(subnet.Mask)
}

// AllOnesAddr returns the all-ones address for a subnet
func AllOnesAddr(subnet *net.IPNet) net.IP {
	ones := net.IPv4(0, 0, 0, 0)
	ip := subnet.IP.To16()
	for i := range ip[12:] {
		ones[12+i] = ip[12+i] | ^subnet.Mask[i]
	}

	return ones
}

func IsRoutableIP(ip net.IP, subnet *net.IPNet) bool {
	return subnet.Contains(ip) && !ip.Equal(AllZerosAddr(subnet)) && !ip.Equal(AllOnesAddr(subnet))
}
