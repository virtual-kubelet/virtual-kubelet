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

package event

import (
	"testing"

	"github.com/vmware/vic/lib/portlayer/event/collector/vsphere"
	"github.com/vmware/vic/lib/portlayer/event/events"

	"github.com/vmware/govmomi/vim25/types"

	"github.com/stretchr/testify/assert"
)

func TestNewManager(t *testing.T) {
	mgr := NewEventManager()
	assert.NotNil(t, mgr)
}

func TestTopic(t *testing.T) {
	vmEvent := newVMEvent()
	assert.Equal(t, vmEvent.Topic(), "vsphere.VMEvent")
}

func TestSubscribe(t *testing.T) {
	mgr := NewEventManager()
	topic := events.NewEventType(vsphere.VMEvent{}).Topic()
	mgr.Subscribe(topic, "tester", callback)
	subs := mgr.Subscribers()
	assert.Equal(t, 1, len(subs))
	assert.Equal(t, 1, mgr.Subscribed())

	mgr.Subscribe(topic, "tester2", callback)
	subs = mgr.Subscribers()

	// should still have 1 topic
	assert.Equal(t, 1, len(subs))
	// now two subscribers for that topic
	assert.Equal(t, 2, mgr.Subscribed())

	mgr.Subscribe(events.NewEventType(&vsphere.VMEvent{}).Topic(), "tester3", callback)
	subs = mgr.Subscribers()
	// should still have 1 topic
	assert.Equal(t, 1, len(subs))
	// now two subscribers for that topic
	assert.Equal(t, 3, mgr.Subscribed())

	mgr.Unsubscribe(topic, "tester2")
	subs = mgr.Subscribers()
	// should still have 1 topic
	assert.Equal(t, 1, len(subs))
	// now two subscribers for that topic
	assert.Equal(t, 2, mgr.Subscribed())

	mgr.Unsubscribe(events.NewEventType(&vsphere.VMEvent{}).Topic(), "tester3")
	subs = mgr.Subscribers()
	// should still have 1 topic
	assert.Equal(t, 1, len(subs))
	// now one subscribers for that topic
	assert.Equal(t, 1, mgr.Subscribed())

}

func TestRegisterCollector(t *testing.T) {
	mgr := NewEventManager()
	// register nil
	mgr.RegisterCollector(nil)
	assert.Equal(t, 0, len(mgr.Collectors()))
}

// utility methods
func newVMMO() *types.ManagedObjectReference {
	return &types.ManagedObjectReference{Value: "101", Type: "vm"}
}

func newBaseEvent() types.BaseEvent {
	vm := newVMMO()
	return types.BaseEvent(&types.VmPoweredOnEvent{VmEvent: types.VmEvent{Event: types.Event{Vm: &types.VmEventArgument{Vm: *vm}}}})
}

func newVMEvent() *vsphere.VMEvent {
	return vsphere.NewVMEvent(newBaseEvent())
}

func callback(e events.Event) {}
