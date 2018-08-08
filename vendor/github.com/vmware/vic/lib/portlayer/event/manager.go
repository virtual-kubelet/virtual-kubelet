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
	"fmt"
	"sync"

	"github.com/vmware/vic/lib/portlayer/event/collector"
	"github.com/vmware/vic/lib/portlayer/event/events"
	"github.com/vmware/vic/pkg/trace"

	log "github.com/Sirupsen/logrus"
)

type Manager struct {
	cos    collectorCache
	subs   subscriberCache
	eventQ chan events.Event
}

const eventQSize = 1000

type collectorCache struct {
	mu sync.RWMutex

	collectors map[string]collector.Collector
}

type subscriberCache struct {
	mu sync.RWMutex

	subscribers map[string]map[string]Subscriber
}

func NewEventManager(collectors ...collector.Collector) *Manager {
	mgr := &Manager{
		cos: collectorCache{
			collectors: make(map[string]collector.Collector),
		},
		subs: subscriberCache{
			subscribers: make(map[string]map[string]Subscriber),
		},
		eventQ: make(chan events.Event, eventQSize),
	}

	// register any collectors provided
	for i := range collectors {
		mgr.RegisterCollector(collectors[i])
	}

	// event processor routine
	go func() {
		for e := range mgr.eventQ {
			// subscribers for this event
			mgr.subs.mu.RLock()
			subs := mgr.subs.subscribers[e.Topic()]
			mgr.subs.mu.RUnlock()

			log.Debugf("Found %d subscribers to %s: %s - %s", len(subs), e.EventID(), e.Topic(), e.Message())

			for sub, s := range subs {
				log.Debugf("Event manager calling back to %s for Event(%s): %s", sub, e.EventID(), e.Topic())
				s.onEvent(e)
			}
		}
	}()

	return mgr
}

func (mgr *Manager) RegisterCollector(collector collector.Collector) {
	if collector == nil {
		return
	}

	mgr.cos.mu.Lock()
	defer mgr.cos.mu.Unlock()

	collector.Register(mgr.Publish)

	mgr.cos.collectors[collector.Name()] = collector
}

func (mgr *Manager) Collectors() map[string]collector.Collector {
	mgr.cos.mu.RLock()
	defer mgr.cos.mu.RUnlock()

	c := make(map[string]collector.Collector)
	for name, collector := range mgr.cos.collectors {
		c[name] = collector
	}
	return c
}

// Subscribe to the event manager for callback
func (mgr *Manager) Subscribe(eventTopic string, caller string, callback func(events.Event)) Subscriber {
	defer trace.End(trace.Begin(fmt.Sprintf("%s:%s", eventTopic, caller)))
	mgr.subs.mu.Lock()
	defer mgr.subs.mu.Unlock()

	if _, ok := mgr.subs.subscribers[eventTopic]; !ok {
		mgr.subs.subscribers[eventTopic] = make(map[string]Subscriber)
	}
	s := newSubscriber(eventTopic, caller, callback)
	mgr.subs.subscribers[eventTopic][caller] = s
	return s
}

// Unsubscribe from callbacks
func (mgr *Manager) Unsubscribe(eventTopic string, caller string) {
	defer trace.End(trace.Begin(fmt.Sprintf("%s:%s", eventTopic, caller)))
	mgr.subs.mu.Lock()
	defer mgr.subs.mu.Unlock()
	if _, ok := mgr.subs.subscribers[eventTopic]; ok {
		delete(mgr.subs.subscribers[eventTopic], caller)
	}
}

func (mgr *Manager) Subscribers() map[string]map[string]Subscriber {
	mgr.subs.mu.RLock()
	defer mgr.subs.mu.RUnlock()
	s := make(map[string]map[string]Subscriber)
	for i, m := range mgr.subs.subscribers {

		if _, ok := s[i]; !ok {
			s[i] = make(map[string]Subscriber)
		}

		for k, v := range m {
			s[i][k] = v
		}
	}
	return s
}

// RegistryCount returns the callback count
func (mgr *Manager) Subscribed() int {
	mgr.subs.mu.RLock()
	defer mgr.subs.mu.RUnlock()
	count := 0
	for _, m := range mgr.subs.subscribers {
		count += len(m)
	}
	return count
}

// Publish events to subscribers
func (mgr *Manager) Publish(e events.Event) {
	mgr.eventQ <- e
}
