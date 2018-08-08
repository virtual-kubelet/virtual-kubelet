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

package events

import (
	"fmt"
	"path"
	"reflect"
	"time"
)

type EventType string

type BaseEvent struct {
	Type        EventType
	Event       string
	ID          string
	Detail      string
	Ref         string
	CreatedTime time.Time
}

func (be *BaseEvent) EventID() string {
	return be.ID
}

// return event type / description
func (be *BaseEvent) String() string {
	return be.Event
}

func (be *BaseEvent) Message() string {
	return be.Detail
}

func (be *BaseEvent) Reference() string {
	return be.Ref
}

func (be *BaseEvent) Created() time.Time {
	return be.CreatedTime
}

// NewEventType utility function that uses reflection to return
// the event type
func NewEventType(kind interface{}) EventType {
	t := reflect.TypeOf(kind)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return EventType(fmt.Sprintf("%s.%s", path.Base(t.PkgPath()), t.Name()))
}

func (t EventType) Topic() string {
	return string(t)
}
