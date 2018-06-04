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

package vsphere

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/vic/lib/portlayer/storage/volume"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/vsphere/datastore"
	"github.com/vmware/vic/pkg/vsphere/tasks"
)

func TestVolumeCreateListAndRestart(t *testing.T) {
	client := datastore.Session(context.TODO(), t)
	if client == nil {
		return
	}

	op := trace.NewOperation(context.Background(), "test")

	// Root our datastore
	testStorePath := datastore.TestName("voltest")
	ds, err := datastore.NewHelper(op, client, client.Datastore, testStorePath)
	if !assert.NoError(t, err) || !assert.NotNil(t, ds) {
		return
	}

	// Create the backing store on vsphere
	DetachAll = false
	vsVolumeStore, err := NewVolumeStore(op, "testStoreName", client, ds)
	if !assert.NoError(t, err) || !assert.NotNil(t, vsVolumeStore) {
		return
	}

	// Clean up the mess
	defer func() {
		fm := object.NewFileManager(client.Vim25())
		tasks.WaitForResult(context.TODO(), func(ctx context.Context) (tasks.Task, error) {
			return fm.DeleteDatastoreFile(ctx, client.Datastore.Path(testStorePath), client.Datacenter)
		})
	}()

	// Create the cache
	cache := volume.NewVolumeLookupCache(op)
	if !assert.NotNil(t, cache) {
		return
	}

	// add the vs to the cache and assert the url matches
	storeURL, err := cache.AddStore(op, "testStoreName", vsVolumeStore)
	if !assert.NoError(t, err) || !assert.Equal(t, vsVolumeStore.SelfLink, storeURL) {
		return
	}

	// test we can list it
	m, err := cache.VolumeStoresList(op)
	if !assert.NoError(t, err) || !assert.Len(t, m, 1) || !assert.Equal(t, m[0], "testStoreName") {
		return
	}

	// Create the volumes (in parallel)
	numVols := 5
	wg := &sync.WaitGroup{}
	wg.Add(numVols)
	volumes := make(map[string]*volume.Volume)
	for i := 0; i < numVols; i++ {
		go func(idx int) {
			defer wg.Done()
			ID := fmt.Sprintf("testvolume-%d", idx)

			// add some metadata if i is even
			var info map[string][]byte

			if idx%2 == 0 {
				info = make(map[string][]byte)
				info[ID] = []byte(ID)
			}

			outVol, err := cache.VolumeCreate(op, ID, storeURL, 10240, info)
			if !assert.NoError(t, err) || !assert.NotNil(t, outVol) {
				return
			}

			volumes[ID] = outVol
		}(i)
	}

	wg.Wait()

	// list using the datastore (skipping the cache)
	outVols, err := vsVolumeStore.VolumesList(op)
	if !assert.NoError(t, err) || !assert.NotNil(t, outVols) || !assert.Equal(t, numVols, len(outVols)) {
		return
	}

	for _, outVol := range outVols {
		if !assert.Equal(t, volumes[outVol.ID], outVol) {
			return
		}
	}

	// Test restart

	// Create a new vs and cache to the same datastore (simulating restart) and compare
	secondVStore, err := NewVolumeStore(op, "testStoreName", client, ds)
	if !assert.NoError(t, err) || !assert.NotNil(t, vsVolumeStore) {
		return
	}

	secondCache := volume.NewVolumeLookupCache(op)
	if !assert.NotNil(t, secondCache) {
		return
	}

	_, err = secondCache.AddStore(op, "testStore", secondVStore)
	if !assert.NoError(t, err) {
		return
	}

	secondOutVols, err := secondCache.VolumesList(op)
	if !assert.NoError(t, err) || !assert.NotNil(t, secondOutVols) || !assert.Equal(t, numVols, len(secondOutVols)) {
		return
	}

	for _, outVol := range secondOutVols {
		// XXX we could compare the Volumes, but the paths are different the
		// second time around on vsan since the vsan UUID is not included.
		if !assert.NotEmpty(t, volumes[outVol.ID].Device.DiskPath()) {
			return
		}
	}
}
