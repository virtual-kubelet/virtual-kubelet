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
	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/vic/pkg/trace"
)

// NewVirtualIDEController returns a VirtualIDEController spec with key.
func NewVirtualIDEController(key int32) *types.VirtualIDEController {
	defer trace.End(trace.Begin(""))

	return &types.VirtualIDEController{
		VirtualController: types.VirtualController{
			VirtualDevice: types.VirtualDevice{
				Key: key,
			},
		},
	}
}

// AddVirtualIDEController adds a virtual IDE controller.
func (s *VirtualMachineConfigSpec) AddVirtualIDEController(device *types.VirtualIDEController) *VirtualMachineConfigSpec {
	defer trace.End(trace.Begin(s.ID()))

	return s.AddVirtualDevice(device)

}

// RemoveVirtualIDEController removes a virtual IDE controller.
func (s *VirtualMachineConfigSpec) RemoveVirtualIDEController(device *types.VirtualIDEController) *VirtualMachineConfigSpec {
	defer trace.End(trace.Begin(s.ID()))

	return s.RemoveVirtualDevice(device)

}

// NewVirtualCdrom returns a virtual CDROM device.
func NewVirtualCdrom(device *types.VirtualIDEController) *types.VirtualCdrom {
	defer trace.End(trace.Begin(""))

	return &types.VirtualCdrom{
		VirtualDevice: types.VirtualDevice{
			ControllerKey: device.Key,
			UnitNumber:    new(int32),
		},
	}
}

// AddVirtualCdrom adds a CD-ROM device in a virtual machine.
func (s *VirtualMachineConfigSpec) AddVirtualCdrom(device *types.VirtualCdrom) *VirtualMachineConfigSpec {
	defer trace.End(trace.Begin(s.ID()))

	device.GetVirtualDevice().Key = s.generateNextKey()

	device.GetVirtualDevice().Backing = &types.VirtualCdromIsoBackingInfo{
		VirtualDeviceFileBackingInfo: types.VirtualDeviceFileBackingInfo{
			FileName: s.BootMediaPath(),
		},
	}

	return s.AddVirtualDevice(device)
}

// RemoveVirtualCdrom adds a CD-ROM device in a virtual machine.
func (s *VirtualMachineConfigSpec) RemoveVirtualCdrom(device *types.VirtualCdrom) *VirtualMachineConfigSpec {
	defer trace.End(trace.Begin(s.ID()))

	return s.RemoveVirtualDevice(device)
}

// NewVirtualFloppy adds a floppy device in a virtual machine.
func NewVirtualFloppy(device *types.VirtualIDEController) *types.VirtualFloppy {
	defer trace.End(trace.Begin(""))

	return &types.VirtualFloppy{
		VirtualDevice: types.VirtualDevice{
			ControllerKey: device.Key,
			UnitNumber:    new(int32),
		},
	}
}

// AddVirtualFloppy adds a floppy device in a virtual machine.
func (s *VirtualMachineConfigSpec) AddVirtualFloppy(device *types.VirtualFloppy) *VirtualMachineConfigSpec {
	defer trace.End(trace.Begin(s.ID()))

	device.GetVirtualDevice().Key = s.generateNextKey()

	device.GetVirtualDevice().Backing = &types.VirtualFloppyImageBackingInfo{
		VirtualDeviceFileBackingInfo: types.VirtualDeviceFileBackingInfo{
			FileName: s.BootMediaPath(),
		},
	}

	return s.AddVirtualDevice(device)
}

// RemoveVirtualFloppyDevice removes a floppy device from the virtual machine.
func (s *VirtualMachineConfigSpec) RemoveVirtualFloppyDevice(device *types.VirtualFloppy) *VirtualMachineConfigSpec {
	defer trace.End(trace.Begin(s.ID()))

	return s.RemoveVirtualDevice(device)
}
