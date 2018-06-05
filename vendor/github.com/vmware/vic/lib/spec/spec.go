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
	"context"
	"net/url"

	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/vic/lib/config/executor"
	"github.com/vmware/vic/lib/constants"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/vsphere/extraconfig"
	"github.com/vmware/vic/pkg/vsphere/extraconfig/vmomi"
	"github.com/vmware/vic/pkg/vsphere/session"
)

// VirtualMachineConfigSpecConfig holds the config values
type VirtualMachineConfigSpecConfig struct {
	// ID of the VM
	ID         string
	BiosUUID   string
	VMFullName string

	// ParentImageID of the VM
	ParentImageID string

	// Name of the VM
	Name string

	// Number of CPUs
	NumCPUs int32
	// Memory - in MB
	MemoryMB int64

	// VMFork enabled
	VMForkEnabled bool

	// datastore path of the media file we boot from
	BootMediaPath string

	// datastore path of the VM
	VMPathName string

	// Name of the image store
	ImageStoreName string

	// url path to image store
	ImageStorePath *url.URL

	// Temporary
	Metadata *executor.ExecutorConfig
}

// VirtualMachineConfigSpec type
type VirtualMachineConfigSpec struct {
	*session.Session

	*types.VirtualMachineConfigSpec

	config *VirtualMachineConfigSpecConfig

	// internal value to keep track of next ID
	key int32
}

// NewVirtualMachineConfigSpec returns a VirtualMachineConfigSpec
func NewVirtualMachineConfigSpec(ctx context.Context, session *session.Session, config *VirtualMachineConfigSpecConfig) (*VirtualMachineConfigSpec, error) {
	defer trace.End(trace.Begin(config.ID))

	s := &types.VirtualMachineConfigSpec{
		Name: config.VMFullName,
		Uuid: config.BiosUUID,
		Files: &types.VirtualMachineFileInfo{
			VmPathName: config.VMPathName,
		},
		NumCPUs:             config.NumCPUs,
		CpuHotAddEnabled:    &config.VMForkEnabled, // this disables vNUMA when true
		MemoryMB:            config.MemoryMB,
		MemoryHotAddEnabled: &config.VMForkEnabled,

		ExtraConfig: []types.BaseOptionValue{
			// lets us see the UUID for the containerfs disk (hidden from daemon)
			&types.OptionValue{Key: "disk.EnableUUID", Value: "true"},
			// needed to avoid the questions that occur when attaching multiple disks with the same uuid (bugzilla 1362918)
			&types.OptionValue{Key: "answer.msg.disk.duplicateUUID", Value: "Yes"},
			// needed to avoid the question that occur when opening a file backed serial port
			&types.OptionValue{Key: "answer.msg.serial.file.open", Value: "Append"},

			&types.OptionValue{Key: "sched.mem.lpage.maxSharedPages", Value: "256"},
			// seems to be needed to avoid children hanging shortly after fork
			&types.OptionValue{Key: "vmotion.checkpointSVGAPrimarySize", Value: "4194304"},

			// trying this out - if it works then we need to determine if we can rely on serial0 being the correct index.
			&types.OptionValue{Key: "serial0.hardwareFlowControl", Value: "TRUE"},

			// https://enatai-jira.eng.vmware.com/browse/BON-257
			// Hotadd memory above 3 GB not working
			&types.OptionValue{Key: "memory.noHotAddOver4GB", Value: "FALSE"},
			&types.OptionValue{Key: "memory.maxGrow", Value: "512"},

			// http://kb.vmware.com/selfservice/microsites/search.do?language=en_US&cmd=displayKC&externalId=2030189
			&types.OptionValue{Key: "tools.remindInstall", Value: "FALSE"},
			&types.OptionValue{Key: "tools.upgrade.policy", Value: "manual"},
		},
	}

	// encode the config as optionvalues
	cfg := map[string]string{}
	extraconfig.Encode(extraconfig.MapSink(cfg), config.Metadata)
	metaCfg := vmomi.OptionValueFromMap(cfg, true)

	// merge it with the sec
	s.ExtraConfig = append(s.ExtraConfig, metaCfg...)

	vmcs := &VirtualMachineConfigSpec{
		Session:                  session,
		VirtualMachineConfigSpec: s,
		config: config,
	}

	return vmcs, nil
}

// AddVirtualDevice appends an Add operation to the DeviceChange list
func (s *VirtualMachineConfigSpec) AddVirtualDevice(device types.BaseVirtualDevice) *VirtualMachineConfigSpec {
	s.DeviceChange = append(s.DeviceChange,
		&types.VirtualDeviceConfigSpec{
			Operation: types.VirtualDeviceConfigSpecOperationAdd,
			Device:    device,
		},
	)
	return s
}

// AddAndCreateVirtualDevice appends an Add operation to the DeviceChange list
func (s *VirtualMachineConfigSpec) AddAndCreateVirtualDevice(device types.BaseVirtualDevice) *VirtualMachineConfigSpec {
	s.DeviceChange = append(s.DeviceChange,
		&types.VirtualDeviceConfigSpec{
			Operation:     types.VirtualDeviceConfigSpecOperationAdd,
			FileOperation: types.VirtualDeviceConfigSpecFileOperationCreate,
			Device:        device,
		},
	)
	return s
}

// RemoveVirtualDevice appends a Remove operation to the DeviceChange list
func (s *VirtualMachineConfigSpec) RemoveVirtualDevice(device types.BaseVirtualDevice) *VirtualMachineConfigSpec {
	s.DeviceChange = append(s.DeviceChange,
		&types.VirtualDeviceConfigSpec{
			Operation: types.VirtualDeviceConfigSpecOperationRemove,
			Device:    device,
		},
	)
	return s
}

// RemoveAndDestroyVirtualDevice appends a Remove operation to the DeviceChange list
func (s *VirtualMachineConfigSpec) RemoveAndDestroyVirtualDevice(device types.BaseVirtualDevice) *VirtualMachineConfigSpec {
	s.DeviceChange = append(s.DeviceChange,
		&types.VirtualDeviceConfigSpec{
			Operation:     types.VirtualDeviceConfigSpecOperationRemove,
			FileOperation: types.VirtualDeviceConfigSpecFileOperationDestroy,

			Device: device,
		},
	)
	return s
}

// Name returns the name of the VM
func (s *VirtualMachineConfigSpec) Name() string {
	defer trace.End(trace.Begin(s.config.Name))

	return s.config.Name
}

// ID returns the ID of the VM
func (s *VirtualMachineConfigSpec) ID() string {
	defer trace.End(trace.Begin(s.config.ID))

	return s.config.ID
}

// ParentImageID returns the ID of the image that VM is based on
func (s *VirtualMachineConfigSpec) ParentImageID() string {
	defer trace.End(trace.Begin(s.config.ParentImageID))

	return s.config.ParentImageID
}

// BootMediaPath returns the image path
func (s *VirtualMachineConfigSpec) BootMediaPath() string {
	defer trace.End(trace.Begin(s.config.ID))

	return s.config.BootMediaPath
}

// VMPathName returns the VM folder path
func (s *VirtualMachineConfigSpec) VMPathName() string {
	defer trace.End(trace.Begin(s.config.ID))

	return s.config.VMPathName
}

// ImageStoreName returns the image store name
func (s *VirtualMachineConfigSpec) ImageStoreName() string {
	defer trace.End(trace.Begin(s.config.ID))

	return s.config.ImageStoreName
}

// ImageStorePath returns the image store url
func (s *VirtualMachineConfigSpec) ImageStorePath() *url.URL {
	defer trace.End(trace.Begin(s.config.ID))

	return s.config.ImageStorePath
}

func (s *VirtualMachineConfigSpec) generateNextKey() int32 {

	s.key -= 10
	return s.key
}

// Spec returns the base types.VirtualMachineConfigSpec object
func (s *VirtualMachineConfigSpec) Spec() *types.VirtualMachineConfigSpec {
	return s.VirtualMachineConfigSpec
}

// VirtualDeviceSlotNumber returns the PCI slot number of a device
func VirtualDeviceSlotNumber(d types.BaseVirtualDevice) int32 {
	s := d.GetVirtualDevice().SlotInfo
	if s == nil {
		return constants.NilSlot
	}

	if i, ok := s.(*types.VirtualDevicePciBusSlotInfo); ok {
		return i.PciSlotNumber
	}

	return constants.NilSlot
}

func findSlotNumber(slots map[int32]bool) int32 {
	// see https://kb.vmware.com/selfservice/microsites/search.do?language=en_US&cmd=displayKC&externalId=2047927
	slot := constants.PCISlotNumberBegin
	for _, ok := slots[slot]; ok && slot != constants.PCISlotNumberEnd; {
		slot += constants.PCISlotNumberInc
		_, ok = slots[slot]
	}

	if slot == constants.PCISlotNumberEnd {
		return constants.NilSlot
	}

	return slot
}

// AssignSlotNumber assigns a specific PCI slot number to the specified device. This ensures that
// the slot is valid and not in use by anything else in the spec
func (s *VirtualMachineConfigSpec) AssignSlotNumber(dev types.BaseVirtualDevice, known map[int32]bool) int32 {
	slot := VirtualDeviceSlotNumber(dev)
	if slot != constants.NilSlot {
		return slot
	}

	// build the slots in use from the spec
	slots := s.CollectSlotNumbers(known)
	slot = findSlotNumber(slots)
	if slot != constants.NilSlot {
		dev.GetVirtualDevice().SlotInfo = &types.VirtualDevicePciBusSlotInfo{PciSlotNumber: slot}
	}

	return slot
}

// CollectSlotNumbers returns a collection of all the PCI slot numbers for devices in the spec
// Can take a nil map as argument
func (s *VirtualMachineConfigSpec) CollectSlotNumbers(known map[int32]bool) map[int32]bool {
	if known == nil {
		known = make(map[int32]bool)
	}
	// collect all the already assigned slot numbers
	for _, c := range s.DeviceChange {
		if s := VirtualDeviceSlotNumber(c.GetVirtualDeviceConfigSpec().Device); s != constants.NilSlot {
			known[s] = true
		}
	}

	return known
}
