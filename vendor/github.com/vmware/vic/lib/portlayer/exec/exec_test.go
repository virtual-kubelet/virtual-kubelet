// Copyright 2016-2018 VMware, Inc. All Rights Reserved.
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

package exec

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/vic/lib/portlayer/event"
	"github.com/vmware/vic/lib/portlayer/event/collector/vsphere"
	"github.com/vmware/vic/lib/portlayer/event/events"
	"github.com/vmware/vic/pkg/trace"
)

var containerEvents []events.Event

func TestEventedState(t *testing.T) {
	id := "123439"
	container := newTestContainer(id)
	addTestVM(container)
	op := trace.NewOperation(context.Background(), "test operations")
	// poweredOn event
	event := createVMEvent(container, StateRunning)
	assert.EqualValues(t, StateStarting, eventedState(op, event, StateStarting))
	assert.EqualValues(t, StateRunning, eventedState(op, event, StateRunning))
	assert.EqualValues(t, StateRunning, eventedState(op, event, StateStopped))
	assert.EqualValues(t, StateRunning, eventedState(op, event, StateSuspended))

	// // powerOff event
	event = createVMEvent(container, StateStopped)
	assert.EqualValues(t, StateStopping, eventedState(op, event, StateStopping))
	assert.EqualValues(t, StateStopped, eventedState(op, event, StateStopped))
	assert.EqualValues(t, StateStopped, eventedState(op, event, StateRunning))

	// // suspended event
	event = createVMEvent(container, StateSuspended)
	assert.EqualValues(t, StateSuspending, eventedState(op, event, StateSuspending))
	assert.EqualValues(t, StateSuspended, eventedState(op, event, StateSuspended))
	assert.EqualValues(t, StateSuspended, eventedState(op, event, StateRunning))

	// removed event
	event = createVMEvent(container, StateRemoved)
	assert.EqualValues(t, StateRemoved, eventedState(op, event, StateRemoved))
	assert.EqualValues(t, StateRemoved, eventedState(op, event, StateStopped))
	assert.EqualValues(t, StateRemoving, eventedState(op, event, StateRemoving))
}

func TestPublishContainerEvent(t *testing.T) {

	NewContainerCache()
	containerEvents = make([]events.Event, 0)
	Config = Configuration{}

	mgr := event.NewEventManager()
	Config.EventManager = mgr
	mgr.Subscribe(events.NewEventType(events.ContainerEvent{}).Topic(), "testing", containerCallback)

	op := trace.NewOperation(context.Background(), "test publish event operation")
	// create new running container and place in cache
	id := "123439"
	container := newTestContainer(id)
	addTestVM(container)
	container.SetState(op, StateRunning)
	Containers.Put(container)

	publishContainerEvent(trace.NewOperation(context.Background(), "container"), id, time.Now().UTC(), events.ContainerPoweredOff)
	time.Sleep(time.Millisecond * 30)

	assert.Equal(t, 1, len(containerEvents))
	assert.Equal(t, id, containerEvents[0].Reference())
	assert.Equal(t, events.ContainerPoweredOff, containerEvents[0].String())
}

func TestVMRemovedEventCallback(t *testing.T) {

	NewContainerCache()
	containerEvents = make([]events.Event, 0)
	Config = Configuration{}

	mgr := event.NewEventManager()
	Config.EventManager = mgr

	// subscribe the exec layer to the event stream for VM events
	mgr.Subscribe(events.NewEventType(&vsphere.VMEvent{}).Topic(), "testing", func(e events.Event) {
		if c := Containers.Container(e.Reference()); c != nil {
			c.OnEvent(e)
		}
	})

	op := trace.NewOperation(context.Background(), "test removed event operation")
	// create new running container and place in cache
	id := "123439"
	container := newTestContainer(id)
	addTestVM(container)
	container.SetState(op, StateRunning)
	Containers.Put(container)

	container.vm.EnterFixingState()
	vmEvent := createVMEvent(container, StateRemoved)

	mgr.Publish(vmEvent)
	time.Sleep(time.Millisecond * 30)
	assertMsg := "Container should have left fixing state in VM remove event handler"
	assert.False(t, container.vm.IsFixing(), assertMsg)

	mgr.Publish(vmEvent)
	time.Sleep(time.Millisecond * 30)
	assertMsg = "Container should be removed now that it has left fixing state"
	assert.True(t, Containers.Container(id) == nil, assertMsg)
}

func containerCallback(ee events.Event) {
	containerEvents = append(containerEvents, ee)
}

func createVMEvent(container *Container, state State) *vsphere.VMEvent {
	// event to return
	var vmEvent *vsphere.VMEvent
	// basic event info
	vme := types.Event{
		CreatedTime: time.Now().UTC(),
		Key:         int32(101),
		Vm: &types.VmEventArgument{
			Vm: container.vm.Reference(),
		},
	}

	switch state {
	case StateSuspended:
		// suspended
		vmwEve := &types.VmSuspendedEvent{
			VmEvent: types.VmEvent{
				Event: vme,
			},
		}
		vmEvent = vsphere.NewVMEvent(vmwEve)
	case StateStopped:
		// poweredOff
		vmwEve := &types.VmPoweredOffEvent{
			VmEvent: types.VmEvent{
				Event: vme,
			},
		}
		vmEvent = vsphere.NewVMEvent(vmwEve)
	case StateRemoved:
		// removed
		vmwEve := &types.VmRemovedEvent{
			VmEvent: types.VmEvent{
				Event: vme,
			},
		}
		vmEvent = vsphere.NewVMEvent(vmwEve)
	default:
		// poweredOn
		vmwEve := &types.VmPoweredOnEvent{
			VmEvent: types.VmEvent{
				Event: vme,
			},
		}
		vmEvent = vsphere.NewVMEvent(vmwEve)
	}

	return vmEvent
}
