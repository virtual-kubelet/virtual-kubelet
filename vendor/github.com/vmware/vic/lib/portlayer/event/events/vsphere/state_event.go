// Copyright 2018 VMware, Inc. All Rights Reserved.
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
	"time"

	"github.com/vmware/govmomi/vim25/types"

	"github.com/vmware/vic/lib/portlayer/event/events"
	"github.com/vmware/vic/pkg/trace"
)

type StateEvent struct {
	*events.BaseEvent
}

func NewStateEvent(op trace.Operation, state types.VirtualMachinePowerState, ref types.ManagedObjectReference) *StateEvent {
	var ee string
	// vm power states that we care about
	switch state {
	case types.VirtualMachinePowerStatePoweredOn:
		ee = events.ContainerPoweredOn
	case types.VirtualMachinePowerStatePoweredOff:
		ee = events.ContainerPoweredOff
	case types.VirtualMachinePowerStateSuspended:
		ee = events.ContainerSuspended
	default:
		panic("Unknown event")
	}

	return &StateEvent{
		&events.BaseEvent{
			Event:       ee,
			ID:          op.ID(),
			Detail:      "Created from power state " + string(state),
			Ref:         ref.String(),
			CreatedTime: time.Now(),
		},
	}

}

func (se *StateEvent) Topic() string {
	if se.Type == "" {
		se.Type = events.NewEventType(se)
	}
	return se.Type.Topic()
}
