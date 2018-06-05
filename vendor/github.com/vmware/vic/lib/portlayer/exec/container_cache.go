// Copyright 2016-2018 VMware, Inc. All Rights Reserved.
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

package exec

import (
	"sync"

	"context"

	"github.com/vmware/vic/pkg/uid"
	"github.com/vmware/vic/pkg/vsphere/session"
)

/*
* ContainerCache will provide an in-memory cache of containerVMs.  It will
* be refreshed on portlayer start and updated via container lifecycle
* operations (start, stop, rm) and well as in response to infrastructure
* events
 */
type containerCache struct {
	m sync.RWMutex

	// cache by container id
	cache map[string]*Container
}

var Containers *containerCache

func NewContainerCache() {
	// cache by the container ID and the vsphere
	// managed object reference
	Containers = &containerCache{
		cache: make(map[string]*Container),
	}
}

func (conCache *containerCache) Container(idOrRef string) *Container {
	conCache.m.RLock()
	defer conCache.m.RUnlock()
	// find by id or moref
	return conCache.cache[idOrRef]
}

func (conCache *containerCache) Containers(states []State) []*Container {
	conCache.m.RLock()
	defer conCache.m.RUnlock()
	// cache contains 2 items for each container
	capacity := len(conCache.cache) / 2
	containers := make([]*Container, 0, capacity)

	for id, con := range conCache.cache {
		// is the key a proper ID?
		if !isContainerID(id) {
			continue
		}

		// no state filtering
		if len(states) == 0 {
			containers = append(containers, con)
			continue
		}

		// filter by container state
		// DO NOT use container.CurrentState as that can
		// cause cache deadlocks
		for _, state := range states {
			if state == con.State() {
				containers = append(containers, con)
			}
		}
	}

	return containers
}

// puts a container in the cache and will overwrite an existing container
func (conCache *containerCache) Put(container *Container) {
	// only add containers w/backing VMs
	if container.vm == nil {
		return
	}

	conCache.m.Lock()
	defer conCache.m.Unlock()

	conCache.put(container)
}

func (conCache *containerCache) put(container *Container) {
	// add pointer to cache by container ID
	conCache.cache[container.ExecConfig.ID] = container
	conCache.cache[container.vm.Reference().String()] = container

}

func (conCache *containerCache) Remove(idOrRef string) {
	conCache.m.Lock()
	defer conCache.m.Unlock()
	// find by id
	container := conCache.cache[idOrRef]
	if container != nil {
		delete(conCache.cache, container.ExecConfig.ID)
		delete(conCache.cache, container.vm.Reference().String())
	}
}

func (conCache *containerCache) sync(ctx context.Context, sess *session.Session) error {
	conCache.m.Lock()
	defer conCache.m.Unlock()

	cons, err := infraContainers(ctx, sess)
	if err != nil {
		return err
	}

	conCache.cache = make(map[string]*Container)
	for _, c := range cons {
		conCache.put(c)
	}

	return nil
}

func isContainerID(id string) bool {
	return uid.Parse(id) != uid.NilUID
}
