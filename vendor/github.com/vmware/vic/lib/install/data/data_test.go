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

package data

import (
	"net"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/vmware/vic/cmd/vic-machine/common"
	"github.com/vmware/vic/lib/config/executor"
	"github.com/vmware/vic/pkg/ip"
)

func TestCopyNonEmpty(t *testing.T) {
	d := NewData()
	s := NewData()

	ipAddr, mask, _ := net.ParseCIDR("1.1.1.1/32")
	s.ClientNetwork.IP.IP = ipAddr
	s.ClientNetwork.IP.Mask = mask.Mask

	d.PublicNetwork.IP.IP = ipAddr
	d.PublicNetwork.IP.Mask = mask.Mask

	d.CopyNonEmpty(s)
	assert.Equal(t, s.ClientNetwork.IP.IP, d.ClientNetwork.IP.IP, "client ip is not right")
	assert.False(t, d.PublicNetwork.IP.IP.Equal(s.PublicNetwork.IP.IP), "public ip should not be changed")
	assert.Equal(t, d.OpsCredentials, s.OpsCredentials)

	opsUser := "ops"
	opsPassword := "pass"
	s.OpsCredentials = common.OpsCredentials{
		OpsUser:     &opsUser,
		OpsPassword: &opsPassword,
	}
	d.CopyNonEmpty(s)
	assert.NotEqual(t, d.OpsCredentials, s.OpsCredentials)

	s.OpsCredentials.IsSet = true
	d.CopyNonEmpty(s)
	assert.Equal(t, d.OpsCredentials, s.OpsCredentials)

	s.Target.Thumbprint = "te:st:th:um:bp:ri:nt"
	d.CopyNonEmpty(s)
	assert.Equal(t, d.Target.Thumbprint, s.Target.Thumbprint)
}

func TestEqualIPRanges(t *testing.T) {
	emptyIPRange := ip.Range{}
	assert.True(t, equalIPRanges([]ip.Range{emptyIPRange}, []ip.Range{emptyIPRange}))

	fooRange := ip.Range{
		FirstIP: net.ParseIP("10.10.10.10"),
		LastIP:  net.ParseIP("10.10.10.24"),
	}
	barRange := ip.Range{
		FirstIP: net.ParseIP("10.10.10.10"),
		LastIP:  net.ParseIP("10.10.10.16"),
	}
	assert.False(t, equalIPRanges([]ip.Range{fooRange}, []ip.Range{barRange}))
}

func TestEqualIPSlices(t *testing.T) {
	emptyIP := net.IP{}
	assert.True(t, equalIPSlices([]net.IP{emptyIP}, []net.IP{emptyIP}))

	fooIP, _, _ := net.ParseCIDR("1.1.1.1/32")
	barIP, _, _ := net.ParseCIDR("2.2.2.2/32")
	assert.False(t, equalIPSlices([]net.IP{fooIP}, []net.IP{barIP}))
}

func TestCopyContainerNetworks(t *testing.T) {
	d := NewData()
	src := NewData()

	fooLabel := "foo"
	fooNet := "Foo Network"
	ipAddr, mask, _ := net.ParseCIDR("1.1.1.1/32")
	ipRange := ip.Range{
		FirstIP: net.ParseIP("10.10.10.10"),
		LastIP:  net.ParseIP("10.10.10.24"),
	}
	trust := executor.Closed
	src.ContainerNetworks.MappedNetworks[fooLabel] = fooNet
	src.ContainerNetworks.MappedNetworksGateways[fooLabel] = *mask
	src.ContainerNetworks.MappedNetworksIPRanges[fooLabel] = []ip.Range{ipRange}
	src.ContainerNetworks.MappedNetworksDNS[fooLabel] = []net.IP{ipAddr}
	src.ContainerNetworks.MappedNetworksFirewalls[fooLabel] = trust

	// Everything in src should be copied to d.
	err := d.copyContainerNetworks(src)
	assert.NoError(t, err)
	assert.Equal(t, d.ContainerNetworks, src.ContainerNetworks)

	barLabel := "bar"
	barNet := "Bar Network"
	src.ContainerNetworks.MappedNetworks[barLabel] = barNet
	src.ContainerNetworks.MappedNetworksGateways[barLabel] = net.IPNet{}
	src.ContainerNetworks.MappedNetworksIPRanges[barLabel] = []ip.Range{}
	src.ContainerNetworks.MappedNetworksDNS[barLabel] = []net.IP{}
	src.ContainerNetworks.MappedNetworksFirewalls[barLabel] = executor.Published

	// The new network should be copied to d.
	err = d.copyContainerNetworks(src)
	assert.NoError(t, err)
	assert.Equal(t, d.ContainerNetworks, src.ContainerNetworks)

	delete(src.ContainerNetworks.MappedNetworks, barLabel)
	delete(src.ContainerNetworks.MappedNetworksGateways, barLabel)
	delete(src.ContainerNetworks.MappedNetworksIPRanges, barLabel)
	delete(src.ContainerNetworks.MappedNetworksDNS, barLabel)
	delete(src.ContainerNetworks.MappedNetworksFirewalls, barLabel)

	// There should be an error if anything in d is not in src.
	err = d.copyContainerNetworks(src)
	assert.NotNil(t, err)

	src.ContainerNetworks.MappedNetworks[barLabel] = barNet
	src.ContainerNetworks.MappedNetworksGateways[barLabel] = net.IPNet{}
	src.ContainerNetworks.MappedNetworksIPRanges[barLabel] = []ip.Range{ipRange}
	src.ContainerNetworks.MappedNetworksDNS[barLabel] = []net.IP{ipAddr}
	src.ContainerNetworks.MappedNetworksFirewalls[barLabel] = trust

	// There should be an error on an attempt to change an existing network.
	err = d.copyContainerNetworks(src)
	assert.NotNil(t, err)
}

func TestCopyVolumeStores(t *testing.T) {
	d := NewData()
	src := NewData()
	d.VolumeLocations = make(map[string]*url.URL)
	src.VolumeLocations = make(map[string]*url.URL)

	var err error
	src.VolumeLocations["foo"], err = url.Parse("ds://fooDS/dir")
	assert.NoError(t, err)
	// Everything in src should be copied to d.
	err = d.copyVolumeStores(src)
	assert.NoError(t, err)
	assert.Equal(t, d.VolumeLocations, src.VolumeLocations)

	src.VolumeLocations["bar"], err = url.Parse("barDS/dir")
	// The new volume store should be copied to d.
	err = d.copyVolumeStores(src)
	assert.NoError(t, err)
	assert.Equal(t, d.VolumeLocations, src.VolumeLocations)

	delete(src.VolumeLocations, "bar")
	// There should be an error if anything in d is not in src.
	err = d.copyVolumeStores(src)
	assert.NotNil(t, err)

	src.VolumeLocations["bar"], err = url.Parse("barDS/path")
	assert.NoError(t, err)
	// There should be an error on an attempt to change an existing volume store.
	err = d.copyVolumeStores(src)
	assert.NotNil(t, err)
}
