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
	"context"
	"fmt"
	"reflect"
	"sync"

	"github.com/vmware/vic/lib/portlayer/event/events"

	vmwEvents "github.com/vmware/govmomi/event"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/types"

	log "github.com/Sirupsen/logrus"
)

const (
	name = "vSphere Event Collector"
)

type EventCollector struct {
	vmwManager *vmwEvents.Manager
	mos        monitoredCache
	callback   func(events.Event)

	lastProcessedID int32
}

type monitoredCache struct {
	mu sync.RWMutex

	mos map[string]types.ManagedObjectReference
}

func NewCollector(client *vim25.Client, objects ...string) *EventCollector {
	ec := &EventCollector{
		vmwManager: vmwEvents.NewManager(client),
		mos:        monitoredCache{mos: make(map[string]types.ManagedObjectReference)},
		// initialize to an index that will not be present in a page
		lastProcessedID: -1,
	}

	for i := range objects {
		// #nosec: Errors unhandled.
		ec.AddMonitoredObject(objects[i])
	}

	return ec
}

func (ec *EventCollector) Name() string {
	return name
}

// Register an event manager callback with the collector
func (ec *EventCollector) Register(callback func(events.Event)) {
	ec.callback = callback
}

func (ec *EventCollector) AddMonitoredObject(ref string) error {
	ec.mos.mu.Lock()
	defer ec.mos.mu.Unlock()

	moRef := types.ManagedObjectReference{}
	if !moRef.FromString(ref) {
		return fmt.Errorf("%s received an invalid Object to monitor(%s)", name, ref)
	}
	ec.mos.mos[ref] = moRef
	return nil
}

func (ec *EventCollector) RemoveMonitoredObject(ref string) {
	ec.mos.mu.Lock()
	defer ec.mos.mu.Unlock()

	delete(ec.mos.mos, ref)
}

func (ec *EventCollector) monitoredObjects() []types.ManagedObjectReference {
	ec.mos.mu.RLock()
	defer ec.mos.mu.RUnlock()

	refs := make([]types.ManagedObjectReference, 0, len(ec.mos.mos))
	for k := range ec.mos.mos {
		refs = append(refs, ec.mos.mos[k])
	}
	return refs
}
func (ec *EventCollector) Stop() {
	_, err := ec.vmwManager.Destroy(context.Background())
	if err != nil {
		log.Warnf("%s failed to destroy the govmomi manager: %s", name, err.Error())
	}
}

// eventTypes is used to filter the event collector so we only receive these event types.
var eventTypes []string

func init() {
	events := []types.BaseEvent{
		(*types.VmGuestShutdownEvent)(nil),
		(*types.VmPoweredOnEvent)(nil),
		(*types.DrsVmPoweredOnEvent)(nil),
		(*types.VmPoweredOffEvent)(nil),
		(*types.VmRemovedEvent)(nil),
		(*types.VmSuspendedEvent)(nil),
		(*types.VmMigratedEvent)(nil),
		(*types.DrsVmMigratedEvent)(nil),
		(*types.VmRelocatedEvent)(nil),
	}

	for _, event := range events {
		eventTypes = append(eventTypes, reflect.TypeOf(event).Elem().Name())
	}
}

// Start the event collector
func (ec *EventCollector) Start() error {
	// array of managed objects
	refs := ec.monitoredObjects()

	// only continue if we have object to monitor
	if len(refs) == 0 {
		return fmt.Errorf("%s requires at least one Monitored Object: objects[%d]", name, 0)
	}

	log.Debugf("%s starting collection for %d managed objects", name, len(refs))

	// we don't want the event listener to timeout
	ctx := context.Background()

	// pageSize is the number of events on the last page of the eventCollector
	// as new events are added the oldest are removed.  Originally this value
	// was 1 and we encountered missed events due to them being evicted
	// before being processed. We bumped to 25 but we still miss events during
	// storms such as a host HA event.
	// Setting pageSize to 1000 overwhelmed hostd via the task history and caused
	// memory exhaustion. Setting pagesize to 200 while filtering for the specific
	// types we require showed directly comparable memory overhead vs the 25 page
	// size setting when running full ci. We may still have significantly higher
	// memory usage in the scenario where we legitimately have events of interest
	// at a rate of greater than 25 per page.
	// This should eventually be replaced with a smaller maximum page size, a page
	// cursor, and maybe a sliding window for the actual page size.
	pageSize := int32(200)
	// bool to follow the stream
	followStream := true
	// don't exceed the govmomi object limit
	force := false

	//TODO: need a proper way to handle failures / status
	go func(pageSize int32, follow bool, ff bool, refs []types.ManagedObjectReference, ec *EventCollector) error {
		// the govmomi event listener can only be configured once per session -- so if it's already listening it
		// will be replaced
		//
		// the manager will be closed with the session

		for {
			err := ec.vmwManager.Events(ctx, refs, pageSize, followStream, force, func(_ types.ManagedObjectReference, page []types.BaseEvent) error {
				evented(ec, page)
				return nil
			}, eventTypes...)
			// TODO: this will disappear in the ether
			if err != nil {
				log.Debugf("Error configuring %s: %s", name, err.Error())
			}
		}
	}(pageSize, followStream, force, refs, ec)

	return nil
}

// evented will process the event and execute the registered callback
//
// Initial implmentation will only act on certain events -- future implementations
// may provide more flexibility
func evented(ec *EventCollector, page []types.BaseEvent) {
	if ec.callback == nil {
		log.Warn("No callback defined for EventManager")
		return
	}

	if len(page) == 0 {
		return
	}

	// skip events already seen
	oldIndex := len(page)
	for i := range page {
		if page[i].GetEvent().Key == ec.lastProcessedID {
			oldIndex = i
		}
	}

	// events appear in page with most recent first - need to reverse for sane ordering
	// we start from the first new event after the last one processed
	for i := oldIndex - 1; i >= 0; i-- {
		ec.callback(NewVMEvent(page[i]))

		ec.lastProcessedID = page[i].GetEvent().Key
	}

}
