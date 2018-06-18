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

package network

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"math/rand"
	"net"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/vic/lib/config"
	"github.com/vmware/vic/lib/config/executor"
	"github.com/vmware/vic/lib/constants"
	"github.com/vmware/vic/lib/portlayer/exec"
	"github.com/vmware/vic/lib/spec"
	"github.com/vmware/vic/pkg/ip"
	"github.com/vmware/vic/pkg/kvstore"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/uid"
	"github.com/vmware/vic/pkg/vsphere/extraconfig"
)

var testBridgeNetwork, testExternalNetwork object.NetworkReference

type params struct {
	scopeType, name string
	subnet          *net.IPNet
	gateway         net.IP
	dns             []net.IP
	pools           []string
}

var validScopeTests = []struct {
	in  params
	out *params
	err error
}{
	// bridge scopes

	// default bridge pool, only name specified
	{params{"bridge", "bar1", nil, net.IPv4(0, 0, 0, 0), nil, nil},
		&params{"bridge", "bar1", &net.IPNet{IP: net.IPv4(172, 17, 0, 0), Mask: net.CIDRMask(16, 32)}, net.ParseIP("172.17.0.1"), nil, nil},
		nil},
	// default bridge pool with gateway specified
	{params{"bridge", "bar2", nil, net.IPv4(172, 18, 0, 2), nil, nil},
		&params{"bridge", "bar2", &net.IPNet{IP: net.IPv4(172, 18, 0, 0), Mask: net.CIDRMask(16, 32)}, net.ParseIP("172.18.0.2"), nil, nil},
		nil},
	// not from default bridge pool
	{params{"bridge", "bar3", &net.IPNet{IP: net.ParseIP("10.10.0.0"), Mask: net.CIDRMask(16, 32)}, net.IPv4(0, 0, 0, 0), nil, nil},
		&params{"bridge", "bar3", &net.IPNet{IP: net.ParseIP("10.10.0.0"), Mask: net.CIDRMask(16, 32)}, net.ParseIP("10.10.0.1"), nil, nil},
		nil},
	// not from default bridge pool, dns specified
	{params{"bridge", "bar4", &net.IPNet{IP: net.ParseIP("10.11.0.0"), Mask: net.CIDRMask(16, 32)}, net.IPv4(0, 0, 0, 0), []net.IP{net.ParseIP("10.10.1.1")}, nil},
		&params{"bridge", "bar4", &net.IPNet{IP: net.ParseIP("10.11.0.0"), Mask: net.CIDRMask(16, 32)}, net.ParseIP("10.11.0.1"), []net.IP{net.ParseIP("10.10.1.1")}, nil},
		nil},
	// not from default pool, dns and ipam specified
	{params{"bridge", "bar5", &net.IPNet{IP: net.ParseIP("10.12.0.0"), Mask: net.CIDRMask(16, 32)}, net.IPv4(0, 0, 0, 0), []net.IP{net.ParseIP("10.10.1.1")}, []string{"10.12.1.0/24", "10.12.2.0/28"}},
		&params{"bridge", "bar5", &net.IPNet{IP: net.ParseIP("10.12.0.0"), Mask: net.CIDRMask(16, 32)}, net.ParseIP("10.12.1.0"), []net.IP{net.ParseIP("10.10.1.1")}, nil},
		nil},
	// not from default pool, dns, gateway, and ipam specified
	{params{"bridge", "bar51", &net.IPNet{IP: net.ParseIP("10.33.0.0"), Mask: net.CIDRMask(16, 32)}, net.IPv4(10, 33, 0, 1), []net.IP{net.ParseIP("10.10.1.1")}, []string{"10.33.0.0/16"}},
		&params{"bridge", "bar51", &net.IPNet{IP: net.ParseIP("10.33.0.0"), Mask: net.CIDRMask(16, 32)}, net.ParseIP("10.33.0.1"), []net.IP{net.ParseIP("10.10.1.1")}, nil},
		nil},
	// from default pool, subnet specified
	{params{"bridge", "bar6", &net.IPNet{IP: net.IPv4(172, 19, 0, 0), Mask: net.CIDRMask(16, 32)}, nil, nil, nil},
		&params{"bridge", "bar6", &net.IPNet{IP: net.IPv4(172, 19, 0, 0), Mask: net.CIDRMask(16, 32)}, net.ParseIP("172.19.0.1"), nil, nil},
		nil},
}

type mockLink struct{}

func (l *mockLink) AddrAdd(_ net.IPNet) error {
	return nil
}

func (l *mockLink) AddrDel(_ net.IPNet) error {
	return nil
}

func (l *mockLink) Attrs() *LinkAttrs {
	return &LinkAttrs{Name: "foo"}
}

type mockNetwork struct {
	name string
}

func (n mockNetwork) Reference() types.ManagedObjectReference {
	return types.ManagedObjectReference{Type: "mockNetwork", Value: n.name}
}

func (n mockNetwork) EthernetCardBackingInfo(ctx context.Context) (types.BaseVirtualDeviceBackingInfo, error) {
	return &types.VirtualEthernetCardNetworkBackingInfo{
		VirtualDeviceDeviceBackingInfo: types.VirtualDeviceDeviceBackingInfo{
			DeviceName: n.name,
		},
	}, nil
}

func testConfig() *Configuration {
	return &Configuration{
		source:     extraconfig.MapSource(map[string]string{}),
		sink:       extraconfig.MapSink(map[string]string{}),
		BridgeLink: &mockLink{},
		Network: config.Network{
			BridgeNetwork: "bridge",
			ContainerNetworks: map[string]*executor.ContainerNetwork{
				"bridge": {
					Common: executor.Common{
						Name: "bridge",
					},
					Type: constants.BridgeScopeType,
				},
				"bar7": {
					Common: executor.Common{
						Name: "external",
					},
					Gateway:     net.IPNet{IP: net.ParseIP("10.13.0.1"), Mask: net.CIDRMask(16, 32)},
					Nameservers: []net.IP{net.ParseIP("10.10.1.1")},
					Pools:       []ip.Range{*ip.ParseRange("10.13.1.0-255"), *ip.ParseRange("10.13.2.0-10.13.2.15")},
					Type:        constants.ExternalScopeType,
				},
				"bar71": {
					Common: executor.Common{
						Name: "external",
					},
					Gateway:     net.IPNet{IP: net.ParseIP("10.131.0.1"), Mask: net.CIDRMask(16, 32)},
					Nameservers: []net.IP{net.ParseIP("10.131.0.1"), net.ParseIP("10.131.0.2")},
					Pools:       []ip.Range{*ip.ParseRange("10.131.1.0/16")},
					Type:        constants.ExternalScopeType,
				},
				"bar72": {
					Common: executor.Common{
						Name: "external",
					},
					Type: constants.ExternalScopeType,
				},
				"bar73": {
					Common: executor.Common{
						Name: "external",
					},
					Gateway: net.IPNet{IP: net.ParseIP("10.133.0.1"), Mask: net.CIDRMask(16, 32)},
					Type:    constants.ExternalScopeType,
				},
			},
		},
		PortGroups: map[string]object.NetworkReference{
			"bridge": testBridgeNetwork,
			"bar7":   testExternalNetwork,
			"bar71":  testExternalNetwork,
			"bar72":  testExternalNetwork,
			"bar73":  testExternalNetwork,
		},
	}
}

func TestMain(m *testing.M) {
	testBridgeNetwork = &mockNetwork{name: "testBridge"}
	testExternalNetwork = &mockNetwork{name: "testExternal"}

	log.SetLevel(log.DebugLevel)

	rc := m.Run()

	os.Exit(rc)
}

func TestMapExternalNetworks(t *testing.T) {
	conf := testConfig()
	ctx, err := NewContext(conf, nil)
	if err != nil {
		t.Fatalf("NewContext() => (nil, %s), want (ctx, nil)", err)
	}

	// check if external networks were loaded
	for n, nn := range conf.ContainerNetworks {
		scopes, err := ctx.findScopes(&n)
		if err != nil || len(scopes) != 1 {
			t.Fatalf("external network %s was not loaded", n)
		}

		s := scopes[0]
		pools := s.Pools()
		if !ip.IsUnspecifiedIP(nn.Gateway.IP) {
			subnet := &net.IPNet{IP: nn.Gateway.IP.Mask(nn.Gateway.Mask), Mask: nn.Gateway.Mask}
			if ip.IsUnspecifiedSubnet(s.Subnet()) || !s.Subnet().IP.Equal(subnet.IP) || !bytes.Equal(s.Subnet().Mask, subnet.Mask) {
				t.Fatalf("external network %s was loaded with wrong subnet, got: %s, want: %s", n, s.Subnet(), subnet)
			}

			if ip.IsUnspecifiedIP(s.Gateway()) || !s.Gateway().Equal(nn.Gateway.IP) {
				t.Fatalf("external network %s was loaded with wrong gateway, got: %s, want: %s", n, s.Gateway(), nn.Gateway.IP)
			}

			if len(nn.Pools) == 0 {
				// only one pool corresponding to the subnet
				if len(pools) != 1 || !pools[0].Equal(ip.ParseRange(subnet.String())) {
					t.Fatalf("external network %s was loaded with wrong pool, got: %+v, want %+v", n, pools, []*net.IPNet{subnet})
				}
			}
		}

		for _, d := range nn.Nameservers {
			found := false
			for _, d2 := range s.DNS() {
				if d2.Equal(d) {
					found = true
					break
				}
			}

			if !found {
				t.Fatalf("external network %s was loaded with wrong nameservers, got: %+v, want: %+v", n, s.DNS(), nn.Nameservers)
			}
		}

		for _, p := range nn.Pools {
			found := false
			for _, p2 := range pools {
				if p2.Equal(&p) {
					found = true
					break
				}
			}

			if !found {
				t.Fatalf("external network %s was loaded with wrong pools, got: %+v, want: %+v", n, s.Pools(), nn.Pools)
			}
		}
	}
}

func TestContextNewScope(t *testing.T) {
	kv := &kvstore.MockKeyValueStore{}
	kv.On("List", mock.Anything).Return(nil, nil)
	ctx, err := NewContext(testConfig(), kv)
	if err != nil {
		t.Fatalf("NewContext() => (nil, %s), want (ctx, nil)", err)
	}

	kv.AssertNumberOfCalls(t, "List", 1)
	kv.AssertCalled(t, "List", `context\.scopes\..+`)

	var tests = []struct {
		in  params
		out *params
		err error
	}{
		// empty name
		{params{"bridge", "", nil, net.IPv4(0, 0, 0, 0), nil, nil}, nil, fmt.Errorf("")},
		// unsupported network type
		{params{"foo", "bar8", nil, net.IPv4(0, 0, 0, 0), nil, nil}, nil, fmt.Errorf("")},
		// duplicate name
		{params{"bridge", "bar6", nil, net.IPv4(0, 0, 0, 0), nil, nil}, nil, DuplicateResourceError{}},
		// ip range already allocated
		{params{"bridge", "bar9", &net.IPNet{IP: net.IPv4(172, 16, 0, 0), Mask: net.CIDRMask(16, 32)}, net.IPv4(0, 0, 0, 0), nil, nil}, nil, fmt.Errorf("")},
		// ipam out of range of network
		{params{"bridge", "bar10", &net.IPNet{IP: net.IPv4(10, 14, 0, 0), Mask: net.CIDRMask(16, 32)}, net.IPv4(0, 0, 0, 0), nil, []string{"10.14.1.0/24", "10.15.1.0/24"}}, nil, fmt.Errorf("")},
		// gateway not on subnet
		{params{"bridge", "bar101", &net.IPNet{IP: net.IPv4(10, 141, 0, 0), Mask: net.CIDRMask(16, 32)}, net.IPv4(10, 14, 0, 1), nil, nil}, nil, fmt.Errorf("")},
		// gateway is allzeros address
		{params{"bridge", "bar102", &net.IPNet{IP: net.IPv4(10, 142, 0, 0), Mask: net.CIDRMask(16, 32)}, net.IPv4(10, 142, 0, 0), nil, nil}, nil, fmt.Errorf("")},
		// gateway is allones address
		{params{"bridge", "bar103", &net.IPNet{IP: net.IPv4(10, 143, 0, 0), Mask: net.CIDRMask(16, 32)}, net.IPv4(10, 143, 255, 255), nil, nil}, nil, fmt.Errorf("")},
		// this should succeed now
		{params{"bridge", "bar11", &net.IPNet{IP: net.IPv4(10, 14, 0, 0), Mask: net.CIDRMask(16, 32)}, net.IPv4(0, 0, 0, 0), nil, []string{"10.14.1.0/24"}},
			&params{"bridge", "bar11", &net.IPNet{IP: net.IPv4(10, 14, 0, 0), Mask: net.CIDRMask(16, 32)}, net.IPv4(10, 14, 1, 0), nil, nil},
			nil},
		// bad ipam
		{params{"bridge", "bar12", &net.IPNet{IP: net.IPv4(10, 14, 0, 0), Mask: net.CIDRMask(16, 32)}, net.IPv4(0, 0, 0, 0), nil, []string{"10.14.1.0/24", "10.15.1"}}, nil, fmt.Errorf("")},
		// bad ipam, default bridge pool
		{params{"bridge", "bar12", &net.IPNet{IP: net.IPv4(172, 21, 0, 0), Mask: net.CIDRMask(16, 32)}, net.IPv4(0, 0, 0, 0), nil, []string{"172.21.1.0/24", "10.15.1"}}, nil, fmt.Errorf("")},
		// external networks must have subnet specified, if pool is specified
		{params{"external", "bar13", nil, net.IPv4(0, 0, 0, 0), nil, []string{"10.15.0.0/24"}}, nil, fmt.Errorf("")},
		// external networks must have gateway specified, if pool is specified
		{params{"external", "bar14", &net.IPNet{IP: net.IPv4(10, 14, 0, 0), Mask: net.CIDRMask(16, 32)}, net.IPv4(0, 0, 0, 0), nil, []string{"10.15.0.0/24"}}, nil, fmt.Errorf("")},
		// external networks cannot overlap bridge pool
		{params{"external", "bar16", &net.IPNet{IP: net.IPv4(172, 20, 0, 0), Mask: net.CIDRMask(16, 32)}, net.IPv4(10, 14, 0, 1), nil, []string{"172.20.0.0/16"}}, nil, fmt.Errorf("")},
	}

	kv.On("Put", context.TODO(), mock.Anything, mock.Anything).Return(nil)
	setCalls := 0

	tests = append(validScopeTests, tests...)

	for _, te := range tests {

		scopeData := &ScopeData{
			ScopeType: te.in.scopeType,
			Name:      te.in.name,
			Subnet:    te.in.subnet,
			Gateway:   te.in.gateway,
			DNS:       te.in.dns,
			Pools:     te.in.pools,
		}
		s, err := ctx.NewScope(context.TODO(), scopeData)

		if te.out == nil {
			// no additional call to kv.Set
			kv.AssertNumberOfCalls(t, "Put", setCalls)

			// error case
			if s != nil || err == nil {
				t.Fatalf("NewScope(%+v) => (s, nil), want (nil, err)", scopeData)
			}

			// if there is an error specified, check if we got that error
			if te.err != nil &&
				reflect.TypeOf(err) != reflect.TypeOf(te.err) {
				t.Fatalf("NewScope() => (nil, %s), want (nil, %s)", reflect.TypeOf(err), reflect.TypeOf(te.err))
			}

			if _, o := err.(DuplicateResourceError); !o {
				// sanity check
				if _, ok := ctx.scopes[te.in.name]; ok {
					t.Fatalf("scope %s added on error", te.in.name)
				}
			}

			continue
		}

		if err != nil {
			t.Fatalf("got: %s, expected: nil", err)
			continue
		}

		setCalls++
		kv.AssertNumberOfCalls(t, "Put", setCalls)

		// check if kv.Set was called with the expected arguments
		d, err := s.MarshalJSON()
		assert.NoError(t, err)
		kv.AssertCalled(t, "Put", context.TODO(), scopeKey(s.Name()), d)

		if s.Type() != te.out.scopeType {
			t.Fatalf("s.Type() => %s, want %s", s.Type(), te.out.scopeType)
			continue
		}

		if s.Name() != te.out.name {
			t.Fatalf("s.Name() => %s, want %s", s.Name(), te.out.name)
		}

		if s.Subnet().String() != te.out.subnet.String() {
			t.Fatalf("s.Subnet() => %s, want %s", s.Subnet(), te.out.subnet)
		}

		if !s.Gateway().Equal(te.out.gateway) {
			t.Fatalf("s.Gateway() => %s, want %s", s.Gateway(), te.out.gateway)
		}

		for _, d1 := range s.DNS() {
			found := false
			for _, d2 := range te.out.dns {
				if d2.Equal(d1) {
					found = true
					break
				}
			}

			if !found {
				t.Fatalf("s.DNS() => %q, want %q", s.DNS(), te.out.dns)
				break
			}
		}

		if s.Type() == constants.BridgeScopeType && s.Network() != testBridgeNetwork {
			t.Fatalf("s.NetworkName => %v, want %s", s.Network(), testBridgeNetwork)
			continue
		}

		hasPools := len(te.in.pools) > 0
		if hasPools {
			assert.Len(t, te.in.pools, len(s.spaces))
		} else {
			assert.Len(t, s.spaces, 1)
		}

		var parent *AddressSpace
		for i, p := range s.spaces {
			if parent == nil {
				parent = p.Parent
			}

			if !hasPools {
				// only one pool equal to the subnet
				assert.EqualValues(t, *p.Network, *s.subnet)
				continue
			}

			assert.NotNil(t, p.Parent)
			// all pools should have the same parent
			assert.Equal(t, parent, p.Parent)
			// if subnet is specified, it should be the same as the parent space
			if s.subnet != nil {
				assert.EqualValues(t, *s.subnet, *p.Parent.Network)
			}

			if p.Network != nil {
				if p.Network.String() != te.in.pools[i] {
					t.Fatalf("p.Network => %s, want %s", p.Network, te.in.pools[i])
				}
			} else if p.Pool.String() != te.in.pools[i] {
				t.Fatalf("p.Pool => %s, want %s", p.Pool, te.in.pools[i])
			}
		}
	}
}

func TestScopes(t *testing.T) {
	ctx, err := NewContext(testConfig(), nil)
	if err != nil {
		t.Fatalf("NewContext() => (nil, %s), want (ctx, nil)", err)
		return
	}

	scopes := make([]*Scope, 0, 0)
	scopesByID := make(map[string]*Scope)
	scopesByName := make(map[string]*Scope)
	for _, te := range validScopeTests {

		scopeData := &ScopeData{
			ScopeType: te.in.scopeType,
			Name:      te.in.name,
			Subnet:    te.in.subnet,
			Gateway:   te.in.gateway,
			DNS:       te.in.dns,
			Pools:     te.in.pools,
		}

		s, err := ctx.NewScope(context.TODO(), scopeData)
		if err != nil {
			t.Fatalf("NewScope() => (_, %s), want (_, nil)", err)
		}

		scopesByID[s.ID().String()] = s
		scopesByName[s.Name()] = s
		scopes = append(scopes, s)
	}

	id := scopesByName[validScopeTests[0].in.name].ID().String()
	partialID := id[:8]
	partialID2 := partialID[1:]
	badName := "foo"

	var tests = []struct {
		in  *string
		out []*Scope
	}{
		// name match
		{&validScopeTests[0].in.name, []*Scope{scopesByName[validScopeTests[0].in.name]}},
		// id match
		{&id, []*Scope{scopesByName[validScopeTests[0].in.name]}},
		// partial id match
		{&partialID, []*Scope{scopesByName[validScopeTests[0].in.name]}},
		// all scopes
		{nil, scopes},
		// partial id match only matches prefix
		{&partialID2, nil},
		// no match
		{&badName, nil},
	}

	for _, te := range tests {
		l, err := ctx.Scopes(context.TODO(), te.in)
		if te.out == nil {
			if err == nil {
				t.Fatalf("Scopes() => (_, nil), want (_, err)")
				continue
			}
		} else {
			if err != nil {
				t.Fatalf("Scopes() => (_, %s), want (_, nil)", err)
				continue
			}
		}

		// +5 for the default bridge scope, and 4 external networks
		if te.in == nil {
			if len(l) != len(te.out)+5 {
				t.Fatalf("len(scopes) => %d != %d", len(l), len(te.out)+5)
				continue
			}
		} else {
			if len(l) != len(te.out) {
				t.Fatalf("len(scopes) => %d != %d", len(l), len(te.out))
				continue
			}
		}

		for _, s1 := range te.out {
			found := false
			for _, s2 := range l {
				if s1 == s2 {
					found = true
					break
				}
			}

			if !found {
				t.Fatalf("got=%v, want=%v", l, te.out)
				break
			}
		}
	}
}

func TestContextAddContainer(t *testing.T) {
	ctx, err := NewContext(testConfig(), nil)
	if err != nil {
		t.Fatalf("NewContext() => (nil, %s), want (ctx, nil)", err)
		return
	}

	h := newContainer("foo")

	var devices object.VirtualDeviceList
	backing, _ := ctx.DefaultScope().Network().EthernetCardBackingInfo(context.TODO())

	specWithEthCard := &spec.VirtualMachineConfigSpec{
		VirtualMachineConfigSpec: &types.VirtualMachineConfigSpec{},
	}

	var d types.BaseVirtualDevice
	if d, err = devices.CreateEthernetCard("vmxnet3", backing); err == nil {
		d.GetVirtualDevice().SlotInfo = &types.VirtualDevicePciBusSlotInfo{
			PciSlotNumber: 1111,
		}
		devices = append(devices, d)
		var cs []types.BaseVirtualDeviceConfigSpec
		if cs, err = devices.ConfigSpec(types.VirtualDeviceConfigSpecOperationAdd); err == nil {
			specWithEthCard.DeviceChange = cs
		}
	}

	if err != nil {
		t.Fatalf(err.Error())
	}

	aecErr := func(_ *exec.Handle, _ *Scope) (types.BaseVirtualDevice, error) {
		return nil, fmt.Errorf("error")
	}

	scopeData := &ScopeData{
		ScopeType: constants.BridgeScopeType,
		Name:      "other",
		Gateway:   net.IPv4(0, 0, 0, 0),
	}
	otherScope, err := ctx.NewScope(context.TODO(), scopeData)
	if err != nil {
		t.Fatalf("failed to add scope")
	}

	hBar := newContainer("bar")

	var tests = []struct {
		aec   func(h *exec.Handle, s *Scope) (types.BaseVirtualDevice, error)
		h     *exec.Handle
		s     *spec.VirtualMachineConfigSpec
		scope string
		ip    net.IP
		err   error
	}{
		// nil handle
		{nil, nil, nil, "", nil, fmt.Errorf("")},
		// scope not found
		{nil, h, nil, "foo", nil, ResourceNotFoundError{}},
		// addEthernetCard returns error
		{aecErr, h, nil, "default", nil, fmt.Errorf("")},
		// add a container
		{nil, h, nil, "default", nil, nil},
		// container already added
		// requires that we preserve h.ExecConfig & h.Spec
		{nil, h, nil, "default", nil, nil},
		{nil, hBar, specWithEthCard, "default", nil, nil},
		{nil, hBar, nil, otherScope.Name(), nil, nil},
	}

	origAEC := addEthernetCard
	defer func() { addEthernetCard = origAEC }()

	for i, te := range tests {
		// setup
		addEthernetCard = origAEC
		scopy := &spec.VirtualMachineConfigSpec{}
		if te.h != nil {
			// seed with a specific spec state
			if te.s != nil {
				te.h.Spec = te.s
			}

			if te.h.Spec != nil {
				*scopy = *te.h.Spec
			} else {
				te.h.Spec = &spec.VirtualMachineConfigSpec{
					VirtualMachineConfigSpec: &types.VirtualMachineConfigSpec{},
				}
			}
		}

		if te.aec != nil {
			addEthernetCard = te.aec
		}

		options := &AddContainerOptions{
			Scope: te.scope,
			IP:    te.ip,
		}
		err := ctx.AddContainer(te.h, options)
		if te.err != nil {
			// expect an error
			if err == nil {
				t.Fatalf("case %d: ctx.AddContainer(%v, %s, %s) => nil want err", i, te.h, te.scope, te.ip)
			}

			if reflect.TypeOf(err) != reflect.TypeOf(te.err) {
				t.Fatalf("case %d: ctx.AddContainer(%v, %s, %s) => (%v, %v) want (%v, %v)", i, te.h, te.scope, te.ip, err, te.err, err, te.err)
			}

			if _, ok := te.err.(DuplicateResourceError); ok {
				continue
			}

			// verify no device changes in the spec
			if te.s != nil {
				if len(scopy.DeviceChange) != len(h.Spec.DeviceChange) {
					t.Fatalf("case %d: ctx.AddContainer(%v, %s, %s) added device", i, te.h, te.scope, te.ip)
				}
			}

			continue
		}

		if err != nil {
			t.Fatalf("case %d: ctx.AddContainer(%v, %s, %s) => %s want nil", i, te.h, te.scope, te.ip, err)
		}

		// verify the container was not added to the scope
		s, _ := ctx.resolveScope(te.scope)
		if s != nil && te.h != nil {
			c := s.Container(uid.Parse(te.h.ExecConfig.ID))
			if c != nil {
				t.Fatalf("case %d: ctx.AddContainer(%v, %s, %s) added container", i, te.h, te.scope, te.ip)
			}
		}

		// spec should have a nic attached to the scope's network
		var dev types.BaseVirtualDevice
		dcs, err := te.h.Spec.FindNICs(context.TODO(), s.Network())
		if len(dcs) == 0 {
			t.Fatalf("case %d: ctx.AddContainer(%v, %s, %s) no NIC was added for scope %s", i, te.h, te.scope, te.ip, s.Network())
		} else if len(dcs) > 1 {
			t.Fatalf("case %d: ctx.AddContainer(%v, %s, %s) more than one NIC added for scope %s", i, te.h, te.scope, te.ip, s.Network())
		}
		dev = dcs[0].GetVirtualDeviceConfigSpec().Device
		if spec.VirtualDeviceSlotNumber(dev) == constants.NilSlot {
			t.Fatalf("case %d: ctx.AddContainer(%v, %s, %s) NIC added has nil pci slot", i, te.h, te.scope, te.ip)
		}

		// spec metadata should be updated with endpoint info
		ne, ok := te.h.ExecConfig.Networks[s.Name()]
		if !ok {
			t.Fatalf("case %d: ctx.AddContainer(%v, %s, %s) no network endpoint info added", i, te.h, te.scope, te.ip)
		}

		if spec.VirtualDeviceSlotNumber(dev) != atoiOrZero(ne.ID) {
			t.Fatalf("case %d; ctx.AddContainer(%v, %s, %s) => ne.ID == %d, want %d", i, te.h, te.scope, te.ip, atoiOrZero(ne.ID), spec.VirtualDeviceSlotNumber(dev))
		}

		if ne.Network.Name != s.Name() {
			t.Fatalf("case %d; ctx.AddContainer(%v, %s, %s) => ne.NetworkName == %s, want %s", i, te.h, te.scope, te.ip, ne.Network.Name, s.Name())
		}

		if te.ip != nil && (!ne.Static || !te.ip.Equal(ne.IP.IP)) {
			t.Fatalf("case %d; ctx.AddContainer(%v, %s, %s) => ne.IP.IP=%s ne.Static=%v, want ne.IP.IP=%s ne.Static=%v", i, te.h, te.scope, te.ip, ne.IP.IP, ne.Static, te.ip, true)
		}

		if te.ip == nil && (ne.Static || ne.IP != nil) {
			t.Fatalf("case %d; ctx.AddContainer(%v, %s, %s) => ne.Static=%v, want ne.Static=%v", i, te.h, te.scope, te.ip, ne.Static, false)
		}
	}
}

func newContainer(name string) *exec.Handle {
	h := exec.TestHandle(uid.New().String())
	h.ExecConfig.ExecutorConfigCommon.Name = name
	return h
}

func TestContextBindUnbindContainer(t *testing.T) {
	ctx, err := NewContext(testConfig(), nil)
	if err != nil {
		t.Fatalf("NewContext() => (nil, %s), want (ctx, nil)", err)
	}
	op := trace.NewOperation(context.Background(), "TestContextBindUnbindContainer")

	scopeData := &ScopeData{
		ScopeType: constants.BridgeScopeType,
		Name:      "scope",
	}
	scope, err := ctx.NewScope(context.TODO(), scopeData)
	if err != nil {
		t.Fatalf("ctx.NewScope(%+v) => (nil, %s)", scopeData, err)
	}

	foo := newContainer("foo")
	added := newContainer("added")
	staticIP := newContainer("staticIP")
	ipErr := newContainer("ipErr")

	options := &AddContainerOptions{
		Scope: ctx.DefaultScope().Name(),
	}
	// add a container to the default scope
	if err = ctx.AddContainer(added, options); err != nil {
		t.Fatalf("ctx.AddContainer(%s, %s, nil) => %s", added, ctx.DefaultScope().Name(), err)
	}

	// add a container with a static IP
	ip := net.IPv4(172, 16, 0, 10)
	options = &AddContainerOptions{
		Scope: ctx.DefaultScope().Name(),
		IP:    ip,
	}
	if err = ctx.AddContainer(staticIP, options); err != nil {
		t.Fatalf("ctx.AddContainer(%s, %s, nil) => %s", staticIP, ctx.DefaultScope().Name(), err)
	}

	// add the "added" container to the "scope" scope
	options = &AddContainerOptions{
		Scope: scope.Name(),
	}
	if err = ctx.AddContainer(added, options); err != nil {
		t.Fatalf("ctx.AddContainer(%s, %s, nil) => %s", added, scope.Name(), err)
	}

	// add a container with an ip that is already taken,
	// causing Scope.BindContainer call to fail
	gw := ctx.DefaultScope().Gateway()
	options = &AddContainerOptions{
		Scope: scope.Name(),
	}
	ctx.AddContainer(ipErr, options)

	options = &AddContainerOptions{
		Scope: ctx.DefaultScope().Name(),
		IP:    gw,
	}
	ctx.AddContainer(ipErr, options)

	var tests = []struct {
		h      *exec.Handle
		scopes []string
		ips    []net.IP
		static bool
		err    error
	}{
		// no scopes to bind to
		{foo, []string{}, []net.IP{}, false, nil},
		// container has bad ip address
		{ipErr, []string{}, nil, false, fmt.Errorf("")},
		// successful container bind
		{added, []string{ctx.DefaultScope().Name(), scope.Name()}, []net.IP{net.IPv4(172, 16, 0, 2), net.IPv4(172, 17, 0, 2)}, false, nil},
		{staticIP, []string{ctx.DefaultScope().Name()}, []net.IP{net.IPv4(172, 16, 0, 10)}, true, nil},
	}

	for i, te := range tests {
		eps, err := ctx.BindContainer(op, te.h)
		if te.err != nil {
			// expect an error
			if err == nil || eps != nil {
				t.Fatalf("%d: ctx.BindContainer(%s) => (%+v, %+v), want (%+v, %+v)", i, te.h, eps, err, nil, te.err)
			}

			con := ctx.Container(te.h.ExecConfig.ID)
			if con != nil {
				t.Fatalf("%d: ctx.BindContainer(%s) added container %#v", i, te.h, con)
			}

			continue
		}

		if len(te.h.ExecConfig.Networks) == 0 {
			continue
		}

		// check if the correct endpoints were added
		con := ctx.Container(te.h.ExecConfig.ID)
		if con == nil {
			t.Fatalf("%d: ctx.Container(%s) => nil, want %s", i, te.h.ExecConfig.ID, te.h.ExecConfig.ID)
		}

		if len(con.Scopes()) != len(te.scopes) {
			t.Fatalf("%d: len(con.Scopes()) %#v != len(te.scopes) %#v", i, con.Scopes(), te.scopes)
		}

		// check endpoints
		for i, s := range te.scopes {
			found := false
			for _, e := range eps {
				if e.Scope().Name() != s {
					continue
				}

				found = true
				if !e.Gateway().Equal(e.Scope().Gateway()) {
					t.Fatalf("%d: ctx.BindContainer(%s) => endpoint gateway %s, want %s", i, te.h, e.Gateway(), e.Scope().Gateway())
				}
				if !e.IP().Equal(te.ips[i]) {
					t.Fatalf("%d: ctx.BindContainer(%s) => endpoint IP %s, want %s", i, te.h, e.IP(), te.ips[i])
				}
				if e.Subnet().String() != e.Scope().Subnet().String() {
					t.Fatalf("%d: ctx.BindContainer(%s) => endpoint subnet %s, want %s", i, te.h, e.Subnet(), e.Scope().Subnet())
				}

				ne := te.h.ExecConfig.Networks[s]
				if !ne.IP.IP.Equal(te.ips[i]) {
					t.Fatalf("%d: ctx.BindContainer(%s) => metadata endpoint IP %s, want %s", i, te.h, ne.IP.IP, te.ips[i])
				}
				if ne.IP.Mask.String() != e.Scope().Subnet().Mask.String() {
					t.Fatalf("%d: ctx.BindContainer(%s) => metadata endpoint IP mask %s, want %s", i, te.h, ne.IP.Mask.String(), e.Scope().Subnet().Mask.String())
				}
				if !ne.Network.Gateway.IP.Equal(e.Scope().Gateway()) {
					t.Fatalf("%d: ctx.BindContainer(%s) => metadata endpoint gateway %s, want %s", i, te.h, ne.Network.Gateway.IP, e.Scope().Gateway())
				}
				if ne.Network.Gateway.Mask.String() != e.Scope().Subnet().Mask.String() {
					t.Fatalf("%d: ctx.BindContainer(%s) => metadata endpoint gateway mask %s, want %s", i, te.h, ne.Network.Gateway.Mask.String(), e.Scope().Subnet().Mask.String())
				}

				break
			}

			if !found {
				t.Fatalf("%d: ctx.BindContainer(%s) => endpoint for scope %s not added", i, te.h, s)
			}
		}
	}

	tests = []struct {
		h      *exec.Handle
		scopes []string
		ips    []net.IP
		static bool
		err    error
	}{
		// container not bound
		{foo, []string{}, nil, false, nil},
		// successful container unbind
		{added, []string{ctx.DefaultScope().Name(), scope.Name()}, nil, false, nil},
		{staticIP, []string{ctx.DefaultScope().Name()}, nil, true, nil},
	}

	// test UnbindContainer
	for i, te := range tests {
		op := trace.NewOperation(context.Background(), "Testing..")
		eps, err := ctx.UnbindContainer(op, te.h)
		if te.err != nil {
			if err == nil {
				t.Fatalf("%d: ctx.UnbindContainer(%s) => nil, want err", i, te.h)
			}

			continue
		}

		// container should not be there
		con := ctx.Container(te.h.ExecConfig.ID)
		if con != nil {
			t.Fatalf("%d: ctx.Container(%s) => %#v, want nil", i, te.h, con)
		}

		for _, s := range te.scopes {
			found := false
			for _, e := range eps {
				if e.Scope().Name() == s {
					found = true
				}
			}

			if !found {
				t.Fatalf("%d: ctx.UnbindContainer(%s) did not return endpoint for scope %s. Endpoints: %+v", i, te.h, s, eps)
			}

			// container should not be part of scope
			scopes, err := ctx.findScopes(&s)
			if err != nil || len(scopes) != 1 {
				t.Fatalf("%d: ctx.Scopes(%s) => (%#v, %#v)", i, s, scopes, err)
			}
			if scopes[0].Container(uid.Parse(te.h.ExecConfig.ID)) != nil {
				t.Fatalf("%d: container %s is still part of scope %s", i, te.h.ExecConfig.ID, s)
			}

			// check if endpoint is still there, but without the ip
			ne, ok := te.h.ExecConfig.Networks[s]
			if !ok {
				t.Fatalf("%d: container endpoint not present in %v", i, te.h.ExecConfig)
			}

			if te.static != ne.Static {
				t.Fatalf("%d: ne.Static=%v, want %v", i, ne.Static, te.static)
			}
		}
	}
}

func TestContextRemoveContainer(t *testing.T) {

	op := trace.NewOperation(context.Background(), "TestContextRemoveContainer")
	hFoo := newContainer("foo")

	ctx, err := NewContext(testConfig(), nil)
	if err != nil {
		t.Fatalf("NewContext() => (nil, %s), want (ctx, nil)", err)
	}

	scopeData := &ScopeData{
		ScopeType: constants.BridgeScopeType,
		Name:      "scope",
	}
	scope, err := ctx.NewScope(context.TODO(), scopeData)
	if err != nil {
		t.Fatalf("ctx.NewScope() => (nil, %s), want (scope, nil)", err)
	}

	options := &AddContainerOptions{
		Scope: scope.Name(),
	}
	ctx.AddContainer(hFoo, options)
	ctx.BindContainer(op, hFoo)

	// container that is added to multiple bridge scopes
	hBar := newContainer("bar")
	options.Scope = "default"
	ctx.AddContainer(hBar, options)
	options.Scope = scope.Name()
	ctx.AddContainer(hBar, options)

	var tests = []struct {
		h     *exec.Handle
		scope string
		err   error
	}{
		{nil, "", fmt.Errorf("")},                        // nil handle
		{hBar, "bar", fmt.Errorf("")},                    // scope not found
		{hFoo, scope.Name(), fmt.Errorf("")},             // bound container
		{newContainer("baz"), "default", fmt.Errorf("")}, // container not part of scope
		{hBar, "default", nil},
		{hBar, scope.Name(), nil},
	}

	for i, te := range tests {
		var ne *executor.NetworkEndpoint
		if te.h != nil && te.h.ExecConfig.Networks != nil {
			ne = te.h.ExecConfig.Networks[te.scope]
		}

		err = ctx.RemoveContainer(te.h, te.scope)
		if te.err != nil {
			// expect error
			if err == nil {
				t.Fatalf("%d: ctx.RemoveContainer(%#v, %s) => nil want err", i, te.h, te.scope)
			}

			continue
		}

		s, err := ctx.resolveScope(te.scope)
		if err != nil {
			t.Fatalf(err.Error())
		}

		if s.Container(uid.Parse(te.h.ExecConfig.ID)) != nil {
			t.Fatalf("container %s is part of scope %s", te.h, s.Name())
		}

		// should have a remove spec for NIC, if container was only part of one bridge scope
		dcs, err := te.h.Spec.FindNICs(context.TODO(), s.Network())
		if err != nil {
			t.Fatalf(err.Error())
		}

		found := false
		var d types.BaseVirtualDevice
		for _, dc := range dcs {
			if dc.GetVirtualDeviceConfigSpec().Operation != types.VirtualDeviceConfigSpecOperationRemove {
				continue
			}

			d = dc.GetVirtualDeviceConfigSpec().Device
			found = true
			break
		}

		// if a remove spec for the NIC was found, check if any other
		// network endpoints are still using it
		if found {
			for _, ne := range te.h.ExecConfig.Networks {
				if atoiOrZero(ne.ID) == spec.VirtualDeviceSlotNumber(d) {
					t.Fatalf("%d: NIC with pci slot %d is still in use by a network endpoint %#v", i, spec.VirtualDeviceSlotNumber(d), ne)
				}
			}
		} else if ne != nil {
			// check if remove spec for NIC should have been there
			for _, ne2 := range te.h.ExecConfig.Networks {
				if ne.ID == ne2.ID {
					t.Fatalf("%d: NIC with pci slot %s should have been removed", i, ne.ID)
				}
			}
		}

		// metadata should be gone
		if _, ok := te.h.ExecConfig.Networks[te.scope]; ok {
			t.Fatalf("%d: endpoint metadata for container still present in handle %#v", i, te.h.ExecConfig)
		}
	}
}

func TestDeleteScope(t *testing.T) {
	kv := &kvstore.MockKeyValueStore{}
	kv.On("List", mock.Anything).Return(nil, nil)
	kv.On("Put", context.TODO(), mock.Anything, mock.Anything).Return(nil)
	kv.On("Delete", context.TODO(), mock.Anything).Return(nil)
	ctx, err := NewContext(testConfig(), kv)
	if err != nil {
		t.Fatalf("NewContext() => (nil, %s), want (ctx, nil)", err)
	}
	op := trace.NewOperation(context.Background(), "TestDeleteScope")

	scopeData := &ScopeData{
		ScopeType: constants.BridgeScopeType,
		Name:      "foo",
	}
	foo, err := ctx.NewScope(context.TODO(), scopeData)
	if err != nil {
		t.Fatalf("ctx.NewScope(%+v) => (nil, %#v), want (foo, nil)", scopeData, err)
	}
	h := newContainer("container")
	options := &AddContainerOptions{
		Scope: foo.Name(),
	}
	ctx.AddContainer(h, options)

	// bar is a scope with bound endpoints
	scopeData = &ScopeData{
		ScopeType: constants.BridgeScopeType,
		Name:      "bar",
	}
	bar, err := ctx.NewScope(context.TODO(), scopeData)
	if err != nil {
		t.Fatalf("ctx.NewScope(%+v) => (nil, %#v), want (bar, nil)", scopeData, err)
	}

	h = newContainer("container2")
	options.Scope = bar.Name()
	ctx.AddContainer(h, options)
	ctx.BindContainer(op, h)

	scopeData = &ScopeData{
		ScopeType: constants.BridgeScopeType,
		Name:      "bazScope",
	}
	baz, err := ctx.NewScope(context.TODO(), scopeData)
	if err != nil {
		t.Fatalf("ctx.NewScope(%+v) => (nil, %#v), want (baz, nil)", scopeData, err)
	}

	scopeData = &ScopeData{
		ScopeType: constants.BridgeScopeType,
		Name:      "quxScope",
	}
	qux, err := ctx.NewScope(context.TODO(), scopeData)
	if err != nil {
		t.Fatalf("ctx.NewScope(%+v) => (nil, %#v), want (qux, nil)", scopeData, err)
	}

	var tests = []struct {
		name string
		err  error
	}{
		{"", ResourceNotFoundError{}},
		{ctx.DefaultScope().Name(), fmt.Errorf("cannot delete builtin scopes")},
		{bar.Name(), fmt.Errorf("cannot delete scope with bound endpoints")},
		// full name
		{foo.Name(), nil},
		// full id
		{baz.ID().String(), nil},
		// partial id
		{qux.ID().String()[:6], nil},
	}

	calls := 0
	for _, te := range tests {
		err := ctx.DeleteScope(context.TODO(), te.name)
		if te.err != nil {
			if err == nil {
				t.Fatalf("DeleteScope(%s) => nil, expected err", te.name)
			}

			if reflect.TypeOf(te.err) != reflect.TypeOf(err) {
				t.Fatalf("DeleteScope(%s) => %#v, want %#v", te.name, err, te.err)
			}

			kv.AssertNumberOfCalls(t, "Delete", calls)
			continue
		}

		calls++
		kv.AssertNumberOfCalls(t, "Delete", calls)
		scopes, err := ctx.findScopes(&te.name)
		if _, ok := err.(ResourceNotFoundError); !ok || len(scopes) != 0 {
			t.Fatalf("scope %s not deleted", te.name)
		}
	}
}

func TestAliases(t *testing.T) {
	ctx, err := NewContext(testConfig(), nil)
	assert.NoError(t, err)
	assert.NotNil(t, ctx)

	op := trace.NewOperation(context.Background(), "TestAliases")
	scope := ctx.DefaultScope()

	var tests = []struct {
		con     string
		aliases []string
		err     error
	}{
		// bad alias
		{"bad1", []string{"bad1"}, assert.AnError},
		{"bad2", []string{"foo:bar:baz"}, assert.AnError},
		// ok
		{"c1", []string{"c2:other", ":c1", ":c1"}, nil},
		{"c2", []string{"c1:other"}, nil},
		{"c3", []string{"c2:c2", "c1:c1"}, nil},
	}

	containers := make(map[string]*exec.Handle)
	for _, te := range tests {
		t.Logf("%+v", te)
		c := newContainer(te.con)

		opts := &AddContainerOptions{
			Scope:   scope.Name(),
			Aliases: te.aliases,
		}

		err = ctx.AddContainer(c, opts)
		assert.NoError(t, err)
		assert.EqualValues(t, opts.Aliases, c.ExecConfig.Networks[scope.Name()].Network.Aliases)

		eps, err := ctx.BindContainer(op, c)
		if te.err != nil {
			assert.Error(t, err)
			assert.Empty(t, eps)
			continue
		}

		assert.NoError(t, err)
		assert.Len(t, eps, 1)

		// verify aliases are present
		assert.NotNil(t, ctx.Container(c.ExecConfig.ID))
		assert.NotNil(t, ctx.Container(uid.Parse(c.ExecConfig.ID).Truncate().String()))
		assert.NotNil(t, ctx.Container(c.ExecConfig.Name))
		assert.NotNil(t, ctx.ContainersByAlias(fmt.Sprintf("%s:%s", scope.Name(), uid.Parse(c.ExecConfig.ID).Truncate())))
		assert.NotNil(t, ctx.ContainersByAlias(fmt.Sprintf("%s:%s", scope.Name(), c.ExecConfig.Name)))

		aliases := c.ExecConfig.Networks[scope.Name()].Network.Aliases
		for _, a := range aliases {
			l := strings.Split(a, ":")
			con, al := l[0], l[1]
			found := false
			var ea alias
			for _, a := range eps[0].getAliases(con) {
				if al == a.Name {
					found = true
					ea = a
					break
				}
			}
			assert.True(t, found, "alias %s not found for container %s", al, con)

			// if the aliased container is bound we should be able to look it up with
			// the scoped alias name
			if c := ctx.Container(ea.Container); c != nil {
				assert.NotNil(t, ctx.ContainersByAlias(ea.scopedName()))
			} else {
				assert.Nil(t, ctx.ContainersByAlias(ea.scopedName()), "scoped name=%s", ea.scopedName())
			}
		}

		// now that the container is bound, there
		// should be additional aliases scoped to
		// other containers
		for _, e := range scope.Endpoints() {
			for _, a := range e.getAliases(c.ExecConfig.Name) {
				t.Logf("alias: %s", a.scopedName())
				assert.NotNil(t, ctx.ContainersByAlias(a.scopedName()))
			}
		}

		containers[te.con] = c
	}

	t.Logf("containers: %#v", ctx.containers)

	c := containers["c2"]
	_, err = ctx.UnbindContainer(op, c)
	assert.NoError(t, err)
	// verify aliases are gone
	assert.Nil(t, ctx.Container(c.ExecConfig.ID))
	assert.Nil(t, ctx.Container(uid.Parse(c.ExecConfig.ID).Truncate().String()))
	assert.Nil(t, ctx.Container(c.ExecConfig.Name))
	assert.Nil(t, ctx.Container(fmt.Sprintf("%s:%s", scope.Name(), c.ExecConfig.Name)))
	assert.Nil(t, ctx.Container(fmt.Sprintf("%s:%s", scope.Name(), uid.Parse(c.ExecConfig.ID).Truncate())))

	// aliases from c1 and c3 to c2 should not resolve anymore
	assert.Nil(t, ctx.Container(fmt.Sprintf("%s:c1:other", scope.Name())))
	assert.Nil(t, ctx.Container(fmt.Sprintf("%s:c3:c2", scope.Name())))
}

func TestLoadScopesFromKV(t *testing.T) {
	// sample kv store data
	var tests = []struct {
		pg string
		sn string
		s  *Scope
	}{
		{
			pg: "bridge",
			sn: "foo",
			s: &Scope{
				id:         uid.New(),
				name:       "foo",
				scopeType:  constants.BridgeScopeType,
				subnet:     &net.IPNet{IP: net.ParseIP("10.10.10.0"), Mask: net.CIDRMask(16, 32)},
				gateway:    net.ParseIP("10.10.10.1"),
				containers: map[uid.UID]*Container{},
				spaces:     []*AddressSpace{NewAddressSpaceFromNetwork(&net.IPNet{IP: net.ParseIP("10.10.10.0"), Mask: net.CIDRMask(16, 32)})},
			},
		},
		{
			pg: "bridge",
			sn: "bar",
			s: &Scope{
				id:         uid.New(),
				name:       "bar",
				scopeType:  constants.BridgeScopeType,
				subnet:     &net.IPNet{IP: net.ParseIP("10.11.0.0"), Mask: net.CIDRMask(16, 32)},
				gateway:    net.ParseIP("10.11.0.1"),
				containers: map[uid.UID]*Container{},
				dns:        []net.IP{net.ParseIP("8.8.8.8")},
				spaces:     []*AddressSpace{NewAddressSpaceFromNetwork(&net.IPNet{IP: net.ParseIP("10.11.0.0"), Mask: net.CIDRMask(16, 32)})},
			},
		},
		{
			pg: "ext",
			sn: "ext",
			s: &Scope{
				id:         uid.New(),
				name:       "ext",
				scopeType:  constants.ExternalScopeType,
				subnet:     &net.IPNet{IP: net.ParseIP("10.12.0.0"), Mask: net.CIDRMask(16, 32)},
				gateway:    net.ParseIP("10.12.0.1"),
				containers: map[uid.UID]*Container{},
				dns:        []net.IP{net.ParseIP("8.8.8.8")},
				spaces:     []*AddressSpace{NewAddressSpaceFromNetwork(&net.IPNet{IP: net.ParseIP("10.12.0.0"), Mask: net.CIDRMask(16, 32)})},
			},
		},
		{
			sn: "bad",
		},
	}

	// load the kv store data
	kvdata := map[string][]byte{}
	for _, te := range tests {
		var d []byte
		if te.s != nil {
			var err error
			d, err = te.s.MarshalJSON()
			assert.NoError(t, err)
		}

		kvdata[scopeKey(te.sn)] = d
	}

	// cases where there is no data in the kv store,
	// or kv.List returns error
	for _, e := range []error{nil, kvstore.ErrKeyNotFound, assert.AnError} {
		kv := &kvstore.MockKeyValueStore{}
		kv.On("List", `context\.scopes\..+`).Return(nil, e)
		ctx, err := NewContext(testConfig(), kv)
		assert.NoError(t, err)
		assert.NotNil(t, ctx)

		// check to see if the only networks
		// are the ones in the config
		scs, err := ctx.Scopes(context.TODO(), nil)
		assert.NoError(t, err)
		assert.Len(t, scs, len(testConfig().ContainerNetworks))
	}

	// kv.List returns kvdata
	kv := &kvstore.MockKeyValueStore{}
	kv.On("List", `context\.scopes\..+`).Return(kvdata, nil)
	ctx, err := NewContext(testConfig(), kv)
	assert.NoError(t, err)
	assert.NotNil(t, ctx)
	for _, te := range tests {
		scs, err := ctx.Scopes(context.TODO(), &te.sn)
		if te.s == nil || ctx.config.PortGroups[te.pg] == nil {
			assert.Error(t, err)
			assert.Len(t, scs, 0)
			continue
		}

		assert.NoError(t, err)
		assert.Len(t, scs, 1)

		assert.Equal(t, scs[0].Name(), te.s.Name())
		assert.Equal(t, scs[0].ID(), te.s.ID())
		assert.Equal(t, scs[0].Type(), te.s.Type())
		assert.True(t, scs[0].Subnet().IP.Equal(te.s.Subnet().IP))
		assert.Equal(t, scs[0].Subnet().Mask, te.s.Subnet().Mask)
		assert.True(t, scs[0].Gateway().Equal(te.s.Gateway()))
		assert.EqualValues(t, scs[0].DNS(), te.s.DNS())
		assert.Len(t, te.s.Pools(), len(scs[0].Pools()))
		for _, p := range te.s.Pools() {
			found := false
			for _, p2 := range scs[0].Pools() {
				if p2.Equal(p) {
					found = true
					break
				}
			}

			assert.True(t, found)
		}
	}
}

func TestKVStoreSetFails(t *testing.T) {
	sn := "foo"

	// set up kv
	kv := &kvstore.MockKeyValueStore{}
	kv.On("List", mock.Anything).Return(nil, nil)
	kv.On("Put", context.TODO(), scopeKey(sn), mock.Anything).Return(assert.AnError)

	ctx, err := NewContext(testConfig(), kv)
	assert.NoError(t, err)
	assert.NotNil(t, ctx)

	scopeData := &ScopeData{
		ScopeType: constants.BridgeScopeType,
		Name:      sn,
	}
	_, err = ctx.NewScope(context.TODO(), scopeData)
	assert.EqualError(t, err, assert.AnError.Error())
	scs, err := ctx.Scopes(context.TODO(), &sn)
	assert.Error(t, err)
	assert.IsType(t, ResourceNotFoundError{}, err)
	assert.Len(t, scs, 0)
}

func TestDeleteScopeFreeAddressSpace(t *testing.T) {
	ctx, err := NewContext(testConfig(), nil)
	assert.NoError(t, err)
	assert.NotNil(t, ctx)

	maskOnes, _ := ctx.defaultBridgeMask.Size()
	poolOnes, _ := ctx.defaultBridgePool.Network.Mask.Size()
	maxNets := int(math.Pow(2, float64(maskOnes-poolOnes)))

	// maxNets-1 since there is a default network
	// called "bridge" already
	scopes := make([]*Scope, maxNets-1)
	// create the maximum number of networks allowed
	for i := range scopes {
		s, err := ctx.NewScope(context.TODO(), &ScopeData{
			ScopeType: constants.BridgeScopeType,
			Name:      fmt.Sprintf("foo%d", i),
		})
		assert.NoError(t, err)
		assert.NotNil(t, s)
		scopes[i] = s
	}

	// pick a random scope to delete
	rand.Seed(time.Now().Unix())
	t.Log(scopes)
	s := scopes[rand.Intn(len(scopes))]
	subnet := s.Subnet()
	t.Log(subnet)

	err = ctx.DeleteScope(context.TODO(), s.Name())
	assert.NoError(t, err)

	// create a new scope and check if we got
	// the subnet for the scope we just
	// deleted
	s, err = ctx.NewScope(context.TODO(), &ScopeData{
		ScopeType: constants.BridgeScopeType,
		Name:      "bar",
	})
	assert.NotNil(t, s)
	assert.NoError(t, err)
	assert.Equal(t, subnet.String(), s.Subnet().String())
}

func TestDeleteScopeLimits(t *testing.T) {
	ctx, err := NewContext(testConfig(), nil)
	assert.NoError(t, err)
	assert.NotNil(t, ctx)

	// repeatedly create and delete a scope
	// to make sure we are free'ing address
	// space for the scope
	const numTries = 100
	scopes, err := ctx.Scopes(context.TODO(), nil)
	assert.NoError(t, err)
	numScopes := len(scopes)
	for i := 0; i < numTries; i++ {
		s, err := ctx.NewScope(context.TODO(), &ScopeData{
			ScopeType: constants.BridgeScopeType,
			Name:      "foo",
		})
		assert.NotNil(t, s)
		assert.NoError(t, err)
		if err != nil {
			t.FailNow()
		}

		err = ctx.DeleteScope(context.TODO(), s.Name())
		assert.NoError(t, err)
		if err != nil {
			t.FailNow()
		}
	}

	scopes, err = ctx.Scopes(context.TODO(), nil)
	assert.NoError(t, err)
	assert.Equal(t, numScopes, len(scopes))
}
