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

package converter

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/vic/lib/config"
	"github.com/vmware/vic/lib/install/data"
	"github.com/vmware/vic/lib/install/validate"
	"github.com/vmware/vic/pkg/vsphere/extraconfig"
)

type testFinder struct {
}

func (f *testFinder) ObjectReference(ctx context.Context, ref types.ManagedObjectReference) (object.Reference, error) {
	return testObject{
		Common: object.NewCommon(nil, ref),
	}, nil
}

type testObject struct {
	object.Common
}

func (c testObject) Name() string {
	switch c.Common.Reference().String() {
	case "DistributedVirtualPortgroup:dvportgroup-357":
		return "management"
	case "DistributedVirtualPortgroup:dvportgroup-358":
		return "vm-network"
	case "DistributedVirtualPortgroup:dvportgroup-55":
		return "bridge"
	case "DistributedVirtualPortgroup:dvportgroup-56":
		return "external"
	default:
		return "unknown"
	}
}

func TestInit(t *testing.T) {
	var ok bool
	_, ok = kindConverters[reflect.Struct]
	assert.True(t, ok, fmt.Sprintf("Struct converter is not found"))
	_, ok = kindConverters[reflect.Slice]
	assert.True(t, ok, fmt.Sprintf("Slice converter is not found"))
	_, ok = kindConverters[reflect.Map]
	assert.True(t, ok, fmt.Sprintf("Map converter is not found"))
	_, ok = kindConverters[reflect.String]
	assert.True(t, ok, fmt.Sprintf("String converter is not found"))
	_, ok = kindConverters[reflect.Ptr]
	assert.True(t, ok, fmt.Sprintf("Pointer converter is not found"))
	_, ok = kindConverters[reflect.Int]
	assert.True(t, ok, fmt.Sprintf("Int converter is not found"))
	_, ok = kindConverters[reflect.Int8]
	assert.True(t, ok, fmt.Sprintf("Int8 converter is not found"))
	_, ok = kindConverters[reflect.Int16]
	assert.True(t, ok, fmt.Sprintf("Int16 converter is not found"))
	_, ok = kindConverters[reflect.Int32]
	assert.True(t, ok, fmt.Sprintf("Int32 converter is not found"))
	_, ok = kindConverters[reflect.Int64]
	assert.True(t, ok, fmt.Sprintf("Int64 converter is not found"))
	_, ok = kindConverters[reflect.Bool]
	assert.True(t, ok, fmt.Sprintf("Bool converter is not found"))
	_, ok = kindConverters[reflect.Float32]
	assert.True(t, ok, fmt.Sprintf("Float32 converter is not found"))
	_, ok = kindConverters[reflect.Float64]
	assert.True(t, ok, fmt.Sprintf("Float64 converter is not found"))

	_, ok = typeConverters["url.URL"]
	assert.True(t, ok, fmt.Sprintf("url.URL converter is not found"))
	_, ok = typeConverters["net.IPNet"]
	assert.True(t, ok, fmt.Sprintf("net.IPNet converter is not found"))
	_, ok = typeConverters["net.IP"]
	assert.True(t, ok, fmt.Sprintf("net.IP converter is not found"))
	_, ok = typeConverters["ip.Range"]
	assert.True(t, ok, fmt.Sprintf("ip.Range converter is not found"))
	_, ok = typeConverters["data.NetworkConfig"]
	assert.True(t, ok, fmt.Sprintf("data.NetworkConfig converter is not found"))
	_, ok = typeConverters["common.ContainerNetworks"]
	assert.True(t, ok, fmt.Sprintf("common.ContainerNetworks converter is not found"))

	_, ok = labelHandlers[keyAfterValueLabel]
	assert.True(t, ok, fmt.Sprintf("value-key handler is not found"))
	_, ok = labelHandlers[valueAfterKeyLabel]
	assert.True(t, ok, fmt.Sprintf("key-value handler is not found"))
}

func TestConvertImageStore(t *testing.T) {
	data := data.NewData()
	data.ImageDatastorePath = "ds://vsan/path"
	options, err := DataToOption(data)
	assert.Empty(t, err)
	assert.Equal(t, 1, len(options), "should not have other option generated")
	assert.Equal(t, "ds://vsan/path", options["image-store"][0], "not expected image-store option")
}

func TestConvertVolumeStore(t *testing.T) {
	tests := []struct {
		in  map[string]*url.URL
		out []string
	}{
		{
			in: map[string]*url.URL{
				"default": {
					Scheme: "ds",
					Host:   "vsan",
					Path:   "path/volume",
				},

				"noSchema": {
					Host: "vsan",
					Path: "path1",
				},
			},
			out: []string{
				"ds://vsan/path/volume:default",
				"vsan/path1:noSchema",
			},
		},
	}
	for _, test := range tests {
		data := data.NewData()
		data.VolumeLocations = test.in
		options, err := DataToOption(data)
		assert.Empty(t, err)

		assert.Equal(t, 1, len(options), "should not have other option generated")
		vols := options["volume-store"]
		assert.Equal(t, len(test.out), len(vols), "not expected length")
		for _, store := range test.out {
			found := false
			for _, actual := range vols {
				if actual == store {
					found = true
					break
				}
			}
			assert.True(t, found, fmt.Sprintf("%s is not created", store))
		}
	}
}

func TestConvertVCHName(t *testing.T) {
	data := data.NewData()
	data.DisplayName = "vch1"
	options, err := DataToOption(data)
	assert.Empty(t, err)
	assert.Equal(t, 1, len(options), "should not have other option generated")
	assert.Equal(t, "vch1", options["name"][0], "not expected name option")
}

func TestConvertTarget(t *testing.T) {
	data := data.NewData()
	data.Target.URL, _ = url.Parse("1.1.1.1")
	options, err := DataToOption(data)
	assert.Empty(t, err)
	assert.Equal(t, 1, len(options), "should not have other option generated")
	assert.Equal(t, "1.1.1.1", options["target"][0], "not expected target option")
}

func TestConvertOps(t *testing.T) {
	data := data.NewData()
	adminUser := "admin"
	data.OpsCredentials.OpsUser = &adminUser
	pass := "password"
	data.OpsCredentials.OpsPassword = &pass
	data.Thumbprint = "uuidstring"
	options, err := DataToOption(data)
	assert.Empty(t, err)

	assert.Equal(t, 2, len(options), "should not have other option generated")
	assert.Equal(t, *data.OpsCredentials.OpsUser, options["ops-user"][0], "not expected ops-user option")
	assert.Equal(t, data.Thumbprint, options["thumbprint"][0], "not expected thumbprint option")
}

func TestConvertBaseImageSize(t *testing.T) {
	data := data.NewData()
	data.ScratchSize = "8GB"
	options, err := DataToOption(data)
	assert.Empty(t, err)
	assert.Equal(t, 1, len(options), "should not have other option generated")
	assert.Equal(t, data.ScratchSize, options["base-image-size"][0], "not expected base-image-size option")
}

func testConvertBridgeNetwork(t *testing.T) {
	_, ipnet, _ := net.ParseCIDR("172.16.0.0/12")

	data := data.NewData()
	data.BridgeIPRange = ipnet
	data.BridgeNetworkName = "portgroup1"
	options, err := DataToOption(data)
	assert.Empty(t, err)
	assert.Equal(t, 2, len(options), "should not have other option generated")
	assert.Equal(t, "172.16.0.0/12", options["bridge-network-range"][0], "not expected bridge-network-range option")
	assert.Equal(t, data.BridgeNetworkName, options["bridge-network"][0], "not expected bridge-network option")
}

func TestConvertPublicNetwork(t *testing.T) {
	gwIP := net.ParseIP("172.16.0.1")
	_, destination, _ := net.ParseCIDR("172.16.0.0/12")
	ip, ipNet, _ := net.ParseCIDR("172.16.0.2/12")

	data := data.NewData()
	data.PublicNetwork.Name = "public"
	data.PublicNetwork.Gateway.IP = gwIP
	data.PublicNetwork.Gateway.Mask = ipNet.Mask
	data.PublicNetwork.IP.IP = ip
	data.PublicNetwork.IP.Mask = ipNet.Mask
	data.PublicNetwork.Destinations = append(data.PublicNetwork.Destinations, *destination)

	options, err := DataToOption(data)
	assert.Empty(t, err)
	assert.Equal(t, 3, len(options), "should not have other option generated")
	assert.Equal(t, "172.16.0.0/12:172.16.0.1", options["public-network-gateway"][0], "not expected public-network-gateway option")
	assert.Equal(t, "172.16.0.2/12", options["public-network-ip"][0], "not expected public-network-ip option")
}

func TestConvertClientNetwork(t *testing.T) {
	gwIP := net.ParseIP("172.16.0.1")
	_, destination, _ := net.ParseCIDR("172.16.0.0/12")
	ip, ipNet, _ := net.ParseCIDR("172.16.0.2/12")

	data := data.NewData()
	data.ClientNetwork.Name = "public"
	data.ClientNetwork.Gateway.IP = gwIP
	data.ClientNetwork.Gateway.Mask = ipNet.Mask
	data.ClientNetwork.IP.IP = ip
	data.ClientNetwork.IP.Mask = ipNet.Mask
	data.ClientNetwork.Destinations = append(data.ClientNetwork.Destinations, *destination)

	options, err := DataToOption(data)
	assert.Empty(t, err)
	assert.Equal(t, 3, len(options), "should not have other option generated")
	assert.Equal(t, "172.16.0.0/12:172.16.0.1", options["client-network-gateway"][0], "not expected client-network-gateway option")
	assert.Equal(t, "172.16.0.2/12", options["client-network-ip"][0], "not expected client-network-ip option")
}

func TestConvertMgmtNetwork(t *testing.T) {
	gwIP := net.ParseIP("172.16.0.1")
	_, destination, _ := net.ParseCIDR("172.16.0.0/12")
	ip, ipNet, _ := net.ParseCIDR("172.16.0.2/12")

	data := data.NewData()
	data.ManagementNetwork.Name = "public"
	data.ManagementNetwork.Gateway.IP = gwIP
	data.ManagementNetwork.Gateway.Mask = ipNet.Mask
	data.ManagementNetwork.IP.IP = ip
	data.ManagementNetwork.IP.Mask = ipNet.Mask
	data.ManagementNetwork.Destinations = append(data.ManagementNetwork.Destinations, *destination)

	options, err := DataToOption(data)
	assert.Empty(t, err)
	assert.Equal(t, 3, len(options), "should not have other option generated")
	assert.Equal(t, "172.16.0.0/12:172.16.0.1", options["management-network-gateway"][0], "not expected management-network-gateway option")
	assert.Equal(t, "172.16.0.2/12", options["management-network-ip"][0], "not expected management-network-ip option")
}

func testConvertContainerNetworks(t *testing.T) {
	gwIP := net.ParseIP("172.16.0.1")
	_, destination, _ := net.ParseCIDR("172.16.0.0/12")
	ip, ipNet, _ := net.ParseCIDR("172.16.0.2/12")

	data := data.NewData()
	data.ManagementNetwork.Name = "public"
	data.ManagementNetwork.Gateway.IP = gwIP
	data.ManagementNetwork.Gateway.Mask = ipNet.Mask
	data.ManagementNetwork.IP.IP = ip
	data.ManagementNetwork.IP.Mask = ipNet.Mask
	data.ManagementNetwork.Destinations = append(data.ManagementNetwork.Destinations, *destination)

	options, err := DataToOption(data)
	assert.Empty(t, err)
	assert.Equal(t, 3, len(options), "should not have other option generated")
	assert.Equal(t, "172.16.0.0/12:172.16.0.1", options["management-network-gateway"][0], "not expected management-network-gateway option")
	assert.Equal(t, "172.16.0.2/12", options["management-network-ip"][0], "not expected management-network-ip option")
}

func TestConvertSharesInfo(t *testing.T) {
	data := data.NewData()
	cLimit, cReserve := 29300, 1024
	data.NumCPUs = 2
	data.VCHCPULimitsMHz = &cLimit
	data.VCHCPUReservationsMHz = &cReserve
	data.VCHCPUShares = &types.SharesInfo{
		Shares: 6000,
		Level:  types.SharesLevelCustom,
	}
	mLimit, mReserve := 13144, 1024
	data.MemoryMB = 4096
	data.VCHMemoryLimitsMB = &mLimit
	data.VCHMemoryReservationsMB = &mReserve
	data.VCHMemoryShares = &types.SharesInfo{
		Shares: 163840,
		Level:  types.SharesLevelNormal,
	}

	options, err := DataToOption(data)
	assert.Empty(t, err)
	assert.Equal(t, 7, len(options), "should not have other option generated")
	assert.Equal(t, "2", options["endpoint-cpu"][0], "not expected endpoint-cpu option")
	assert.Equal(t, "4096", options["endpoint-memory"][0], "not expected endpoint-memory option")
	assert.Equal(t, "13144", options["memory"][0], "not expected memory option")
	assert.Equal(t, "1024", options["memory-reservation"][0], "not expected memory-reservation option")
	assert.Equal(t, "29300", options["cpu"][0], "not expected cpu option")
	assert.Equal(t, "1024", options["cpu-reservation"][0], "not expected cpu-reservation option")
	assert.Equal(t, "6000", options["cpu-shares"][0], "not expected cpu-shares option")
}

func TestGuestInfo(t *testing.T) {
	kv := map[string]string{
		"guestinfo.vice..init.sessions|docker-personality.detail.createtime":           "0",
		"guestinfo.vice./init/sessions|docker-personality/Group":                       "",
		"init/networks|client/Common/notes":                                            "",
		"guestinfo.vice./network/container_networks|vnet/gateway/IP":                   "AAAAAAAAAAAAAP//CgoKAQ==",
		"guestinfo.vice./init/sessions|port-layer/tty":                                 "false",
		"guestinfo.vice..init.networks|bridge.network.type":                            "bridge",
		"vmotion.checkpointSVGAPrimarySize":                                            "4194304",
		"guestinfo.vice./storage/image_stores|0/Host":                                  "vsanDatastore",
		"guestinfo.vice./init/sessions|vicadmin/stopSignal":                            "",
		"guestinfo.vice./network/container_networks|vnet/Common/id":                    "DistributedVirtualPortgroup:dvportgroup-357",
		"guestinfo.vice..init.networks|client.network.assigned.dns|1":                  "CqYBAQ==",
		"guestinfo.vice..init.sessions|vicadmin.status":                                "0",
		"guestinfo.vice./init/networks|bridge/Common/id":                               "224",
		"guestinfo.vice./init/networks|bridge/network/Common/id":                       "DistributedVirtualPortgroup:dvportgroup-55",
		"guestinfo.vice./init/diagnostics/debug":                                       "3",
		"guestinfo.vice./storage/VolumeLocations|default/ForceQuery":                   "false",
		"guestinfo.vice./init/sessions|docker-personality/active":                      "true",
		"guestinfo.vice./container/ComputeResources":                                   "0",
		"guestinfo.vice..init.networks|client.network.assigned.gateway.Mask":           "///gAA==",
		"guestinfo.vice./storage/VolumeLocations|vol1/Opaque":                          "",
		"guestinfo.vice..init.networks|management.assigned.IP":                         "CsCoOQ==",
		"guestinfo.vice./init/sessions|port-layer/common/ExecutionEnvironment":         "",
		"guestinfo.vice./init/sessions|docker-personality/stopSignal":                  "",
		"init/networks|management/Common/notes":                                        "",
		"guestinfo.vice./storage/VolumeLocations|vol1/RawPath":                         "",
		"network/container_networks|bridge/Common/notes":                               "",
		"guestinfo.vice..network.container_networks|vmnet.type":                        "external",
		"guestinfo.vice..init.sessions|docker-personality.detail.stoptime":             "0",
		"guestinfo.vice./init/sessions|vicadmin/cmd/Dir":                               "/home/vicadmin",
		"guestinfo.vice./connect/keepalive":                                            "0",
		"guestinfo.vice..init.networks|management.network.type":                        "",
		"guestinfo.vice./init/sessions|port-layer/common/name":                         "port-layer",
		"guestinfo.vice..init.networks|public.network.assigned.dns|1":                  "CqYBAQ==",
		"guestinfo.vice./init/sessions|docker-personality/common/ExecutionEnvironment": "",
		"guestinfo.vice./init/sessions|vicadmin/openstdin":                             "false",
		"guestinfo.vice./connect/target_thumbprint":                                    "9F:F0:DF:BA:7F:E2:89:F0:98:E4:A6:D1:58:24:68:74:8A:9B:25:6F",
		"guestinfo.vice..init.networks|public.network.type":                            "",
		"guestinfo.vice./init/sessions|docker-personality/cmd/Path":                    "/sbin/docker-engine-server",
		"guestinfo.vice..init.createtime":                                              "0",
		"guestinfo.vice./init/sessions|port-layer/active":                              "true",
		"guestinfo.vice./network/container_networks|vnet/pools|0/last":                 "CgoK/w==",
		"init/sessions|docker-personality/common/notes":                                "",
		"guestinfo.vice..init.diagnostics.resurrections":                               "0",
		"guestinfo.vice..init.networks|public.network.assigned.gateway.Mask":           "///gAA==",
		"guestinfo.vice./init/sessions|port-layer/cmd/Dir":                             "",
		"guestinfo.vice./network/container_networks|vnet/pools":                        "0",
		"guestinfo.vice./init/sessions|vicadmin/tty":                                   "false",
		"guestinfo.vice./storage/image_stores|0/RawPath":                               "",
		"guestinfo.vice./init/sessions|port-layer/attach":                              "false",
		"guestinfo.vice./container/ComputeResources|0/Type":                            "VirtualApp",
		"guestinfo.vice./storage/image_stores|0/Scheme":                                "ds",
		"guestinfo.vice..init.networks|public.assigned.Mask":                           "///gAA==",
		"guestinfo.vice./init/networks|bridge/Common/ExecutionEnvironment":             "",
		"guestinfo.vice./storage/image_stores|0/Path":                                  "91512359-242e-5a30-ba4a-0200360a2e79",
		"numa.autosize.vcpu.maxPerVirtualNode":                                         "1",
		"guestinfo.vice./init/version/BuildDate":                                       "2017/05/06@14:16:54",
		"guestinfo.vice./init/sessions|docker-personality/User":                        "",
		"guestinfo.vice..init.networks|client.assigned.Mask":                           "///gAA==",
		"guestinfo.vice./init/networks|management/network/Common/id":                   "DistributedVirtualPortgroup:dvportgroup-56",
		"guestinfo.vice./connect/target":                                               "https://10.192.171.116",
		"guestinfo.vice./storage/image_stores|0/RawQuery":                              "",
		"guestinfo.vice./init/sessions|docker-personality/runblock":                    "false",
		"guestinfo.vice..init.networks|management.network.assigned.gateway.IP":         "CsC//Q==",
		"init/networks|management/network/Common/notes":                                "",
		"guestinfo.vice./init/version/GitCommit":                                       "65a1889",
		"guestinfo.vice./init/networks|public/Common/name":                             "public",
		"guestinfo.vice./init/version/Version":                                         "v1.1.0-rc3",
		"guestinfo.vice./storage/VolumeLocations|vol1/Scheme":                          "ds",
		"answer.msg.serial.file.open":                                                  "Append",
		"guestinfo.vice./storage/VolumeLocations|default/Path":                         "[vsanDatastore] volumes/default",
		"guestinfo.vice..init.networks|management.network.assigned.dns":                "1",
		"guestinfo.vice./init/networks|public/network/Common/id":                       "DistributedVirtualPortgroup:dvportgroup-56",
		"guestinfo.vice./storage/image_stores|0/ForceQuery":                            "false",
		"guestinfo.vice./init/imageid":                                                 "",
		"guestinfo.vice./init/sessions|docker-personality/cmd/Env":                     "3",
		"guestinfo.vice./init/sessions|docker-personality/common/id":                   "docker-personality",
		"guestinfo.vice./storage/VolumeLocations|vol1/Fragment":                        "",
		"guestinfo.vice./init/sessions|vicadmin/cmd/Env":                               "3",
		"guestinfo.vice./init/sessions|vicadmin/common/ExecutionEnvironment":           "",
		"guestinfo.vice./init/sessions|vicadmin/User":                                  "vicadmin",
		"guestinfo.vice..init.networks|public.network.assigned.dns":                    "1",
		"network/container_networks|vnet/Common/notes":                                 "",
		"guestinfo.vice./init/sessions|vicadmin/common/id":                             "vicadmin",
		"guestinfo.vice..init.sessions|docker-personality.diagnostics.resurrections":   "0",
		"guestinfo.vice./init/sessions|vicadmin/cmd/Args~":                             "/sbin/vicadmin|--dc=vcqaDC|--pool=|--cluster=/vcqaDC/host/cls",
		"guestinfo.vice./init/networks|public/static":                                  "false",
		"guestinfo.vice./init/networks|client/network/default":                         "false",
		"init/networks|public/network/Common/notes":                                    "",
		"guestinfo.vice./init/sessions|docker-personality/restart":                     "true",
		"guestinfo.vice./storage/VolumeLocations|vol1/Host":                            "vsanDatastore",
		"guestinfo.vice./init/networks|bridge/network/Common/name":                     "bridge",
		"guestinfo.vice..init.sessions|vicadmin.detail.stoptime":                       "0",
		"guestinfo.vice./container/ComputeResources|0/Value":                           "resgroup-v364",
		"guestinfo.vice./network/container_networks|vmnet/Common/name":                 "vmnet",
		"guestinfo.vice./network/container_networks":                                   "bridge|vmnet|vnet",
		"guestinfo.vice./init/sessions|docker-personality/cmd/Args":                    "2",
		"guestinfo.vice./init/networks|bridge/ip/IP":                                   "AAAAAAAAAAAAAP//AAAAAA==",
		"guestinfo.vice./init/networks|bridge/network/Common/ExecutionEnvironment":     "",
		"guestinfo.vice./storage/VolumeLocations|default/RawQuery":                     "",
		"guestinfo.vice..init.networks|public.assigned.IP":                             "CsCoOQ==",
		"guestinfo.vice./init/networks|bridge/Common/name":                             "bridge",
		"guestinfo.vice./init/sessions|port-layer/openstdin":                           "false",
		"guestinfo.vice./create_bridge_network":                                        "false",
		"guestinfo.vice./init/networks|management/Common/id":                           "192",
		"guestinfo.vice..init.networks|management.network.assigned.dns|0":              "CqLMAQ==",
		"guestinfo.vice..init.networks|public.network.assigned.dns|0":                  "CqLMAQ==",
		"guestinfo.vice..init.sessions|docker-personality.started":                     "true",
		"init/networks|public/Common/notes":                                            "",
		"guestinfo.vice..init.networks|management.network.assigned.dns|1":              "CqYBAQ==",
		"guestinfo.vice./init/sessions|vicadmin/active":                                "true",
		"guestinfo.vice./init/networks":                                                "bridge|client|management|public",
		"guestinfo.vice./init/networks|client/Common/id":                               "192",
		"guestinfo.vice./network/container_networks|vnet/Common/ExecutionEnvironment":  "",
		"init/networks|client/network/Common/notes":                                    "",
		"guestinfo.vice..init.sessions|port-layer.detail.stoptime":                     "0",
		"guestinfo.vice./init/sessions|docker-personality/cmd/Args~":                   "/sbin/docker-engine-server|-port=2376|-port-layer-port=2377",
		"guestinfo.vice..init.sessions|vicadmin.runblock":                              "false",
		"guestinfo.vice./init/sessions|port-layer/Group":                               "",
		"guestinfo.vice./init/version/PluginVersion":                                   "6",
		"guestinfo.vice./storage/scratch_size":                                         "8000000",
		"guestinfo.vice./init/networks|client/Common/ExecutionEnvironment":             "",
		"init/networks|bridge/network/Common/notes":                                    "",
		"guestinfo.vice./storage/VolumeLocations|default/Scheme":                       "ds",
		"guestinfo.vice..init.sessions|vicadmin.diagnostics.resurrections":             "0",
		"guestinfo.vice./storage/VolumeLocations|vol1/ForceQuery":                      "false",
		"guestinfo.vice./network/container_networks|vmnet/default":                     "false",
		"guestinfo.vice..init.sessions|vicadmin.detail.createtime":                     "0",
		"guestinfo.vice./storage/VolumeLocations":                                      "default|vol1",
		"guestinfo.vice./storage/VolumeLocations|vol1/Path":                            "[vsanDatastore] volumes/vol1",
		"guestinfo.vice./init/sessions|vicadmin/cmd/Args":                              "3",
		"guestinfo.vice..init.sessions|docker-personality.runblock":                    "false",
		"guestinfo.vice./init/version/BuildNumber":                                     "10373",
		"guestinfo.vice./network/container_networks|vmnet/Common/id":                   "DistributedVirtualPortgroup:dvportgroup-358",
		"guestinfo.vice./init/networks|client/network/Common/id":                       "DistributedVirtualPortgroup:dvportgroup-56",
		"guestinfo.vice..init.sessions|docker-personality.status":                      "0",
		"guestinfo.vice..init.sessions|port-layer.started":                             "true",
		"guestinfo.vice./init/sessions|port-layer/runblock":                            "false",
		"guestinfo.vice./init/sessions|port-layer/diagnostics/debug":                   "0",
		"init/common/name":                                                             "test1",
		"guestinfo.vice..init.sessions|docker-personality.detail.starttime":            "0",
		"guestinfo.vice./storage/VolumeLocations|vol1/RawQuery":                        "",
		"init/common/notes":                                                            "",
		"guestinfo.vice./network/container_networks|bridge/Common/ExecutionEnvironment": "",
		"guestinfo.vice..init.networks|client.network.assigned.gateway.IP":              "CsC//Q==",
		"guestinfo.vice..init.sessions|vicadmin.detail.starttime":                       "0",
		"guestinfo.vice./init/networks|client/Common/name":                              "client",
		"guestinfo.vice./init/networks|bridge/static":                                   "true",
		"network/container_networks|vmnet/Common/notes":                                 "",
		"guestinfo.vice./init/repo":                                                     "",
		"guestinfo.vice./storage/image_stores":                                          "0",
		"guestinfo.vice./init/sessions|vicadmin/runblock":                               "false",
		"guestinfo.vice./network/container_networks|vnet/gateway/Mask":                  "////AA==",
		"guestinfo.vice..init.networks|client.network.assigned.dns|0":                   "CqLMAQ==",
		"guestinfo.vice./init/networks|public/Common/id":                                "192",
		"guestinfo.vice./init/networks|public/Common/ExecutionEnvironment":              "",
		"guestinfo.vice./init/networks|public/network/default":                          "true",
		"guestinfo.vice./storage/VolumeLocations|default/Fragment":                      "",
		"guestinfo.vice./init/asymrouting":                                              "false",
		"guestinfo.vice..init.sessions|vicadmin.started":                                "true",
		"guestinfo.vice./init/sessions|port-layer/User":                                 "",
		"guestinfo.vice./init/networks|management/Common/name":                          "",
		"guestinfo.vice./init/sessions|docker-personality/cmd/Dir":                      "",
		"guestinfo.vice./network/container_networks|vnet/Common/name":                   "vnet",
		"guestinfo.vice./network/container_networks|bridge/Common/id":                   "DistributedVirtualPortgroup:dvportgroup-55",
		"init/sessions|port-layer/common/notes":                                         "",
		"guestinfo.vice./network/bridge-ip-range/IP":                                    "rBAAAA==",
		"guestinfo.vice..init.networks|management.network.assigned.gateway.Mask":        "///gAA==",
		"guestinfo.vice./init/sessions|vicadmin/common/name":                            "vicadmin",
		"guestinfo.vice./init/networks|management/network/Common/ExecutionEnvironment":  "",
		"guestinfo.vice./init/networks|management/static":                               "false",
		"guestinfo.vice./init/common/id":                                                "VirtualMachine:vm-365",
		"guestinfo.vice./network/bridge-ip-range/Mask":                                  "//AAAA==",
		"guestinfo.vice./vic_machine_create_options":                                    "13",
		"guestinfo.vice..init.sessions|port-layer.status":                               "0",
		"guestinfo.vice./storage/VolumeLocations|default/Opaque":                        "",
		"guestinfo.vice./init/networks|client/static":                                   "false",
		"guestinfo.vice./init/networks|management/network/Common/name":                  "management",
		"guestinfo.vice./init/sessions|docker-personality/openstdin":                    "false",
		"guestinfo.vice./init/sessions|port-layer/cmd/Args":                             "2",
		"guestinfo.vice..init.networks|client.assigned.IP":                              "CsCoOQ==",
		"guestinfo.vice./init/networks|public/network/Common/ExecutionEnvironment":      "",
		"guestinfo.vice./init/version/State":                                            "",
		"guestinfo.vice./network/container_networks|vmnet/Common/ExecutionEnvironment":  "",
		"guestinfo.vice./init/sessions|docker-personality/attach":                       "false",
		"guestinfo.vice./storage/image_stores|0/Opaque":                                 "",
		"guestinfo.vice..init.networks|client.network.assigned.dns":                     "1",
		"guestinfo.vice./init/sessions|port-layer/cmd/Env~":                             "VC_URL=https://10.192.171.116|DC_PATH=vcqaDC|CS_PATH=/vcqaDC/host/cls|POOL_PATH=|DS_PATH=vsanDatastore",
		"guestinfo.vice./network/container_networks|bridge/Common/name":                 "bridge",
		"guestinfo.vice./storage/VolumeLocations|default/Host":                          "vsanDatastore",
		"guestinfo.vice./init/sessions|port-layer/cmd/Env":                              "4",
		"guestinfo.vice..init.sessions|port-layer.detail.starttime":                     "0",
		"guestinfo.vice./init/networks|management/Common/ExecutionEnvironment":          "",
		"guestinfo.vice./init/sessions":                                                 "docker-personality|port-layer|vicadmin",
		"guestinfo.vice..init.sessions|port-layer.detail.createtime":                    "0",
		"guestinfo.vice./storage/image_stores|0/Fragment":                               "",
		"guestinfo.vice..init.networks|management.assigned.Mask":                        "///gAA==",
		"guestinfo.vice./network/container_networks|bridge/default":                     "false",
		"guestinfo.vice./container/ContainerNameConvention":                             "",
		"guestinfo.vice..init.networks|client.network.type":                             "",
		"guestinfo.vice./init/sessions|vicadmin/restart":                                "true",
		"guestinfo.vice./init/networks|public/network/Common/name":                      "public",
		"guestinfo.vice..init.sessions|port-layer.runblock":                             "false",
		"guestinfo.vice./init/sessions|port-layer/common/id":                            "port-layer",
		"guestinfo.vice./init/networks|client/network/Common/name":                      "client",
		"guestinfo.vice./network/container_networks|vnet/pools|0/first":                 "CgoKAA==",
		"guestinfo.vice..init.networks|public.network.assigned.gateway.IP":              "CsC//Q==",
		"guestinfo.vice./init/layerid":                                                  "",
		"guestinfo.vice./init/sessions|docker-personality/common/name":                  "docker-personality",
		"guestinfo.vice./init/sessions|docker-personality/diagnostics/debug":            "0",
		"guestinfo.vice./container/bootstrap_image_path":                                "[vsanDatastore] 91512359-242e-5a30-ba4a-0200360a2e79/V1.1.0-RC3-10373-65A1889-bootstrap.iso",
		"guestinfo.vice..network.container_networks|bridge.type":                        "bridge",
		"guestinfo.vice..network.container_networks|vnet.type":                          "external", "migrate.hostLogState": "none",
		"init/sessions|vicadmin/common/notes":                                "",
		"guestinfo.vice./init/sessions|vicadmin/diagnostics/debug":           "0",
		"guestinfo.vice./init/sessions|vicadmin/attach":                      "false",
		"guestinfo.vice./init/sessions|vicadmin/Group":                       "vicadmin",
		"guestinfo.vice./storage/VolumeLocations|default/RawPath":            "",
		"guestinfo.vice./init/networks|bridge/network/default":               "false",
		"guestinfo.vice./init/sessions|port-layer/stopSignal":                "",
		"guestinfo.vice./connect/username":                                   "administrator@vsphere.local",
		"guestinfo.vice..init.sessions|port-layer.diagnostics.resurrections": "0",
		"guestinfo.vice./init/common/ExecutionEnvironment":                   "",
		"guestinfo.vice./network/bridge_network":                             "bridge",
		"guestinfo.vice./registry/whitelist_registries":                      "1",
		"guestinfo.vice./registry/whitelist_registries~":                     "harbor.com:2345|insecure:2345",
		"guestinfo.vice./registry/insecure_registries~":                      "insecure:2345",
		"guestinfo.vice./registry/insecure_registries":                       "0",
		"guestinfo.vice./init/sessions|vicadmin/cmd/Env~":                    "PATH=/sbin:/bin|GOTRACEBACK=all|HTTP_PROXY=http://proxy.vmware.com:2318|HTTPS_PROXY=https://proxy.vmware.com:2318",
		"guestinfo.vice./init/sessions|docker-personality/cmd/Env~":          "PATH=/sbin|GOTRACEBACK=all|HTTP_PROXY=http://proxy.vmware.com:2318|HTTPS_PROXY=https://proxy.vmware.com:2318",
	}

	commands := map[string][]string{
		"target":      {"https://10.192.171.116"},
		"thumbprint":  {"9F:F0:DF:BA:7F:E2:89:F0:98:E4:A6:D1:58:24:68:74:8A:9B:25:6F"},
		"image-store": {"vsanDatastore"},
		"volume-store": {
			"vsanDatastore/volumes/default:default",
			"vsanDatastore/volumes/vol1:vol1",
		},
		"bridge-network": {"bridge"},
		"public-network": {"external"},
		"container-network": {"management:vnet",
			"vm-network:vmnet",
		},
		"container-network-gateway":  {"management:10.10.10.1/24"},
		"container-network-ip-range": {"management:10.10.10.0/24"},

		"debug":              {"3"},
		"insecure-registry":  {"insecure:2345"},
		"whitelist-registry": {"harbor.com:2345", "insecure:2345"},
		"http-proxy":         {"http://proxy.vmware.com:2318"},
		"https-proxy":        {"https://proxy.vmware.com:2318"},
	}
	conf := &config.VirtualContainerHostConfigSpec{}
	extraconfig.DecodeWithPrefix(extraconfig.MapSource(kv), conf, "")

	ctx := context.Background()
	d, err := validate.NewDataFromConfig(ctx, &testFinder{}, conf)
	if err != nil {
		t.Errorf(err.Error())
	}
	dest, err := DataToOption(d)
	if err != nil {
		t.Errorf(err.Error())
	}
	for k, v := range dest {
		if strings.Contains(k, "store") {
			for i := range v {
				v[i] = strings.TrimLeft(v[i], "ds://")
			}
		}
		c := commands[k]
		testArrays(t, c, v, k)
	}
	for k, v := range commands {
		if strings.Contains(k, "store") {
			for i := range v {
				v[i] = strings.TrimLeft(v[i], "ds://")
			}
		}
		c := dest[k]
		testArrays(t, v, c, k)
	}

}

func testArrays(t *testing.T, l, r []string, key string) {
	for i := range l {
		found := false
		for j := range r {
			if l[i] == r[j] {
				found = true
				break
			}
		}
		assert.True(t, found, fmt.Sprintf("option value %s mismatch: value %s in array %s is not found in array %s", key, l[i], l, r))
	}
}
