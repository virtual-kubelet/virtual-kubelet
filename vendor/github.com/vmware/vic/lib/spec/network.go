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

package spec

import (
	"fmt"

	"context"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/vic/pkg/trace"
)

// NewVirtualVmxnet3 returns VirtualVmxnet3 spec.
func NewVirtualVmxnet3() *types.VirtualVmxnet3 {
	defer trace.End(trace.Begin(""))

	return &types.VirtualVmxnet3{
		VirtualVmxnet: types.VirtualVmxnet{
			VirtualEthernetCard: types.VirtualEthernetCard{
				AddressType: string(types.VirtualEthernetCardMacTypeGenerated),
			},
		},
	}
}

// NewVirtualPCNet32 returns VirtualPCNet32 spec.
func NewVirtualPCNet32() *types.VirtualPCNet32 {
	defer trace.End(trace.Begin(""))

	return &types.VirtualPCNet32{
		VirtualEthernetCard: types.VirtualEthernetCard{
			AddressType: string(types.VirtualEthernetCardMacTypeGenerated),
		},
	}
}

// NewVirtualE1000 returns VirtualE1000 spec.
func NewVirtualE1000() *types.VirtualE1000 {
	defer trace.End(trace.Begin(""))

	return &types.VirtualE1000{
		VirtualEthernetCard: types.VirtualEthernetCard{
			AddressType: string(types.VirtualEthernetCardMacTypeGenerated),
		},
	}
}

func (s *VirtualMachineConfigSpec) addVirtualNIC(device types.BaseVirtualDevice) *VirtualMachineConfigSpec {
	device.GetVirtualDevice().Key = s.generateNextKey()

	return s.AddVirtualDevice(device)
}

// AddVirtualVmxnet3 adds a VirtualVmxnet3 device.
func (s *VirtualMachineConfigSpec) AddVirtualVmxnet3(device *types.VirtualVmxnet3) *VirtualMachineConfigSpec {
	defer trace.End(trace.Begin(s.ID()))

	return s.addVirtualNIC(device)
}

// AddVirtualPCNet32 adds a VirtualPCNet32 device.
func (s *VirtualMachineConfigSpec) AddVirtualPCNet32(device *types.VirtualPCNet32) *VirtualMachineConfigSpec {
	defer trace.End(trace.Begin(s.ID()))

	return s.addVirtualNIC(device)
}

// AddVirtualE1000 adds a VirtualE1000 device.
func (s *VirtualMachineConfigSpec) AddVirtualE1000(device *types.VirtualE1000) *VirtualMachineConfigSpec {
	defer trace.End(trace.Begin(s.ID()))

	return s.addVirtualNIC(device)
}

// RemoveVirtualVmxnet3 adds a VirtualVmxnet3 device.
func (s *VirtualMachineConfigSpec) RemoveVirtualVmxnet3(device *types.VirtualVmxnet3) *VirtualMachineConfigSpec {
	defer trace.End(trace.Begin(s.ID()))

	return s.RemoveVirtualDevice(device)
}

// RemoveVirtualPCNet32 adds a VirtualPCNet32 device.
func (s *VirtualMachineConfigSpec) RemoveVirtualPCNet32(device *types.VirtualPCNet32) *VirtualMachineConfigSpec {
	defer trace.End(trace.Begin(s.ID()))

	return s.RemoveVirtualDevice(device)
}

// RemoveVirtualE1000 adds a VirtualE1000 device.
func (s *VirtualMachineConfigSpec) RemoveVirtualE1000(device *types.VirtualE1000) *VirtualMachineConfigSpec {
	defer trace.End(trace.Begin(s.ID()))

	return s.RemoveVirtualDevice(device)
}

func (s *VirtualMachineConfigSpec) FindNICs(ctx context.Context, network object.NetworkReference) ([]types.BaseVirtualDeviceConfigSpec, error) {
	if network == nil {
		return nil, fmt.Errorf("no network provided")
	}

	backing, err := network.EthernetCardBackingInfo(ctx)
	if err != nil {
		return nil, err
	}

	var dcs []types.BaseVirtualDeviceConfigSpec
	for _, d := range s.DeviceChange {
		dev := d.GetVirtualDeviceConfigSpec().Device
		if _, ok := dev.(types.BaseVirtualEthernetCard); ok {
			var dl object.VirtualDeviceList
			dl = append(dl, dev)
			dl = dl.SelectByBackingInfo(backing)
			if len(dl) > 0 {
				dcs = append(dcs, d)
			}
		}
	}

	return dcs, nil
}
