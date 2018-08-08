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
	"sync/atomic"

	log "github.com/Sirupsen/logrus"

	"github.com/vmware/vic/lib/portlayer/event/events"
)

const (
	suspendDisabled int32 = iota
	suspendDiscard
	suspendQueue
)

type Subscriber interface {
	// Topic returns the topic this subscriber is subscribed to
	Topic() string
	// Name returns the name of the subscriber
	Name() string

	// Suspend suspends processing events by the subscriber. If
	// queueEvents is true, the events are queued until Resume()
	// is called. If queueEvents is false, events passed into
	// onEvent() after this call are discarded.
	Suspend(queueEvents bool)
	// Resume resumes processing of events by the subscriber.
	// If Suspend() was called with queueEvents as true, any events
	// that were passed to onEvent() after Suspend() returned are
	// processed first.
	Resume()
	// IsSuspended returns true if the subscriber is suspended.
	IsSuspended() bool

	// Discarded returns the number of packets that were discarded by
	// the subscriber as a result of Pause() being called with
	// queueEvents as false.
	Discarded() uint64
	// Dropped returns the number of packets that were dropped when
	// the event queue overflows. This only happens when Pause()
	// is called with queueEvents as true.
	Dropped() uint64

	// onEvent is called by event.Manager to send an event to
	// a subscriber
	onEvent(events.Event)
}

type subscriber struct {
	topic              string
	name               string
	callback           func(e events.Event)
	eventQ             chan events.Event
	suspendState       int32
	discarded, dropped uint64
	suspend            chan suspendCmd
}

type suspendCmd struct {
	suspend bool
	done    chan struct{}
}

const maxEventQueueSize = 1000

// newSubscriber creates a new subscriber to topic
func newSubscriber(topic, name string, callback func(e events.Event)) Subscriber {
	s := &subscriber{
		topic:    topic,
		name:     name,
		callback: callback,
		eventQ:   make(chan events.Event, maxEventQueueSize),
		suspend:  make(chan suspendCmd),
	}

	go func() {
		suspended := false
		var done chan struct{}
		for {
			if done != nil {
				done <- struct{}{}
				done = nil
			}

			if suspended {
				select {
				case c := <-s.suspend:
					suspended = c.suspend
					done = c.done
				}

				continue
			}

			// not suspended
			select {
			case e := <-s.eventQ:
				s.callback(e)
			case c := <-s.suspend:
				suspended = c.suspend
				done = c.done
			}
		}
	}()

	return s
}

// Topic returns the topic this subscriber is subscribed to
func (s *subscriber) Topic() string {
	return s.topic
}

// Name returns the name of the subscriber
func (s *subscriber) Name() string {
	return s.name
}

// onEvent is called by event.Manager to send an event to
// a subscriber
func (s *subscriber) onEvent(e events.Event) {
	switch atomic.LoadInt32(&s.suspendState) {
	case suspendDisabled:
		s.eventQ <- e
	case suspendDiscard:
		log.Warnf("discarding event %q", e)
		atomic.AddUint64(&s.discarded, 1)
	case suspendQueue:
		done := false
		for !done {
			select {
			case s.eventQ <- e:
				done = true
			default:
				// make room; discard oldest
				log.Warnf("dropping event %q", <-s.eventQ)
				atomic.AddUint64(&s.dropped, 1)
			}
		}
	}
}

// Suspend suspends processing events by the subscriber. If
// queueEvents is true, the events are queued until Resume()
// is called. If queueEvents is false, events passed into
// onEvent() after this call are discarded.
func (s *subscriber) Suspend(queueEvents bool) {
	defer func() {
		done := make(chan struct{})
		s.suspend <- suspendCmd{suspend: true, done: done}
		<-done
		close(done)
	}()

	if queueEvents {
		atomic.StoreInt32(&s.suspendState, suspendQueue)
		return
	}

	atomic.StoreInt32(&s.suspendState, suspendDiscard)
}

// Resume resumes processing of events by the subscriber.
// If Suspend() was called with queueEvents as true, any events
// that were passed to onEvent() after Suspend() returned are
// processed first.
func (s *subscriber) Resume() {
	defer func() {
		done := make(chan struct{})
		s.suspend <- suspendCmd{suspend: false, done: done}
		<-done
		close(done)
	}()
	atomic.StoreInt32(&s.suspendState, suspendDisabled)
}

// IsSuspended returns true if the subscriber is suspended.
func (s *subscriber) IsSuspended() bool {
	return atomic.LoadInt32(&s.suspendState) != suspendDisabled
}

// Discarded returns the number of packets that were discarded by
// the subscriber as a result of Pause() being called with
// queueEvents as false.
func (s *subscriber) Discarded() uint64 {
	return atomic.LoadUint64(&s.discarded)
}

// Dropped returns the number of packets that were dropped when
// the event queue overflows. This only happens when Pause()
// is called with queueEvents as true.
func (s *subscriber) Dropped() uint64 {
	return atomic.LoadUint64(&s.dropped)
}
