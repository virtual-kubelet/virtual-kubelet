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

package exec

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/vmware/vic/pkg/uid"
	"github.com/vmware/vic/pkg/vsphere/vm"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"
)

func TestContainerCache(t *testing.T) {
	NewContainerCache()
	containerID := uid.New().String()

	// create a new container
	container := newTestContainer(containerID)

	// put it in the cache
	Containers.Put(container)
	// still shouldn't have a container because there's no vm
	assert.Equal(t, len(Containers.cache), 0)

	// add a test vm
	addTestVM(container)

	// put in cache
	Containers.Put(container)
	// get all containers -- should have 1
	assert.Equal(t, len(Containers.Containers(nil)), 1)
	// Get specific container
	cachedContainer := Containers.Container(containerID)
	// did we find it?
	assert.NotNil(t, cachedContainer)
	// do we have this one in the cache?
	assert.Equal(t, cachedContainer.ExecConfig.ID, containerID)
	// remove the container
	Containers.Remove(containerID)
	assert.Equal(t, len(Containers.cache), 0)
	// remove non-existent container
	Containers.Remove("blahblah")
}

func TestIsContainerID(t *testing.T) {
	validID := uid.New().String()
	invalidID := "ABC-XZ_@"

	assert.True(t, isContainerID(validID))
	assert.False(t, isContainerID(invalidID))
}

// addTestVM will add a pseudo VM to the container
func addTestVM(container *Container) {
	mo := types.ManagedObjectReference{Type: "vm", Value: "12"}
	v := object.NewVirtualMachine(nil, mo)
	container.vm = vm.NewVirtualMachineFromVM(nil, nil, v)
}

func newTestContainer(id string) *Container {
	h := TestHandle(id)
	return newContainer(&h.containerBase)
}
