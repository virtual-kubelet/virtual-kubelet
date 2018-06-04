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

package ip

import (
	"fmt"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRangeMarshalText(t *testing.T) {
	var tests = []struct {
		ipr *Range
		s   string
		err error
	}{
		{&Range{net.ParseIP("10.10.10.10"), net.ParseIP("10.10.10.24")}, "10.10.10.10-10.10.10.24", nil},
	}

	for _, te := range tests {
		b, err := te.ipr.MarshalText()
		if te.err != nil && err == nil {
			t.Fatalf("MarshalText() => (%v, nil) want (nil, err)", b)
			continue
		}

		if string(b) != te.s {
			t.Fatalf("MarshalText() => (%s, %s) want (%s, nil)", string(b), err, te.s)
		}
	}
}

func TestRangeUnmarshalText(t *testing.T) {

	var tests = []struct {
		r   string
		ipr *Range
		err error
	}{
		{"10.10.10.10-9", nil, fmt.Errorf("")},
		{"10.10.10.10-10.10.10.9", nil, fmt.Errorf("")},
		{"10.10.10.10-24", &Range{net.ParseIP("10.10.10.10"), net.ParseIP("10.10.10.24")}, nil},
		{"10.10.10.10-10.10.10.24", &Range{net.ParseIP("10.10.10.10"), net.ParseIP("10.10.10.24")}, nil},
		{"10.10.10.0/24", &Range{net.ParseIP("10.10.10.0"), net.ParseIP("10.10.10.255")}, nil},
	}

	for _, te := range tests {
		ipr := &Range{}
		err := ipr.UnmarshalText([]byte(te.r))
		if te.err != nil {
			if err == nil {
				t.Fatalf("UnmarshalText(%s) => nil want err", te.r)
			}

			continue
		}

		if !te.ipr.FirstIP.Equal(ipr.FirstIP) ||
			!te.ipr.LastIP.Equal(ipr.LastIP) {
			t.Fatalf("UnmarshalText(%s) => %#v want %#v", te.r, ipr, te.ipr)
		}
	}
}

func TestRangeOverlap(t *testing.T) {
	var tests = []struct {
		ipr1, ipr2 Range
		res        bool
	}{
		{Range{net.ParseIP("10.10.10.10"), net.ParseIP("10.10.10.24")}, Range{net.ParseIP("10.10.10.10"), net.ParseIP("10.10.10.24")}, true},
		{Range{net.ParseIP("10.10.10.10"), net.ParseIP("10.10.10.24")}, Range{net.ParseIP("10.10.10.15"), net.ParseIP("10.10.10.24")}, true},
		{Range{net.ParseIP("10.10.10.10"), net.ParseIP("10.10.10.24")}, Range{net.ParseIP("10.10.10.15"), net.ParseIP("10.10.10.20")}, true},
		{Range{net.ParseIP("10.10.10.10"), net.ParseIP("10.10.10.24")}, Range{net.ParseIP("10.10.10.9"), net.ParseIP("10.10.10.25")}, true},
		{Range{net.ParseIP("10.10.10.10"), net.ParseIP("10.10.10.24")}, Range{net.ParseIP("10.10.10.24"), net.ParseIP("10.10.10.25")}, true},
		{Range{net.ParseIP("10.10.10.10"), net.ParseIP("10.10.10.24")}, Range{net.ParseIP("10.10.10.25"), net.ParseIP("10.10.10.50")}, false},
	}

	for _, te := range tests {
		res := te.ipr1.Overlaps(te.ipr2)
		if res != te.res {
			t.Fatalf("(%s).Overlaps(%s) => %t want %t", te.ipr1, te.ipr2, res, te.res)
		}
	}
}

func TestAllZerosAddr(t *testing.T) {
	var tests = []struct {
		subnet *net.IPNet
		addr   net.IP
	}{
		{&net.IPNet{IP: net.ParseIP("192.168.0.0"), Mask: net.CIDRMask(16, 32)}, net.ParseIP("192.168.0.0")},
		{&net.IPNet{IP: net.ParseIP("192.168.100.0"), Mask: net.CIDRMask(24, 32)}, net.ParseIP("192.168.100.0")},
	}

	for _, te := range tests {
		addr := AllZerosAddr(te.subnet)
		if !te.addr.Equal(addr) {
			t.Fatalf("AllZerosAddr(%s) => got %s, want %s", te.subnet, addr, te.addr)
		}
	}
}

func TestAllOnesAddr(t *testing.T) {
	var tests = []struct {
		subnet *net.IPNet
		addr   net.IP
	}{
		{&net.IPNet{IP: net.ParseIP("192.168.0.0"), Mask: net.CIDRMask(16, 32)}, net.ParseIP("192.168.255.255")},
		{&net.IPNet{IP: net.ParseIP("192.168.100.0"), Mask: net.CIDRMask(24, 32)}, net.ParseIP("192.168.100.255")},
	}

	for _, te := range tests {
		addr := AllOnesAddr(te.subnet)
		if !te.addr.Equal(addr) {
			t.Fatalf("AllOnesAddr(%s) => got %s, want %s", te.subnet, addr, te.addr)
		}
	}
}

func TestRangeNetwork(t *testing.T) {
	var tests = []struct {
		r *Range
		n *net.IPNet
	}{
		{ParseRange("10.10.10.10/24"), &net.IPNet{IP: net.ParseIP("10.10.10.0"), Mask: net.CIDRMask(24, 32)}},
		{ParseRange("10.10.10.10-10.10.14.11"), nil},
		{ParseRange("10.10.10.10-10.10.10.11"), &net.IPNet{IP: net.ParseIP("10.10.10.10"), Mask: net.CIDRMask(31, 32)}},
	}

	for _, te := range tests {
		n := te.r.Network()
		if te.n != nil {
			assert.NotNil(t, n)
		} else {
			assert.Nil(t, n)
			continue
		}

		if !n.IP.Equal(te.n.IP) {
			assert.FailNow(t, fmt.Sprintf("got %s, want %s", n, te.n))
		}

		assert.EqualValues(t, n.Mask, te.n.Mask)
	}
}
