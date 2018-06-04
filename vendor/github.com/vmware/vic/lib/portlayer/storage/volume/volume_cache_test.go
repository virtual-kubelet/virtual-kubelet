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

package volume

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"sync"
	"testing"

	"context"

	"github.com/stretchr/testify/assert"

	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/vic/lib/archive"
	"github.com/vmware/vic/lib/portlayer/exec"
	"github.com/vmware/vic/lib/portlayer/storage"
	"github.com/vmware/vic/lib/portlayer/util"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/vsphere/vm"
)

type MockVolumeStore struct {
	// id -> volume
	db map[string]*Volume
}

func NewMockVolumeStore() *MockVolumeStore {
	m := &MockVolumeStore{
		db: make(map[string]*Volume),
	}

	return m
}

func (m *MockVolumeStore) VolumeStoresList(op trace.Operation) (map[string]url.URL, error) {
	return nil, nil
}

// Creates a volume on the given volume store, of the given size, with the given metadata.
func (m *MockVolumeStore) VolumeCreate(op trace.Operation, ID string, store *url.URL, capacityKB uint64, info map[string][]byte) (*Volume, error) {
	storeName, err := util.VolumeStoreName(store)
	if err != nil {
		return nil, err
	}

	selfLink, err := util.VolumeURL(storeName, ID)
	if err != nil {
		return nil, err
	}

	vol := &Volume{
		ID:       ID,
		Store:    store,
		SelfLink: selfLink,
	}

	m.db[ID] = vol

	return vol, nil
}

// Get an existing volume via it's ID and volume store.
func (m *MockVolumeStore) VolumeGet(op trace.Operation, ID string) (*Volume, error) {
	vol, ok := m.db[ID]
	if !ok {
		return nil, os.ErrNotExist
	}

	return vol, nil
}

// Destroys a volume
func (m *MockVolumeStore) VolumeDestroy(op trace.Operation, vol *Volume) error {
	if _, ok := m.db[vol.ID]; !ok {
		return os.ErrNotExist
	}

	delete(m.db, vol.ID)

	return nil
}

// VolumesList lists all volumes on the given volume store.
func (m *MockVolumeStore) VolumesList(op trace.Operation) ([]*Volume, error) {
	var i int
	list := make([]*Volume, len(m.db))
	for _, v := range m.db {
		t := *v
		list[i] = &t
		i++
	}

	return list, nil
}

func (m *MockVolumeStore) Export(op trace.Operation, child, ancestor string, spec *archive.FilterSpec, data bool) (io.ReadCloser, error) {
	return nil, nil
}

func (m *MockVolumeStore) Import(op trace.Operation, id string, spec *archive.FilterSpec, tarstream io.ReadCloser) error {
	return nil
}

func (m *MockVolumeStore) NewDataSink(op trace.Operation, id string) (storage.DataSink, error) {
	return nil, nil
}

func (m *MockVolumeStore) NewDataSource(op trace.Operation, id string) (storage.DataSource, error) {
	return nil, nil
}

func (m *MockVolumeStore) URL(op trace.Operation, id string) (*url.URL, error) {
	return nil, nil
}

func (m *MockVolumeStore) Owners(op trace.Operation, url *url.URL, filter func(vm *mo.VirtualMachine) bool) ([]*vm.VirtualMachine, error) {
	return nil, nil
}

func TestVolumeCreateGetListAndDelete(t *testing.T) {
	op := trace.NewOperation(context.Background(), "test")

	exec.NewContainerCache()

	mvs := NewMockVolumeStore()
	v := NewVolumeLookupCache(op)
	storeURL, err := v.AddStore(op, "testStore", mvs)
	if !assert.NoError(t, err) {
		return
	}

	inVols := make(map[string]*Volume)
	inVolsM := &sync.Mutex{}

	wg := &sync.WaitGroup{}
	createFn := func(i int) {
		defer wg.Done()

		id := fmt.Sprintf("ID-%d", i)

		// Write to the datastore
		vol, err := v.VolumeCreate(op, id, storeURL, 0, nil)
		if !assert.NoError(t, err) || !assert.NotNil(t, vol) {
			return
		}

		inVolsM.Lock()
		inVols[id] = vol
		inVolsM.Unlock()
	}

	// Create a set of volumes
	numVolumes := 5
	wg.Add(numVolumes)
	for i := 0; i < numVolumes; i++ {
		go createFn(i)
	}
	wg.Wait()

	getFn := func(inVol *Volume) {
		vol, err := v.VolumeGet(op, inVol.ID)
		if !assert.NoError(t, err) || !assert.NotNil(t, vol) {
			return
		}

		if !assert.Equal(t, inVol, vol) {
			return
		}
		wg.Done()
	}

	wg.Add(numVolumes)
	for _, inVol := range inVols {
		getFn(inVol)
	}
	wg.Wait()

	volumeList, err := v.VolumesList(op)
	if !assert.NoError(t, err) || !assert.Equal(t, numVolumes, len(volumeList)) {
		return
	}

	// Test that the list returned by VolumeList matches our inVols list.  Then
	// delete each vol via the cache, then check the datastore to ensure it's
	// empty
	for _, outVol := range volumeList {
		if !assert.Equal(t, inVols[outVol.ID], outVol) {
			return
		}

		if err = v.VolumeDestroy(op, outVol.ID); !assert.NoError(t, err) {
			return
		}
	}

	// check the datastore is empty.
	if !assert.Empty(t, mvs.db) {
		return
	}
}

// createVolumes is a test helper that creates a set of num volumes on the input volume cache and volume store.
func createVolumes(t *testing.T, op trace.Operation, v *VolumeLookupCache, storeURL *url.URL, num int) map[string]*Volume {
	vols := make(map[string]*Volume)
	for i := 1; i <= num; i++ {
		id := fmt.Sprintf("ID-%d", i)

		// Write to the datastore
		vol, err := v.VolumeCreate(op, id, storeURL, 0, nil)
		if !assert.NoError(t, err) || !assert.NotNil(t, vol) {
			return nil
		}

		vols[id] = vol
	}

	return vols
}

func TestAddVolumesToCache(t *testing.T) {
	mvs1 := NewMockVolumeStore()
	op := trace.NewOperation(context.Background(), "test")
	v := NewVolumeLookupCache(op)

	storeURL, err := util.VolumeStoreNameToURL("testStore")
	assert.NotNil(t, storeURL)
	storeURLStr := storeURL.String()
	v.volumeStores[storeURLStr] = mvs1

	// Create 50 volumes on the volume store.
	vols := createVolumes(t, op, v, storeURL, 50)

	// Clear the volume map after it has been filled during volume creation.
	v.vlc = make(map[string]Volume)

	err = v.addVolumesToCache(op, storeURLStr, mvs1)
	assert.Nil(t, err)

	// Check that the volume map is intact again in the cache.
	for _, expectedVol := range vols {
		vol, err := v.VolumeGet(op, expectedVol.ID)
		if !assert.NoError(t, err) || !assert.Equal(t, expectedVol, vol) {
			return
		}
	}
}

// Create 2 store caches but use the same backing datastore.  Create images
// with the first cache, then get the image with the second.  This simulates
// restart since the second cache is empty and has to go to the backing store.
func TestVolumeCacheRestart(t *testing.T) {
	mvs := NewMockVolumeStore()
	op := trace.NewOperation(context.Background(), "test")

	firstCache := NewVolumeLookupCache(op)
	storeURL, err := firstCache.AddStore(op, "testStore", mvs)
	if !assert.NoError(t, err) || !assert.NotNil(t, storeURL) {
		return
	}

	// Create a set of 50 volumes.
	inVols := createVolumes(t, op, firstCache, storeURL, 50)

	secondCache := NewVolumeLookupCache(op)
	if !assert.NotNil(t, secondCache) {
		return
	}

	storeURL, err = secondCache.AddStore(op, "testStore", mvs)
	if !assert.NoError(t, err) || !assert.NotNil(t, storeURL) {
		return
	}

	// get the vols from the second cache to ensure it goes to the ds
	for _, expectedVol := range inVols {
		vol, err := secondCache.VolumeGet(op, expectedVol.ID)
		if !assert.NoError(t, err) || !assert.Equal(t, expectedVol, vol) {
			return
		}
	}
}
