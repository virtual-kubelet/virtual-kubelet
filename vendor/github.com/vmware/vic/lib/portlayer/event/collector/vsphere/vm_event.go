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

package vsphere

import (
	"strconv"

	"github.com/vmware/govmomi/vim25/types"

	"github.com/vmware/vic/lib/portlayer/event/events"
)

type VMEvent struct {
	*events.BaseEvent
}

func NewVMEvent(be types.BaseEvent) *VMEvent {
	var ee string
	// vm events that we care about
	switch be.(type) {
	case *types.VmPoweredOnEvent,
		*types.DrsVmPoweredOnEvent:
		ee = events.ContainerPoweredOn
	case *types.VmPoweredOffEvent:
		ee = events.ContainerPoweredOff
	case *types.VmSuspendedEvent:
		ee = events.ContainerSuspended
	case *types.VmRemovedEvent:
		ee = events.ContainerRemoved
	case *types.VmGuestShutdownEvent:
		ee = events.ContainerShutdown
	case *types.VmMigratedEvent:
		ee = events.ContainerMigrated
	case *types.DrsVmMigratedEvent:
		ee = events.ContainerMigratedByDrs
	case *types.VmRelocatedEvent:
		ee = events.ContainerRelocated
	default:
		panic("Unknown event")
	}
	e := be.GetEvent()
	return &VMEvent{
		&events.BaseEvent{
			Event:       ee,
			ID:          strconv.Itoa(int(e.Key)),
			Detail:      e.FullFormattedMessage,
			Ref:         e.Vm.Vm.String(),
			CreatedTime: e.CreatedTime,
		},
	}

}

func (vme *VMEvent) Topic() string {
	if vme.Type == "" {
		vme.Type = events.NewEventType(vme)
	}
	return vme.Type.Topic()
}
