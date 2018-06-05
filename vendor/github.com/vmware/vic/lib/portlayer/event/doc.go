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

/*
Package event manages events via a simple pub / sub mechanism.  Events could be created
by vic components or registered Collectors.

Basic Overview

The Event Manager provides basic pub / sub functionality.  A subscription consists of a
topic (any defined Event), a subscription name (string) and a callback function.  When an event is
published the event manager will determine the event type and check to see if any components
have registered a callback for that event type.  For all subscriptions the event manager will
facilitate the callback.  Publication of events can be accomplished by any component that has
a pointer to the event manager or via the registered collectors.

Collectors are responsible for collecting events or data from external systems and then publishing
relevant vic events to the event manager.  In theory the collector could monitor anything and when
certain criteria are meet publish vic events to the manager.  Collectors are registered with the
event manager which instructs the collector where to publish.  Multiple collectors are allowed per
event manager, but each collector has a single publish target.

An example of a collector is the vSphere Event Collector which uses the vSphere EventHistoryCollector
to monitor the vSphere event stream and publish relevant events to vic.  In the initial implementation
the vSphere Event Collector is focused on a subset of VM Events that are then transformed to vic Events
and published to the event manager.
*/
package event
