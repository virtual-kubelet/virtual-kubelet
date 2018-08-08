// Copyright 2017 VMware, Inc. All Rights Reserved.
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

package backends

import (
	"encoding/json"
	"fmt"
	"io"
	"testing"
	"time"

	"golang.org/x/net/context"

	"github.com/stretchr/testify/assert"

	plevents "github.com/vmware/vic/lib/portlayer/event/events"
)

type MockEventProxy struct {
	MockEvents []plevents.BaseEvent
	Delay      time.Duration
}

type MockEventPublisher struct {
	MockEventChan chan plevents.BaseEvent
}

func (ep *MockEventProxy) StreamEvents(ctx context.Context, out io.Writer) error {
	encoder := json.NewEncoder(out)
	if encoder == nil {
		return fmt.Errorf("Failed to create a json encoder")
	}

	for _, event := range ep.MockEvents {
		if err := encoder.Encode(event); err != nil {
			return err
		}

		time.Sleep(ep.Delay)
	}

	return nil
}

func (p *MockEventPublisher) PublishEvent(event plevents.BaseEvent) {
	if p.MockEventChan != nil {
		p.MockEventChan <- event
	}
}

func TestStartStopMonitor(t *testing.T) {
	proxy := MockEventProxy{
		MockEvents: []plevents.BaseEvent{
			{
				Event: plevents.ContainerCreated,
				Ref:   "abc",
			},
			{
				Event: plevents.ContainerStarted,
				Ref:   "abc",
			},
			{
				Event: plevents.ContainerStopped,
				Ref:   "abc",
			},
		},
		Delay: 1 * time.Second,
	}
	publisher := MockEventPublisher{
		MockEventChan: make(chan plevents.BaseEvent, 3),
	}
	monitor := NewPortlayerEventMonitor(&proxy, &publisher)

	var err error

	// The actual tests
	err = monitor.Start()
	assert.Nil(t, err, "Expected monitor start to succeed, but received: %#v", err)

	err = monitor.Start()
	assert.NotEqual(t, err, nil, "Expected error but received nil on double start")
	if err != nil {
		assert.Contains(t, err.Error(), "Already started", "Expected already started error but received: %s", err)
	}

	monitor.Stop()
}

func TestEventMonitor(t *testing.T) {
	proxy := MockEventProxy{
		MockEvents: []plevents.BaseEvent{
			{
				Event: plevents.ContainerCreated,
				Ref:   "abc",
			},
			{
				Event: plevents.ContainerStarted,
				Ref:   "abc",
			},
			{
				Event: plevents.ContainerStopped,
				Ref:   "abc",
			},
		},
		Delay: 0,
	}
	publisher := MockEventPublisher{
		MockEventChan: make(chan plevents.BaseEvent, 3),
	}
	monitor := NewPortlayerEventMonitor(&proxy, &publisher)

	var err error

	// The actual tests
	err = monitor.Start()
	assert.Nil(t, err, "Expected monitor start to succeed, but received: %#v", err)

	time.Sleep(1 * time.Second)

	count := len(publisher.MockEventChan)
	for i := 0; i < count; i++ {
		event := <-publisher.MockEventChan
		assert.Equal(t, event.Event, proxy.MockEvents[i].Event, "Expected to find event %s but found %s", proxy.MockEvents[i].Event, event.Event)
	}
}
