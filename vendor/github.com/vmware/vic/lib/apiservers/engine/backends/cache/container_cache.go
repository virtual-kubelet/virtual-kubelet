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

package cache

import (
	"fmt"
	"sync"

	log "github.com/Sirupsen/logrus"

	derr "github.com/docker/docker/api/errors"
	"github.com/docker/docker/pkg/truncindex"

	"github.com/vmware/vic/lib/apiservers/engine/backends/container"
)

// Tracks our container info from calls
type CCache struct {
	m sync.RWMutex

	idIndex            *truncindex.TruncIndex
	containersByID     map[string]*container.VicContainer
	containersByName   map[string]*container.VicContainer
	containersByExecID map[string]*container.VicContainer
}

var containerCache *CCache

func init() {
	containerCache = &CCache{
		idIndex:            truncindex.NewTruncIndex([]string{}),
		containersByID:     make(map[string]*container.VicContainer),
		containersByName:   make(map[string]*container.VicContainer),
		containersByExecID: make(map[string]*container.VicContainer),
	}
}

// ContainerCache returns a reference to the container cache
func ContainerCache() *CCache {
	return containerCache
}

func (cc *CCache) getContainerByName(nameOnly string) *container.VicContainer {
	if container, exist := cc.containersByName[nameOnly]; exist {
		return container
	}
	return nil
}

func (cc *CCache) getContainer(nameOrID string) *container.VicContainer {
	// full name matching should take precedence over id prefix matching
	if container, exist := cc.containersByName[nameOrID]; exist {
		return container
	}

	// get the full ID if we only have a prefix
	if cid, err := cc.idIndex.Get(nameOrID); err == nil {
		nameOrID = cid
	}

	if container, exist := cc.containersByID[nameOrID]; exist {
		return container
	}
	return nil
}

// GetContainerByName returns a container whose name "exactly" matches nameOnly
func (cc *CCache) GetContainerByName(nameOnly string) *container.VicContainer {
	cc.m.RLock()
	defer cc.m.RUnlock()

	return cc.getContainerByName(nameOnly)
}

func (cc *CCache) GetContainer(nameOrID string) *container.VicContainer {
	cc.m.RLock()
	defer cc.m.RUnlock()

	return cc.getContainer(nameOrID)
}

func (cc *CCache) AddContainer(container *container.VicContainer) {
	cc.m.Lock()
	defer cc.m.Unlock()

	// TODO(jzt): this probably shouldn't assume a valid container ID
	if err := cc.idIndex.Add(container.ContainerID); err != nil {
		log.Warnf("Error adding ID into index: %s", err)
	}
	cc.containersByID[container.ContainerID] = container
	cc.containersByName[container.Name] = container
}

func (cc *CCache) DeleteContainer(nameOrID string) {
	cc.m.Lock()
	defer cc.m.Unlock()

	container := cc.getContainer(nameOrID)
	if container == nil {
		return
	}

	delete(cc.containersByID, container.ContainerID)
	delete(cc.containersByName, container.Name)

	if err := cc.idIndex.Delete(container.ContainerID); err != nil {
		log.Warnf("Error deleting ID from index: %s", err)
	}

	// remove exec references
	for _, id := range container.List() {
		container.Delete(id)
	}
}

func (cc *CCache) AddExecToContainer(container *container.VicContainer, eid string) {
	cc.m.Lock()
	defer cc.m.Unlock()

	// ignore if we already have it
	if _, ok := cc.containersByExecID[eid]; ok {
		return
	}

	container.Add(eid)
	cc.containersByExecID[eid] = container
}

func (cc *CCache) GetContainerFromExec(eid string) *container.VicContainer {
	cc.m.RLock()
	defer cc.m.RUnlock()

	if container, exist := cc.containersByExecID[eid]; exist {
		return container
	}
	return nil
}

// UpdateContainerName assumes that the newName is already reserved by ReserveName
// so no need to check the existence of a container with the new name.
func (cc *CCache) UpdateContainerName(oldName, newName string) error {
	cc.m.Lock()
	defer cc.m.Unlock()

	container := cc.getContainer(oldName)
	if container == nil {
		return derr.NewRequestNotFoundError(fmt.Errorf("no such container: %s", oldName))
	}

	delete(cc.containersByName, container.Name)

	container.Name = newName
	cc.containersByName[newName] = container
	cc.containersByID[container.ContainerID] = container

	return nil
}

// ReserveName is used during a container create/rename operation to prevent concurrent
// container create/rename operations from grabbing the new name.
func (cc *CCache) ReserveName(container *container.VicContainer, name string) error {
	cc.m.Lock()
	defer cc.m.Unlock()

	if cont, exist := cc.containersByName[name]; exist {
		return fmt.Errorf("conflict. The name %q is already in use by container %s. You have to remove (or rename) that container to be able to re use that name.", name, cont.ContainerID)
	}

	cc.containersByName[name] = container

	return nil
}

// ReleaseName is used during a container rename operation to allow concurrent container
// create/rename operations to use the name. It is also used during a failed create
// operation to allow subsequent create operations to use that name.
func (cc *CCache) ReleaseName(name string) {
	cc.m.Lock()
	defer cc.m.Unlock()

	if _, exist := cc.containersByName[name]; !exist {
		log.Errorf("ReleaseName error: Name %s not found", name)
		return
	}

	delete(cc.containersByName, name)
}
