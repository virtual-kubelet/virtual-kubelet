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

	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/vic/pkg/trace"
)

// NewVirtualSCSIController returns a VirtualSCSIController with bus number and key.
func NewVirtualSCSIController(bus int32, key int32) types.VirtualSCSIController {
	defer trace.End(trace.Begin(fmt.Sprintf("%d - %d", bus, key)))

	return types.VirtualSCSIController{
		SharedBus: types.VirtualSCSISharingNoSharing,
		VirtualController: types.VirtualController{
			BusNumber: bus,
			VirtualDevice: types.VirtualDevice{
				Key: key,
			},
		},
	}
}

// NewParaVirtualSCSIController returns ParaVirtualSCSIController spec.
func NewParaVirtualSCSIController(controller types.VirtualSCSIController) *types.ParaVirtualSCSIController {
	defer trace.End(trace.Begin(""))

	return &types.ParaVirtualSCSIController{
		VirtualSCSIController: controller,
	}
}

// NewVirtualBusLogicController returns VirtualBusLogicController spec.
func NewVirtualBusLogicController(controller types.VirtualSCSIController) *types.VirtualBusLogicController {
	defer trace.End(trace.Begin(""))

	return &types.VirtualBusLogicController{
		VirtualSCSIController: controller,
	}
}

// NewVirtualLsiLogicController returns a VirtualLsiLogicController spec
func NewVirtualLsiLogicController(controller types.VirtualSCSIController) *types.VirtualLsiLogicController {
	defer trace.End(trace.Begin(""))

	return &types.VirtualLsiLogicController{
		VirtualSCSIController: controller,
	}
}

// NewVirtualLsiLogicSASController returns VirtualLsiLogicSASController spec.
func NewVirtualLsiLogicSASController(controller types.VirtualSCSIController) *types.VirtualLsiLogicSASController {
	defer trace.End(trace.Begin(""))

	return &types.VirtualLsiLogicSASController{
		VirtualSCSIController: controller,
	}
}

// AddParaVirtualSCSIController adds a paravirtualized SCSI controller.
func (s *VirtualMachineConfigSpec) AddParaVirtualSCSIController(device *types.ParaVirtualSCSIController) *VirtualMachineConfigSpec {
	defer trace.End(trace.Begin(s.ID()))

	return s.AddVirtualDevice(device)
}

// RemoveParaVirtualSCSIController removes a paravirtualized SCSI controller.
func (s *VirtualMachineConfigSpec) RemoveParaVirtualSCSIController(device *types.ParaVirtualSCSIController) *VirtualMachineConfigSpec {
	defer trace.End(trace.Begin(s.ID()))

	return s.RemoveVirtualDevice(device)

}

// AddVirtualBusLogicController adds a BusLogic SCSI controller.
func (s *VirtualMachineConfigSpec) AddVirtualBusLogicController(device *types.VirtualBusLogicController) *VirtualMachineConfigSpec {
	defer trace.End(trace.Begin(s.ID()))

	return s.AddVirtualDevice(device)

}

// RemoveVirtualBusLogicController removes a BusLogic SCSI controller.
func (s *VirtualMachineConfigSpec) RemoveVirtualBusLogicController(device *types.VirtualBusLogicController) *VirtualMachineConfigSpec {
	defer trace.End(trace.Begin(s.ID()))

	return s.RemoveVirtualDevice(device)

}

// AddVirtualLsiLogicController adds a LSI Logic SCSI controller.
func (s *VirtualMachineConfigSpec) AddVirtualLsiLogicController(device *types.VirtualLsiLogicController) *VirtualMachineConfigSpec {
	defer trace.End(trace.Begin(s.ID()))

	return s.AddVirtualDevice(device)

}

// RemoveVirtualLsiLogicController removes a LSI Logic SCSI controller.
func (s *VirtualMachineConfigSpec) RemoveVirtualLsiLogicController(device *types.VirtualLsiLogicController) *VirtualMachineConfigSpec {
	defer trace.End(trace.Begin(s.ID()))

	return s.RemoveVirtualDevice(device)

}

// AddVirtualLsiLogicSASController add a LSI Logic SAS SCSI controller.
func (s *VirtualMachineConfigSpec) AddVirtualLsiLogicSASController(device *types.VirtualLsiLogicSASController) *VirtualMachineConfigSpec {
	defer trace.End(trace.Begin(s.ID()))

	return s.AddVirtualDevice(device)

}

// RemoveVirtualLsiLogicSASController removes a LSI Logic SAS SCSI controller.
func (s *VirtualMachineConfigSpec) RemoveVirtualLsiLogicSASController(device *types.VirtualLsiLogicSASController) *VirtualMachineConfigSpec {
	defer trace.End(trace.Begin(s.ID()))

	return s.RemoveVirtualDevice(device)
}
