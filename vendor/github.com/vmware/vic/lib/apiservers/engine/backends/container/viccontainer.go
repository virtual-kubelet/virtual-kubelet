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

package container

import (
	"sync"
	"time"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
)

// VicContainer is VIC's abridged version of Docker's container object.
type VicContainer struct {
	Name        string
	ImageID     string // maps to the image used by this container
	LayerID     string // child-most layer ID used to find vmdk for this container
	ContainerID string
	Config      *containertypes.Config //Working copy of config (with overrides from container create)
	HostConfig  *containertypes.HostConfig
	NATMap      nat.PortMap // the endpoint NAT mappings only

	m        sync.RWMutex
	execs    map[string]struct{}
	lockChan chan bool
}

// NewVicContainer returns a reference to a new VicContainer
func NewVicContainer() *VicContainer {
	vc := &VicContainer{
		Config:   &containertypes.Config{},
		execs:    make(map[string]struct{}),
		lockChan: make(chan bool, 1),
	}
	return vc
}

// Add adds a new exec configuration to the container.
func (v *VicContainer) Add(id string) {
	v.m.Lock()
	v.execs[id] = struct{}{}
	v.m.Unlock()
}

// Delete removes an exec configuration from the container.
func (v *VicContainer) Delete(id string) {
	v.m.Lock()
	delete(v.execs, id)
	v.m.Unlock()
}

// List returns the list of exec ids in the container.
func (v *VicContainer) List() []string {
	var IDs []string
	v.m.RLock()
	for id := range v.execs {
		IDs = append(IDs, id)
	}
	v.m.RUnlock()
	return IDs
}

// Tries to lock the container.  Timeout argument defines how long the lock
// attempt will be tried.  Returns true if locked, false if timed out.
func (v *VicContainer) TryLock(timeout time.Duration) bool {
	timeChan := time.After(timeout)
	select {
	case <-timeChan:
		return false
	case v.lockChan <- true:
		return true
	}
}

// Unlocks the container
func (v *VicContainer) Unlock() {
	select {
	case <-v.lockChan:
	default:
		panic("Attempt to release container %s's lock that is not locked")
	}
}
