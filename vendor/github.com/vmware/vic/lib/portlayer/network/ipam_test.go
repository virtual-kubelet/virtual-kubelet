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

package network

import (
	"net"
	"testing"
)

func TestIncrementIP4(t *testing.T) {
	var tests = []struct {
		in  net.IP
		out net.IP
	}{
		{net.IPv6loopback, nil},
		{net.ParseIP("10.10.10.255"), net.ParseIP("10.10.11.0")},
		{net.ParseIP("10.10.255.255"), net.ParseIP("10.11.0.0")},
		{net.ParseIP("10.255.255.255"), net.ParseIP("11.0.0.0")},
		{net.ParseIP("255.255.255.255"), net.ParseIP("0.0.0.0")},
	}

	for _, te := range tests {
		ip := incrementIP4(te.in)
		if !te.out.Equal(ip) {
			t.Errorf("got: %s, expected: %s", ip, te.out)
		}
	}
}

func TestDecrementIP4(t *testing.T) {
	var tests = []struct {
		in  net.IP
		out net.IP
	}{
		{net.IPv6loopback, nil},
		{net.ParseIP("10.10.10.0"), net.ParseIP("10.10.9.255")},
		{net.ParseIP("10.10.0.0"), net.ParseIP("10.9.255.255")},
		{net.ParseIP("10.0.0.0"), net.ParseIP("9.255.255.255")},
		{net.ParseIP("0.0.0.0"), net.ParseIP("255.255.255.255")},
	}

	for _, te := range tests {
		ip := decrementIP4(te.in)
		if !te.out.Equal(ip) {
			t.Errorf("got: %s, expected: %s", ip, te.out)
		}
	}
}

func TestCompareIP4(t *testing.T) {
	ips := []net.IP{
		net.ParseIP("10.10.10.10"),
		net.ParseIP("10.10.10.9"),
		net.ParseIP("10.10.9.9"),
		net.ParseIP("10.9.9.9"),
		net.ParseIP("9.9.9.9")}

	for i := 0; i < len(ips)-1; i++ {
		if res := compareIP4(ips[i+1], ips[i]); res != -1 {
			t.Fatalf("comparing %s %s got: %v, expected: -1", ips[i+1], ips[i], res)
		}
		if res := compareIP4(ips[i], ips[i+1]); res != 1 {
			t.Fatalf("comparing %s %s got: %v, expected: 1", ips[i], ips[i+1], res)
		}
		if res := compareIP4(ips[i], ips[i]); res != 0 {
			t.Fatalf("comparing %s %s got: %v expected: 0", ips[i], ips[i], res)
		}
	}
}

func TestIsIP4(t *testing.T) {
	ip4 := net.IPv4(10, 10, 10, 10)
	if !isIP4(ip4) {
		t.Fatalf("ip %s got: false expected: true", ip4)
	}
	ip6 := net.IPv6loopback
	if isIP4(ip6) {
		t.Fatalf("ip %s got: true, expected: false", ip6)
	}
}

func TestLowestIP4(t *testing.T) {
	r := &net.IPNet{IP: net.ParseIP("10.10.10.10").To4(), Mask: net.CIDRMask(24, 32)}
	ip := net.ParseIP("10.10.10.0")
	if res := lowestIP4(r); !res.Equal(ip) {
		t.Errorf("range %s got: %s expected %s", r, res, ip)
	}
}

func TestHighestIP4(t *testing.T) {
	var tests = []struct {
		in  *net.IPNet
		out net.IP
	}{
		{&net.IPNet{IP: net.IPv6loopback}, nil},
		{&net.IPNet{IP: net.ParseIP("10.10.10.10").To4(), Mask: net.CIDRMask(24, 32)}, net.ParseIP("10.10.10.255")},
	}

	for _, te := range tests {
		if res := highestIP4(te.in); !res.Equal(te.out) {
			t.Errorf("range %s got: %s expected %s", te.in, res, te.out)
		}
	}
}

func TestReserveIP4(t *testing.T) {
	space := NewAddressSpaceFromRange(net.ParseIP("10.10.10.10"),
		net.ParseIP("10.10.10.11"))

	ip, err := space.ReserveNextIP4()
	expected := net.ParseIP("10.10.10.10")
	if err != nil || !ip.Equal(expected) {
		t.Errorf("got: %s, %s expected: %s, nil", ip, err, expected)
	}

	ip, err = space.ReserveNextIP4()
	expected = net.ParseIP("10.10.10.11")
	if err != nil || !ip.Equal(expected) {
		t.Errorf("got: %s, %s expected: %s, nil", ip, err, expected)
	}

	ip, err = space.ReserveNextIP4()
	if err == nil {
		t.Errorf("got: %s, %s expected: nil, error", ip, err)
	}
}

func TestReleaseIP4(t *testing.T) {
	space := NewAddressSpaceFromRange(net.ParseIP("10.10.10.10"),
		net.ParseIP("10.10.10.11"))

	ip, err := space.ReserveNextIP4()
	expected := net.ParseIP("10.10.10.10")
	if err != nil || !ip.Equal(expected) {
		t.Errorf("got: %s, %s expected: %s, nil", ip, err, expected)
	}

	ip, err = space.ReserveNextIP4()
	expected = net.ParseIP("10.10.10.11")
	if err != nil || !ip.Equal(expected) {
		t.Errorf("got: %s, %s expected: %s, nil", ip, err, expected)
	}

	ip, err = space.ReserveNextIP4()
	if err == nil {
		t.Errorf("got: %s, %s expected: nil, error", ip, err)
	}

	err = space.ReleaseIP4(net.ParseIP("10.10.10.10"))
	if err != nil {
		t.Errorf("got: %s expected: nil", err)
	}

	err = space.ReleaseIP4(net.ParseIP("10.10.10.10"))
	if err == nil {
		t.Errorf("got: nil expected: error")
	}

	err = space.ReleaseIP4(net.ParseIP("10.10.10.11"))
	if err != nil {
		t.Errorf("got: %s expected: nil", err)
	}

	ip, err = space.ReserveNextIP4()
	expected = net.ParseIP("10.10.10.10")
	if err != nil || !ip.Equal(expected) {
		t.Errorf("got: %s, %s expected: %s, nil", ip, err, expected)
	}

}

func TestReserveNextIP4Net(t *testing.T) {
	_, net1, _ := net.ParseCIDR("172.16.0.0/12")
	space := NewAddressSpaceFromNetwork(net1)
	firstIP := net.IPv4(172, 16, 0, 0)
	lastIP := net.IPv4(172, 16, 255, 255)
	totalSubspaces := 0
	subspace, err := space.ReserveNextIP4Net(net.CIDRMask(16, 32))
	for err == nil {
		totalSubspaces++
		if compareIP4(firstIP, subspace.availableRanges[0].FirstIP) != 0 {
			t.Errorf("got: %s, expected: %s", subspace.availableRanges[0].FirstIP, firstIP)
		}
		if compareIP4(lastIP, subspace.availableRanges[0].LastIP) != 0 {
			t.Errorf("got: %s, expected: %s", subspace.availableRanges[0].LastIP, lastIP)
		}
		firstIP = net.IPv4(172, firstIP[13]+1, 0, 0)
		lastIP = net.IPv4(172, lastIP[13]+1, 255, 255)
		subspace, err = space.ReserveNextIP4Net(net.CIDRMask(16, 32))
	}

	if totalSubspaces != 16 {
		t.Errorf("got: %d, expected: 16", totalSubspaces)
	}

	space = NewAddressSpaceFromNetwork(net1)
	// peal off one ip from the range
	ip, err := space.ReserveNextIP4()
	if !ip.Equal(net.ParseIP("172.16.0.0")) {
		t.Errorf("got: %s, expected: 172.16.0.0", ip)
	}
	subSpace, err := space.ReserveNextIP4Net(net.CIDRMask(16, 32))
	ip, err = subSpace.ReserveNextIP4()
	if compareIP4(ip, net.ParseIP("172.17.0.0")) != 0 {
		t.Errorf("got: %s, expected: %s", ip, net.ParseIP("172.17.0.0"))
	}

	subSpace, err = space.ReserveNextIP4Net(net.CIDRMask(15, 32))
	ip, err = subSpace.ReserveNextIP4()
	if compareIP4(ip, net.ParseIP("172.18.0.0")) != 0 {
		t.Errorf("got: %s, expected: %s", ip, net.ParseIP("172.18.0.0"))
	}
}

func TestReserveIP4Net(t *testing.T) {
	ipNet := &net.IPNet{IP: net.ParseIP("172.16.0.0"), Mask: net.CIDRMask(12, 32)}
	space := NewAddressSpaceFromNetwork(ipNet)
	// no mask
	_, err := space.ReserveIP4Net(&net.IPNet{IP: net.ParseIP("10.10.10.10")})
	if err == nil {
		t.Errorf("got: nil, expected: error")
	}

	// IP == nil, Mask != nil
	_, err = space.ReserveIP4Net(&net.IPNet{Mask: net.CIDRMask(12, 32)})
	if err != nil {
		t.Errorf("got: %s, expected: nil", err)
	}
	_, err = space.ReserveNextIP4()
	if err == nil {
		t.Errorf("got: nil, expected: error")
	}
	space = NewAddressSpaceFromNetwork(ipNet)

	// ip == "0.0.0.0", Mask != nil
	_, err = space.ReserveIP4Net(&net.IPNet{IP: net.ParseIP("0.0.0.0"), Mask: net.CIDRMask(12, 32)})
	if err != nil {
		t.Errorf("got: %s, expected: nil", err)
	}
	_, err = space.ReserveNextIP4()
	if err == nil {
		t.Errorf("got: nil, expected: error")
	}
	space = NewAddressSpaceFromNetwork(ipNet)

	// reserve the full space
	_, err = space.ReserveIP4Net(ipNet)
	if err != nil {
		t.Errorf("got: %s, expected: nil", err)
	}
	// no more ips left
	_, err = space.ReserveNextIP4()
	if err == nil {
		t.Errorf("got: nil, expected: error")
	}
}

func TestReserveIP4Range(t *testing.T) {
	s := NewAddressSpaceFromNetwork(&net.IPNet{IP: net.IPv4(10, 10, 10, 0), Mask: net.CIDRMask(24, 32)})
	s.ReserveNextIP4()
	// try to reserve an unavailable range
	_, err := s.ReserveIP4Range(net.IPv4(10, 10, 10, 0), net.IPv4(10, 10, 10, 255))
	if err == nil {
		t.Errorf("got: nil, expected: error")
	}
}

func TestReleaseIP4Range(t *testing.T) {
	_, net1, _ := net.ParseCIDR("172.16.0.0/12")
	space := NewAddressSpaceFromNetwork(net1)
	err := space.ReleaseIP4Range(nil)
	if err != nil {
		t.Errorf("got: %s, expected: nil", err)
	}

	// reserve the full range
	subSpaces := make([]*AddressSpace, 16)
	totalReserved := 0
	subSpaces[0], err = space.ReserveNextIP4Net(net.CIDRMask(16, 32))
	if err != nil {
		t.Errorf("got: %s, expected: nil", err)
	}
	totalReserved++
	for i := 1; i < len(subSpaces) && err == nil; i++ {
		totalReserved++
		subSpaces[i], err = space.ReserveNextIP4Net(net.CIDRMask(16, 32))
	}
	if totalReserved != 16 {
		t.Errorf("got: %d, expected: 16", totalReserved)
	}

	// release a range at the beginning
	err = space.ReleaseIP4Range(subSpaces[0])
	if err != nil {
		t.Errorf("got: %s, expected: nil", err)
	}

	// try to release an already released range
	err = space.ReleaseIP4Range(subSpaces[0])
	if err == nil {
		t.Errorf("got: nil, expected: error")
	}

	// release a range in the middle
	err = space.ReleaseIP4Range(subSpaces[5])
	if err != nil {
		t.Errorf("got: %s, expected: nil", err)
	}

	// release a range at the end
	err = space.ReleaseIP4Range(subSpaces[len(subSpaces)-1])
	if err != nil {
		t.Errorf("got: %s, expected: nil", err)
	}

	// try to reserve a released range
	subspace, err := space.ReserveNextIP4Net(net.CIDRMask(16, 32))
	if err != nil || !subSpaces[0].Equal(subspace) {
		t.Fail()
	}

	space = NewAddressSpaceFromNetwork(net1)
	// get a sub space
	subSpace, err := space.ReserveNextIP4Net(net.CIDRMask(16, 32))
	if err != nil {
		t.Errorf("got: %s, expected: nil", err)
	}
	// fragment the sub space
	err = subSpace.ReserveIP4(net.ParseIP("172.16.0.2"))
	if err != nil {
		t.Errorf("got: %s, expected: nil", err)
	}
	// try to release it; should fail
	err = space.ReleaseIP4Range(subSpace)
	if err == nil {
		t.Errorf("got: nil, expected: error")
	}
}

func TestDefragment(t *testing.T) {
	_, net1, _ := net.ParseCIDR("172.16.0.0/24")
	space := NewAddressSpaceFromNetwork(net1)
	ip, _ := space.ReserveNextIP4()
	if compareIP4(ip, net.ParseIP("172.16.0.0")) != 0 {
		t.Errorf("got: %s, expected: %s", ip, net.ParseIP("172.16.0.0"))
	}

	err := space.ReserveIP4(net.ParseIP("172.16.0.24"))
	if err != nil {
		t.Errorf("got: %s, expected: nil", err)
	}

	space.ReleaseIP4(ip)
	if len(space.availableRanges) != 2 {
		t.Errorf("got: %d, expected: 2", len(space.availableRanges))
	}

	space.Defragment()
	if len(space.availableRanges) != 2 {
		t.Errorf("got: %d, expected: 2", len(space.availableRanges))
	}

	space.ReleaseIP4(net.ParseIP("172.16.0.24"))
	if len(space.availableRanges) != 1 {
		t.Errorf("got: %d, expected: 1", len(space.availableRanges))
	}

	space.Defragment()
	if len(space.availableRanges) != 1 {
		t.Errorf("got: %d, expected: 1", len(space.availableRanges))
	}
}
