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

package vsphere

import (
	"strconv"
	"testing"

	"github.com/vmware/vic/lib/portlayer/event/events"

	"github.com/vmware/govmomi/vim25/types"

	"github.com/stretchr/testify/assert"
)

const (
	LifeCycle = iota
	Reconfigure
	Mixed
)

// used to test callbacks
var callcount int

func newVMMO() *types.ManagedObjectReference {
	return &types.ManagedObjectReference{Value: "101", Type: "vm"}
}

func TestMonitoredObject(t *testing.T) {

	mgr := newCollector()
	mo := newVMMO()

	mgr.AddMonitoredObject(mo.String())
	mos := mgr.monitoredObjects()
	assert.Equal(t, 1, len(mos))
	mgr.RemoveMonitoredObject(mo.String())
	mos = mgr.monitoredObjects()
	assert.Equal(t, 0, len(mos))
}

func TestRegistration(t *testing.T) {
	mgr := newCollector()

	mgr.Register(callMe)
	assert.NotNil(t, mgr.callback)

}

func TestEvented(t *testing.T) {
	mgr := newCollector()
	callcount = 0

	// register local callback
	mgr.Register(callMe)

	// test lifecycle events
	page := eventPage(3, LifeCycle)
	evented(mgr, page)
	assert.Equal(t, 3, callcount)
}

func TestName(t *testing.T) {
	mgr := newCollector()
	assert.NotNil(t, mgr.Name())
	assert.Equal(t, name, mgr.Name())
}

func TestStart(t *testing.T) {
	mgr := newCollector()
	// start should fail as no objects registered
	assert.Error(t, mgr.Start())
}

func TestEventTypes(t *testing.T) {
	if len(eventTypes) != 9 {
		t.Fatalf("eventTypes=%d", len(eventTypes))
	}

	f := types.TypeFunc()

	for _, name := range eventTypes {
		_, ok := f(name)
		if !ok {
			t.Errorf("unknown event type: %q", name)
		}
	}
}

func newCollector() *EventCollector {
	return &EventCollector{mos: monitoredCache{mos: make(map[string]types.ManagedObjectReference)}, lastProcessedID: -1}
}

// simple callback counter
func callMe(vm events.Event) {
	callcount++
}

// utility function to mock a vsphere event
//
// size is the number of events to create
// lifeCycle is true when we want to generate state events
// lifeCycle events == poweredOn, poweredOff, etc..

func eventPage(size int, eventType int) []types.BaseEvent {
	page := make([]types.BaseEvent, 0, size)
	moid := 100
	for i := 0; i < size; i++ {
		var eve types.BaseEvent
		var eType int
		moid++
		vm := types.ManagedObjectReference{Value: strconv.Itoa(moid), Type: "vm"}
		eType = eventType
		if eType == Mixed {
			if i%2 == 0 {
				eType = LifeCycle
			} else {
				eType = Reconfigure
			}
		}
		if eType == LifeCycle {
			eve = types.BaseEvent(&types.VmPoweredOnEvent{VmEvent: types.VmEvent{Event: types.Event{Vm: &types.VmEventArgument{Vm: vm}}}})
		} else {
			eve = types.BaseEvent(&types.VmReconfiguredEvent{VmEvent: types.VmEvent{Event: types.Event{Vm: &types.VmEventArgument{Vm: vm}}}})
		}

		page = append(page, eve)
	}

	return page
}
