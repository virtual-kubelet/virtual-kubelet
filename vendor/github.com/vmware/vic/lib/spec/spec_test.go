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
	"testing"
	"time"

	"context"

	"github.com/stretchr/testify/assert"

	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/vic/lib/constants"
	"github.com/vmware/vic/pkg/vsphere/session"
	"github.com/vmware/vic/pkg/vsphere/test/env"
)

func TestVirtualMachineConfigSpec(t *testing.T) {

	ctx := context.Background()

	sessionconfig := &session.Config{
		Service:        env.URL(t),
		Insecure:       true,
		Keepalive:      time.Duration(5) * time.Minute,
		DatacenterPath: "",
		DatastorePath:  "/ha-datacenter/datastore/*",
		HostPath:       "/ha-datacenter/host/*/*",
		PoolPath:       "/ha-datacenter/host/*/Resources",
	}

	s, err := session.NewSession(sessionconfig).Create(ctx)
	if err != nil {
		t.Logf("%+v", err.Error())
		if _, ok := err.(*find.MultipleFoundError); !ok {
			t.Errorf(err.Error())
		} else {
			t.SkipNow()
		}
	}
	defer s.Logout(ctx)

	specconfig := &VirtualMachineConfigSpecConfig{
		NumCPUs:       2,
		MemoryMB:      2048,
		VMForkEnabled: true,

		ID: "zombie_attack",

		BootMediaPath: s.Datastore.Path("brainz.iso"),
		VMPathName:    fmt.Sprintf("[%s]", s.Datastore.Name()),
	}
	// FIXME: find a better way to pass those
	var scsibus int32
	var scsikey int32 = 100
	var idekey int32 = 200

	root, _ := NewVirtualMachineConfigSpec(ctx, s, specconfig)
	scsi := NewVirtualSCSIController(scsibus, scsikey)

	pv := NewParaVirtualSCSIController(scsi)
	root.AddParaVirtualSCSIController(pv)

	bl := NewVirtualBusLogicController(scsi)
	root.AddVirtualBusLogicController(bl)

	ll := NewVirtualLsiLogicController(scsi)
	root.AddVirtualLsiLogicController(ll)

	ls := NewVirtualLsiLogicSASController(scsi)
	root.AddVirtualLsiLogicSASController(ls)
	///
	ide := NewVirtualIDEController(idekey)
	root.AddVirtualIDEController(ide)

	cdrom := NewVirtualCdrom(ide)
	root.AddVirtualCdrom(cdrom)

	floppy := NewVirtualFloppy(ide)
	root.AddVirtualFloppy(floppy)

	vmxnet3 := NewVirtualVmxnet3()
	root.AddVirtualVmxnet3(vmxnet3)

	pcnet32 := NewVirtualPCNet32()
	root.AddVirtualPCNet32(pcnet32)

	e1000 := NewVirtualE1000()
	root.AddVirtualE1000(e1000)

	for i := 0; i < len(root.DeviceChange); i++ {
		t.Logf("%+v", root.DeviceChange[i].GetVirtualDeviceConfigSpec().Device)
	}

}

func TestCollectSlotNumbers(t *testing.T) {
	s := &VirtualMachineConfigSpec{
		config: &VirtualMachineConfigSpecConfig{
			ID: "foo",
		},
		VirtualMachineConfigSpec: &types.VirtualMachineConfigSpec{},
	}

	slots := s.CollectSlotNumbers(nil)
	assert.Empty(t, slots)

	s.AddVirtualVmxnet3(NewVirtualVmxnet3())
	s.DeviceChange[0].GetVirtualDeviceConfigSpec().Device.GetVirtualDevice().SlotInfo = &types.VirtualDevicePciBusSlotInfo{PciSlotNumber: 32}
	slots = s.CollectSlotNumbers(nil)
	assert.EqualValues(t, map[int32]bool{32: true}, slots)

	// add a device without a slot number
	s.AddVirtualVmxnet3(NewVirtualVmxnet3())
	slots = s.CollectSlotNumbers(nil)
	assert.EqualValues(t, map[int32]bool{32: true}, slots)

	// add another device with slot number
	s.AddVirtualVmxnet3(NewVirtualVmxnet3())
	s.DeviceChange[len(s.DeviceChange)-1].GetVirtualDeviceConfigSpec().Device.GetVirtualDevice().SlotInfo = &types.VirtualDevicePciBusSlotInfo{PciSlotNumber: 33}
	slots = s.CollectSlotNumbers(slots)
	assert.EqualValues(t, map[int32]bool{32: true, 33: true}, slots)

}

func TestFindSlotNumber(t *testing.T) {
	allSlots := make(map[int32]bool)
	for s := constants.PCISlotNumberBegin; s != constants.PCISlotNumberEnd; s += constants.PCISlotNumberInc {
		allSlots[s] = true
	}

	// missing first slot
	missingFirstSlot := make(map[int32]bool)
	for s := constants.PCISlotNumberBegin + constants.PCISlotNumberInc; s != constants.PCISlotNumberEnd; s += constants.PCISlotNumberInc {
		missingFirstSlot[s] = true
	}

	// missing last slot
	missingLastSlot := make(map[int32]bool)
	for s := constants.PCISlotNumberBegin; s != constants.PCISlotNumberEnd-constants.PCISlotNumberInc; s += constants.PCISlotNumberInc {
		missingLastSlot[s] = true
	}

	// missing a slot in the middle
	var missingSlot int32
	missingMiddleSlot := make(map[int32]bool)
	for s := constants.PCISlotNumberBegin; s != constants.PCISlotNumberEnd-constants.PCISlotNumberInc; s += constants.PCISlotNumberInc {
		if constants.PCISlotNumberBegin+(2*constants.PCISlotNumberInc) == s {
			missingSlot = s
			continue
		}
		missingMiddleSlot[s] = true
	}

	var tests = []struct {
		slots map[int32]bool
		out   int32
	}{
		{make(map[int32]bool), constants.PCISlotNumberBegin},
		{allSlots, constants.NilSlot},
		{missingFirstSlot, constants.PCISlotNumberBegin},
		{missingLastSlot, constants.PCISlotNumberEnd - constants.PCISlotNumberInc},
		{missingMiddleSlot, missingSlot},
	}

	for _, te := range tests {
		if s := findSlotNumber(te.slots); s != te.out {
			t.Fatalf("findSlotNumber(%v) => %d, want %d", te.slots, s, te.out)
		}
	}
}
