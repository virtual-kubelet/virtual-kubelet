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

package guest

import (
	"fmt"

	"context"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/vic/lib/constants"
	"github.com/vmware/vic/lib/spec"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/vsphere/session"
	"github.com/vmware/vic/pkg/vsphere/sys"
)

const (
	scsiBusNumber = 0
	scsiKey       = 100
	ideKey        = 200
)

// LinuxGuestType type
type LinuxGuestType struct {
	*spec.VirtualMachineConfigSpec

	// holds the controller so that we don't end up calling
	// FindIDEController or FindSCSIController
	controller types.BaseVirtualController
}

// NewLinuxGuest returns a new Linux guest spec with predefined values
func NewLinuxGuest(ctx context.Context, session *session.Session, config *spec.VirtualMachineConfigSpecConfig) (Guest, error) {
	s, err := spec.NewVirtualMachineConfigSpec(ctx, session, config)
	if err != nil {
		return nil, err
	}

	// SCSI controller
	scsi := spec.NewVirtualSCSIController(scsiBusNumber, scsiKey)
	// PV SCSI controller
	pv := spec.NewParaVirtualSCSIController(scsi)
	s.AddParaVirtualSCSIController(pv)

	// IDE controller
	ide := spec.NewVirtualIDEController(ideKey)
	s.AddVirtualIDEController(ide)

	// CDROM
	cdrom := spec.NewVirtualCdrom(ide)
	s.AddVirtualCdrom(cdrom)

	// Set the guest id
	s.GuestId = string(types.VirtualMachineGuestOsIdentifierOtherGuest64)
	s.AlternateGuestName = constants.DefaultAltContainerGuestName()

	return &LinuxGuestType{
		VirtualMachineConfigSpec: s,
		controller:               &scsi,
	}, nil
}

// GuestID returns the guest id of the linux guest
func (l *LinuxGuestType) GuestID() string {
	return l.VirtualMachineConfigSpec.GuestId
}

// Spec returns the underlying types.VirtualMachineConfigSpec to the caller
func (l *LinuxGuestType) Spec() *spec.VirtualMachineConfigSpec {
	return l.VirtualMachineConfigSpec
}

// Controller returns the types.BaseVirtualController to the caller
func (l *LinuxGuestType) Controller() *types.BaseVirtualController {
	return &l.controller
}

func (l *LinuxGuestType) NewDisk() *types.VirtualDisk {
	return spec.NewVirtualDisk(l.controller)
}

// GetSelf gets VirtualMachine reference for the VM this process is running on
func GetSelf(ctx context.Context, s *session.Session) (*object.VirtualMachine, error) {
	u, err := sys.UUID()
	if err != nil {
		return nil, err
	}

	search := object.NewSearchIndex(s.Vim25())
	ref, err := search.FindByUuid(ctx, s.Datacenter, u, true, nil)
	if err != nil {
		return nil, err
	}

	if ref == nil {
		return nil, fmt.Errorf("can't find the hosting vm")
	}

	vm := object.NewVirtualMachine(s.Client.Client, ref.Reference())
	return vm, nil
}

func IsSelf(op trace.Operation, uuid string) (bool, error) {
	self, err := sys.UUID()
	if err != nil {
		return false, err
	}

	return self == uuid, nil
}
