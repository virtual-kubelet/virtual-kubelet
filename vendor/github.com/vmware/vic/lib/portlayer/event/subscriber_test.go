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

package event

import (
	"strconv"
	"testing"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/vmware/vic/lib/portlayer/event/events"
)

func init() {
	log.SetLevel(log.DebugLevel)
}

type mockCollector struct {
	c func(events.Event)
}

// AddMonitoredObject will add the object for event listening
func (m *mockCollector) AddMonitoredObject(_ string) error {
	return nil
}

// RemoveMonitoredObject will remove the object from event listening
func (m *mockCollector) RemoveMonitoredObject(_ string) {
}

// Start listening for events and publish to function
func (m *mockCollector) Start() error {
	return nil
}

// Stop listening for events
func (m *mockCollector) Stop() {}

// Register a callback function
func (m *mockCollector) Register(c func(events.Event)) {
	m.c = c
}

// Name returns the collector name
func (m *mockCollector) Name() string {
	return "mock"
}

type mockEvent struct {
	id string
}

// id of event
func (e *mockEvent) EventID() string {
	return e.id
}

// event (PowerOn, PowerOff, etc)
func (e *mockEvent) String() string {
	return e.id
}

// reference evented object
func (e *mockEvent) Reference() string {
	return ""
}

// event message
func (e *mockEvent) Message() string {
	return ""
}

func (e *mockEvent) Created() time.Time {
	return time.Now()
}

func (e *mockEvent) Topic() string {
	return "test"
}

func TestSuspendQueue(t *testing.T) {
	c := &mockCollector{}
	m := NewEventManager(c)
	var evs []events.Event
	done := make(chan struct{})
	s := m.Subscribe("test", "test", func(e events.Event) {
		evs = append(evs, e)
		if len(evs) == 100 {
			close(done)
		}
	})

	suspended := false
	for i := 0; i < 100; i++ {
		if !suspended && i >= 50 {
			s.Suspend(true)
			assert.True(t, s.IsSuspended())
			suspended = true
		}
		c.c(&mockEvent{id: strconv.Itoa(i)})
	}

	select {
	case <-done:
		assert.Fail(t, "unexpectedly got all events despite suspend")
	case <-time.After(2 * time.Second):
		assert.Condition(t, func() bool { return len(evs) <= 50 })
	}

	s.Resume()
	assert.False(t, s.IsSuspended())

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		assert.Fail(t, "timed out waiting for suspended events")
	}

	// check for dups
	for i := range evs {
		for j := range evs {
			if j == i {
				continue
			}
			if evs[j].EventID() == evs[i].EventID() {
				assert.Fail(t, "dup event found for id %d", evs[j].EventID())
			}
		}
	}
}

func TestSuspendDiscard(t *testing.T) {
	c := &mockCollector{}
	m := NewEventManager(c)
	var evs []events.Event
	s := m.Subscribe("test", "test", func(e events.Event) {
		assert.Fail(t, "got an event %q when expecting none", e)
	})

	// discard events
	s.Suspend(false)
	assert.True(t, s.IsSuspended())
	for i := 0; i < 50; i++ {
		c.c(&mockEvent{id: strconv.Itoa(i)})
	}

	<-time.After(5 * time.Second)

	assert.Empty(t, evs)
	assert.Equal(t, uint64(50), s.Discarded())
	assert.Equal(t, uint64(0), s.Dropped())
}

func TestSuspendOverflow(t *testing.T) {
	c := &mockCollector{}
	m := NewEventManager(c)
	var evs []events.Event
	done := make(chan struct{})
	s := m.Subscribe("test", "test", func(e events.Event) {
		evs = append(evs, e)
		if len(evs) == maxEventQueueSize {
			close(done)
		}
	})

	s.Suspend(true)
	assert.True(t, s.IsSuspended())
	for i := 0; i < maxEventQueueSize+1; i++ {
		c.c(&mockEvent{id: strconv.Itoa(i)})
	}

	select {
	case <-done:
		assert.Fail(t, "unexpectedly got all events despite suspend")
	case <-time.After(5 * time.Second):
	}

	s.Resume()
	assert.False(t, s.IsSuspended())

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		assert.Fail(t, "timed out waiting for sentinel event")
	}

	assert.Len(t, evs, maxEventQueueSize)
	assert.Equal(t, uint64(1), s.Dropped())
	assert.Equal(t, uint64(0), s.Discarded())

	// check for dups
	for i := range evs {
		// should not have an event with id 0
		assert.NotEqual(t, 0, evs[i].EventID(), "got event with event id 0")
		for j := range evs {
			if j == i {
				continue
			}
			if evs[j].EventID() == evs[i].EventID() {
				assert.Fail(t, "dup event found for id %d", evs[j].EventID())
			}
		}
	}
}
