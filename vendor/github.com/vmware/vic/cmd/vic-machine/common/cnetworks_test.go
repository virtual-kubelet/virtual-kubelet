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
	"bytes"
	"fmt"
	"net"
	"testing"

	"github.com/vmware/vic/pkg/ip"
)

func TestParseContainerNetworkGateways(t *testing.T) {
	var tests = []struct {
		cgs []string
		gws map[string]net.IPNet
		err error
	}{
		{[]string{""}, nil, fmt.Errorf("")},
		{[]string{"foo:"}, nil, fmt.Errorf("")},
		{[]string{":"}, nil, fmt.Errorf("")},
		{[]string{":10.10.10.10/24"}, nil, fmt.Errorf("")},
		{[]string{":foo"}, nil, fmt.Errorf("")},
		{[]string{"foo:10"}, nil, fmt.Errorf("")},
		{[]string{"foo:10.10.10.10/24", "foo:10.10.10.2/24"}, nil, fmt.Errorf("")},
		{
			[]string{"foo:10.10.10.10/24", "bar:10.10.9.1/16"},
			map[string]net.IPNet{
				"foo": {IP: net.ParseIP("10.10.10.10"), Mask: net.CIDRMask(24, 32)},
				"bar": {IP: net.ParseIP("10.10.9.1"), Mask: net.CIDRMask(16, 32)},
			},
			nil,
		},
	}

	for _, te := range tests {
		gws, err := parseContainerNetworkGateways(te.cgs)
		if te.err != nil {
			if err == nil {
				t.Fatalf("parseContainerNetworkGateways(%s) => (%v, nil) want (nil, err)", te.cgs, gws)
			}

			continue
		}

		if err != te.err ||
			gws == nil ||
			len(gws) != len(te.gws) {
			t.Fatalf("parseContainerNetworkGateways(%s) => (%v, %s) want (%v, nil)", te.cgs, gws, err, te.gws)
		}

		for v, g := range gws {
			if g2, ok := te.gws[v]; !ok {
				t.Fatalf("parseContainerNetworkGateways(%s) => (%v, %s) want (%v, nil)", te.cgs, gws, err, te.gws)
			} else if !g2.IP.Equal(g.IP) || bytes.Compare(g2.Mask, g.Mask) != 0 {
				t.Fatalf("parseContainerNetworkGateways(%s) => (%v, %s) want (%v, nil)", te.cgs, gws, err, te.gws)
			}
		}
	}
}

func TestParseContainerNetworkIPRanges(t *testing.T) {
	var tests = []struct {
		cps  []string
		iprs map[string][]*ip.Range
		err  error
	}{
		{[]string{""}, nil, fmt.Errorf("")},
		{[]string{"foo:"}, nil, fmt.Errorf("")},
		{[]string{":"}, nil, fmt.Errorf("")},
		{[]string{":10.10.10.10-24"}, nil, fmt.Errorf("")},
		{[]string{":foo"}, nil, fmt.Errorf("")},
		{[]string{"foo:10"}, nil, fmt.Errorf("")},
		{[]string{"foo:10.10.10.10-9"}, nil, fmt.Errorf("")},
		{[]string{"foo:10.10.10.10-10.10.10.9"}, nil, fmt.Errorf("")},
		{
			[]string{"foo:10.10.10.10-24"},
			map[string][]*ip.Range{"foo": {ip.NewRange(net.ParseIP("10.10.10.10"), net.ParseIP("10.10.10.24"))}}, nil},
		{
			[]string{"foo:10.10.10.10-10.10.10.24"},
			map[string][]*ip.Range{"foo": {ip.NewRange(net.ParseIP("10.10.10.10"), net.ParseIP("10.10.10.24"))}},
			nil,
		},
		{
			[]string{"foo:10.10.10.10-10.10.10.24", "foo:10.10.10.100-10.10.10.105"},
			map[string][]*ip.Range{
				"foo": {
					ip.NewRange(net.ParseIP("10.10.10.10"), net.ParseIP("10.10.10.24")),
					ip.NewRange(net.ParseIP("10.10.10.100"), net.ParseIP("10.10.10.105")),
				},
			},
			nil,
		},
		{
			[]string{"foo:10.10.10.10-10.10.10.24", "bar:10.10.9.1-10.10.9.10"},
			map[string][]*ip.Range{
				"foo": {ip.NewRange(net.ParseIP("10.10.10.10"), net.ParseIP("10.10.10.24"))},
				"bar": {ip.NewRange(net.ParseIP("10.10.9.1"), net.ParseIP("10.10.9.10"))},
			},
			nil,
		},
	}

	for _, te := range tests {
		iprs, err := parseContainerNetworkIPRanges(te.cps)
		if te.err != nil {
			if err == nil {
				t.Fatalf("parseContainerNetworkIPRanges(%s) => (%v, nil) want (nil, err)", te.cps, iprs)
			}

			continue
		}

		if err != te.err ||
			len(iprs) != len(te.iprs) {
			t.Fatalf("parseContainerNetworkIPRanges(%s) => (%v, %s) want (%v, %s)", te.cps, iprs, err, te.iprs, te.err)
		}

		for v, ipr := range iprs {
			if ipr2, ok := te.iprs[v]; !ok {
				t.Fatalf("parseContainerNetworkIPRanges(%s) => (%v, %s) want (%v, %s)", te.cps, iprs, err, te.iprs, te.err)
			} else {
				for _, i := range ipr {
					found := false
					for _, i2 := range ipr2 {
						if i.Equal(i2) {
							found = true
							break
						}
					}

					if !found {
						t.Fatalf("parseContainerNetworkIPRanges(%s) => (%v, %s) want (%v, %s)", te.cps, iprs, err, te.iprs, te.err)
					}
				}
			}
		}
	}
}

func TestParseContainerNetworkDNS(t *testing.T) {
	var tests = []struct {
		cds []string
		dns map[string][]net.IP
		err error
	}{
		{[]string{""}, nil, fmt.Errorf("")},
		{[]string{"foo:"}, nil, fmt.Errorf("")},
		{[]string{":"}, nil, fmt.Errorf("")},
		{[]string{":10.10.10.10"}, nil, fmt.Errorf("")},
		{[]string{":foo"}, nil, fmt.Errorf("")},
		{[]string{"foo:10"}, nil, fmt.Errorf("")},
		{[]string{"foo:10.10.10.109"}, map[string][]net.IP{"foo": {net.ParseIP("10.10.10.109")}}, nil},
		{[]string{"foo:10.10.10.109", "foo:10.10.10.110", "bar:10.10.9.109", "bar:10.10.9.110"},
			map[string][]net.IP{
				"foo": {net.ParseIP("10.10.10.109"), net.ParseIP("10.10.10.110")},
				"bar": {net.ParseIP("10.10.9.109"), net.ParseIP("10.10.9.110")},
			},
			nil,
		},
	}

	for _, te := range tests {
		dns, err := parseContainerNetworkDNS(te.cds)
		if te.err != nil {
			if err == nil {
				t.Fatalf("parseContainerNetworkDNS(%s) => (%v, nil) want (nil, err)", te.cds, dns)
			}

			continue
		}

		if err != te.err ||
			len(dns) != len(te.dns) {
			t.Fatalf("parseContainerNetworkDNS(%s) => (%v, %s) want (%v, %s)", te.cds, dns, err, te.dns, te.err)
		}

		for v, d := range dns {
			if d2, ok := te.dns[v]; !ok {
				t.Fatalf("parseContainerNetworkDNS(%s) => (%v, %s) want (%v, %s)", te.cds, dns, err, te.dns, te.err)
			} else {
				for _, i := range d {
					found := false
					for _, i2 := range d2 {
						if i.Equal(i2) {
							found = true
							break
						}
					}

					if !found {
						t.Fatalf("parseContainerNetworkDNS(%s) => (%v, %s) want (%v, %s)", te.cds, dns, err, te.dns, te.err)
					}
				}
			}
		}
	}
}
