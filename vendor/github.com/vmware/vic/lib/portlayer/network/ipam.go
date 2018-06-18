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

// IP address management
//
// The API here just concerns itself with tracking blocks of
// IP addresses, as well as individual IPs within the blocks.
// The API does not have a concept of "network", in particular
// when managing CIDR blocks, the network and broadcast address
// are available as valid addresses. This behavior can be
// accomplished, however, by just reserving those two addresses
// first thing after requesting a CIDR address space, by using
// the ReserveIP4() call.

package network

import (
	"bytes"
	"fmt"
	"net"

	log "github.com/Sirupsen/logrus"

	"github.com/vmware/vic/pkg/ip"
)

// An AddressSpace is a collection of
// IP address ranges
type AddressSpace struct {
	Parent          *AddressSpace
	Network         *net.IPNet
	Pool            *ip.Range
	availableRanges []*ip.Range
}

// compareIPv4 compares two IPv4 addresses.
// Returns -1 if ip1 < ip2, 0 if they are equal,
// and 1 if ip1 > ip2
func compareIP4(ip1 net.IP, ip2 net.IP) int {
	ip1 = ip1.To16()
	ip2 = ip2.To16()
	return bytes.Compare(ip1, ip2)
}

func incrementIP4(ip net.IP) net.IP {
	if !isIP4(ip) {
		return nil
	}

	newIP := copyIP(ip)
	s := 0
	if len(ip) == net.IPv6len {
		s = 12
	}
	for i := len(newIP) - 1; i >= s; i-- {
		newIP[i]++
		if newIP[i] > 0 {
			break
		}
	}

	return newIP
}

func decrementIP4(ip net.IP) net.IP {
	if !isIP4(ip) {
		return nil
	}

	newIP := copyIP(ip)
	s := 0
	if len(ip) == net.IPv6len {
		s = 12
	}
	for i := len(newIP) - 1; i >= s; i-- {
		newIP[i]--
		if newIP[i] != 0xff {
			break
		}
	}

	return newIP
}

func copyIP(ip net.IP) net.IP {
	newIP := make([]byte, len(ip))
	copy(newIP, ip)
	return newIP
}

func isIP4(ip net.IP) bool {
	return ip.To4() != nil
}

// lowestIP4 returns the lowest possible IP address
// in an IP network. For example:
//
//     lowestIP4(net.IPNet{}IP: net.ParseIP("172.16.0.0"), Mask: net.CIDRMask(16, 32)}) -> 172.16.0.0
//
func lowestIP4(ipRange *net.IPNet) net.IP {
	return ipRange.IP.Mask(ipRange.Mask).To16()
}

// highestIP4 returns the highest possible IP address
// in an IP network. For example:
//
//     highestIP4(net.IPNet{}IP: net.ParseIP("172.16.0.0"), Mask: net.CIDRMask(16, 32)}) -> 172.16.255.255
//
func highestIP4(ipRange *net.IPNet) net.IP {
	if !isIP4(ipRange.IP) {
		return nil
	}

	newIP := net.IPv4(0, 0, 0, 0)
	ipRange.IP = ipRange.IP.To4()
	for i := 0; i < len(ipRange.Mask); i++ {
		newIP[i+12] = ipRange.IP[i] | ^ipRange.Mask[i]
	}

	return newIP
}

// NewAddressSpaceFromNetwork creates a new AddressSpace from a network specification.
func NewAddressSpaceFromNetwork(ipRange *net.IPNet) *AddressSpace {
	s := &AddressSpace{
		Network: ipRange,
		Pool:    &ip.Range{FirstIP: lowestIP4(ipRange), LastIP: highestIP4(ipRange)},
	}
	s.availableRanges = []*ip.Range{s.Pool}

	return s
}

// NewAddressSpaceFromRange creates a new AddressSpace from a range of IP addresses.
func NewAddressSpaceFromRange(firstIP net.IP, lastIP net.IP) *AddressSpace {
	if compareIP4(firstIP, lastIP) > 0 {
		return nil
	}

	return &AddressSpace{
		Pool:            &ip.Range{FirstIP: firstIP, LastIP: lastIP},
		availableRanges: []*ip.Range{{FirstIP: firstIP, LastIP: lastIP}}}
}

func (s *AddressSpace) NextIP4Net(mask net.IPMask) (*net.IPNet, error) {
	ones, _ := mask.Size()
	for _, r := range s.availableRanges {
		network := r.FirstIP.Mask(mask).To16()
		var firstIP net.IP
		// check if the start of the current range
		// is lower than the network boundary
		if compareIP4(network, r.FirstIP) >= 0 {
			// found the start of the range
			firstIP = network
		} else {
			// network address is lower than the first
			// ip in the range; try the next network
			// in the mask
			for i := len(network) - 1; i >= 12; i-- {
				partialByteIndex := ones/8 + 12
				var inc byte
				if i == partialByteIndex {
					// this octet may only be occupied
					// by the mask partially, e.g.
					// for a /25, the last octet has
					// only one bit in the mask
					//
					// in order to get the next network
					// we need to increment starting at
					// the last bit of the mask, e.g. 25
					// in this example, which would be
					// bit 8 in the last octet.
					inc = (byte)(1 << (uint)(8-ones%8))
				} else if i < partialByteIndex {
					// we are past the partial octets,
					// so this is portion where the mask
					// occupies the octets fully, so
					// we can just increment the last bit
					inc = 1
				}

				if inc == 0 {
					continue
				}

				network[i] += inc
				if network[i] > 0 {
					firstIP = network
					break
				}

			}
		}

		if firstIP != nil {
			// we found the first IP for the requested range,
			// now check if the available range can accommodate
			// the highest address given the first IP and the mask
			lastIP := highestIP4(&net.IPNet{IP: firstIP, Mask: mask})
			if compareIP4(lastIP, r.LastIP) <= 0 {
				return &net.IPNet{IP: firstIP, Mask: mask}, nil
			}
		}
	}

	return nil, fmt.Errorf("could not find IP range for mask %s", mask)
}

// ReserveNextIP4Net reserves a new sub address space within the given address
// space, given a bitmask specifying the "width" of the requested space.
func (s *AddressSpace) ReserveNextIP4Net(mask net.IPMask) (*AddressSpace, error) {
	n, err := s.NextIP4Net(mask)
	if err != nil {
		return nil, err
	}

	return s.ReserveIP4Net(n)
}

func splitRange(parentRange *ip.Range, firstIP net.IP, lastIP net.IP) (before, reserved, after *ip.Range) {
	if !firstIP.Equal(parentRange.FirstIP) {
		before = ip.NewRange(parentRange.FirstIP, decrementIP4(firstIP))
	}
	if !lastIP.Equal(parentRange.LastIP) {
		after = ip.NewRange(incrementIP4(lastIP), parentRange.LastIP)
	}

	reserved = ip.NewRange(firstIP, lastIP)
	return
}

// ReserveIP4Net reserves a new sub address space given an IP and mask.
// Mask is required.
// If IP is nil or "0.0.0.0", same as calling ReserveNextIP4Net
// with the mask.
func (s *AddressSpace) ReserveIP4Net(ipNet *net.IPNet) (*AddressSpace, error) {
	if ipNet.Mask == nil {
		return nil, fmt.Errorf("network mask not specified")
	}

	if ipNet.IP == nil || ipNet.IP.Equal(net.ParseIP("0.0.0.0")) {
		return s.ReserveNextIP4Net(ipNet.Mask)
	}

	sub, err := s.ReserveIP4Range(lowestIP4(ipNet), highestIP4(ipNet))
	if err != nil {
		return nil, err
	}

	sub.Network = &net.IPNet{IP: ipNet.IP, Mask: ipNet.Mask}
	return sub, nil
}

func (s *AddressSpace) reserveSubRange(firstIP net.IP, lastIP net.IP, index int) {
	before, _, after := splitRange(s.availableRanges[index], firstIP, lastIP)
	s.availableRanges = append(s.availableRanges[:index], s.availableRanges[index+1:]...)
	if before != nil {
		s.availableRanges = insertAddressRanges(s.availableRanges, index, before)
		index++
	}
	if after != nil {
		s.availableRanges = insertAddressRanges(s.availableRanges, index, after)
	}
}

// ReserveIP4Range reserves a sub address space given a first and last IP.
func (s *AddressSpace) ReserveIP4Range(firstIP net.IP, lastIP net.IP) (*AddressSpace, error) {
	for i, r := range s.availableRanges {
		if compareIP4(firstIP, r.FirstIP) < 0 ||
			compareIP4(lastIP, r.LastIP) > 0 {
			continue
		}

		// found range
		log.Infof("Reserving IP range [%s, %s]", firstIP.String(), lastIP.String())
		s.reserveSubRange(firstIP, lastIP, i)
		subSpace := NewAddressSpaceFromRange(firstIP, lastIP)
		subSpace.Parent = s
		return subSpace, nil
	}

	var err error
	if compareIP4(firstIP, s.Pool.FirstIP) > 0 && compareIP4(lastIP, s.Pool.LastIP) < 0 {
		// IP range is within the pool but not found available
		err = fmt.Errorf("Cannot reserve IP range %s - %s.  Already in use", firstIP.String(), lastIP.String())
	} else {
		err = fmt.Errorf("Cannot reserve IP range %s - %s.  Not within pool's range %s - %s",
			firstIP.String(), lastIP.String(), s.Pool.FirstIP, s.Pool.LastIP)
	}

	log.Errorf(err.Error())

	return nil, err
}

func insertAddressRanges(r []*ip.Range, index int, ranges ...*ip.Range) []*ip.Range {
	if index == len(r) {
		return append(r, ranges...)
	}

	for i := 0; i < len(ranges); i++ {
		r = append(r, &ip.Range{})
	}

	copy(r[index+len(ranges):], r[index:])
	for i := 0; i < len(ranges); i++ {
		r[index+i] = ranges[i]
	}

	return r
}

// ReserveNextIP4 reserves the next available IPv4 address.
func (s *AddressSpace) ReserveNextIP4() (net.IP, error) {
	space, err := s.ReserveIP4Net(&net.IPNet{Mask: net.CIDRMask(32, 32)})
	if err != nil {
		return nil, err
	}

	return space.availableRanges[0].FirstIP, nil
}

// ReserveIP4 reserves the given IPv4 address.
func (s *AddressSpace) ReserveIP4(ip net.IP) error {
	_, err := s.ReserveIP4Range(ip, ip)
	return err
}

// ReleaseIP4Range releases a sub address space into the parent address space.
// Sub address space has to have only a single available range.
func (s *AddressSpace) ReleaseIP4Range(space *AddressSpace) error {
	// nothing to release
	if space == nil || len(space.availableRanges) == 0 {
		return nil
	}

	if space.Parent != s {
		return fmt.Errorf("cannot release subspace into another parent")
	}

	// cannot release a range if it has more than one available sub range
	if len(space.availableRanges) > 1 {
		return fmt.Errorf("can not release an address space with more than one available range")
	}

	firstIP := space.availableRanges[0].FirstIP
	lastIP := space.availableRanges[0].LastIP
	if compareIP4(firstIP, lastIP) > 0 {
		return fmt.Errorf("address space first ip %s is greater than last ip %s", firstIP, lastIP)
	}

	i := 0
	for ; i < len(s.availableRanges); i++ {
		if compareIP4(lastIP, s.availableRanges[i].FirstIP) < 0 {
			if i == 0 {
				break
			}

			if i > 0 && compareIP4(firstIP, s.availableRanges[i-1].LastIP) > 0 {
				break
			}
		}
	}

	if i > 0 && i == len(s.availableRanges) {
		if compareIP4(firstIP, s.availableRanges[i-1].LastIP) <= 0 {
			return fmt.Errorf("Could not release IP range")
		}
	}

	s.availableRanges = insertAddressRanges(s.availableRanges, i, space.availableRanges...)
	// #nosec: Errors unhandled.
	s.Defragment()

	log.Infof("Released IP range [%s, %s]", firstIP, lastIP)
	return nil
}

// ReleaseIP4 releases the given IPv4 address.
func (s *AddressSpace) ReleaseIP4(ip net.IP) error {
	tmp := NewAddressSpaceFromRange(ip, ip)
	tmp.Parent = s
	return s.ReleaseIP4Range(tmp)
}

func (s *AddressSpace) Defragment() error {
	for i := 1; i < len(s.availableRanges); {
		first := s.availableRanges[i-1]
		second := s.availableRanges[i]
		if incrementIP4(first.LastIP).Equal(second.FirstIP) {
			first.LastIP = second.LastIP
			s.availableRanges = append(s.availableRanges[:i], s.availableRanges[i+1:]...)
		} else {
			i++
		}
	}

	return nil
}

// Equal compares two address spaces for equality.
func (s *AddressSpace) Equal(other *AddressSpace) bool {
	if len(s.availableRanges) != len(other.availableRanges) {
		return false
	}

	for i := 0; i < len(s.availableRanges); i++ {
		if compareIP4(s.availableRanges[i].FirstIP, other.availableRanges[i].FirstIP) != 0 ||
			compareIP4(s.availableRanges[i].LastIP, other.availableRanges[i].LastIP) != 0 {
			return false
		}
	}

	return true
}
