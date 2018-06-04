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

package handlers

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/vic/lib/apiservers/portlayer/models"
	"github.com/vmware/vic/lib/apiservers/portlayer/restapi/operations/storage"
	"github.com/vmware/vic/lib/archive"
	"github.com/vmware/vic/lib/constants"
	spl "github.com/vmware/vic/lib/portlayer/storage"
	"github.com/vmware/vic/lib/portlayer/storage/image"
	"github.com/vmware/vic/lib/portlayer/storage/volume"
	"github.com/vmware/vic/lib/portlayer/util"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/vsphere/vm"
)

var (
	testImageID     = "testImage"
	testImageSum    = "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	testHostName, _ = os.Hostname()
	testStoreName   = "testStore"
	testStoreURL    = url.URL{
		Scheme: "http",
		Host:   testHostName,
		Path:   "/" + util.ImageURLPath + "/" + testStoreName,
	}
)

type MockDataStore struct {
}

type MockVolumeStore struct {
	// id -> volume
	db map[string]*volume.Volume
}

func NewMockVolumeStore() *MockVolumeStore {
	m := &MockVolumeStore{
		db: make(map[string]*volume.Volume),
	}

	return m
}

// Creates a volume on the given volume store, of the given size, with the given metadata.
func (m *MockVolumeStore) VolumeCreate(op trace.Operation, ID string, store *url.URL, capacityKB uint64, info map[string][]byte) (*volume.Volume, error) {
	storeName, err := util.VolumeStoreName(store)
	if err != nil {
		return nil, err
	}

	selfLink, err := util.VolumeURL(storeName, ID)
	if err != nil {
		return nil, err
	}

	vol := &volume.Volume{
		ID:       ID,
		Store:    store,
		SelfLink: selfLink,
	}

	m.db[ID] = vol

	return vol, nil
}

// Get an existing volume via it's ID and volume store.
func (m *MockVolumeStore) VolumeGet(op trace.Operation, ID string) (*volume.Volume, error) {
	vol, ok := m.db[ID]
	if !ok {
		return nil, os.ErrNotExist
	}

	return vol, nil
}

// Destroys a volume
func (m *MockVolumeStore) VolumeDestroy(op trace.Operation, vol *volume.Volume) error {
	if _, ok := m.db[vol.ID]; !ok {
		return os.ErrNotExist
	}

	delete(m.db, vol.ID)

	return nil
}

func (m *MockVolumeStore) VolumeStoresList(op trace.Operation) (map[string]url.URL, error) {
	return nil, fmt.Errorf("not implemented")
}

// Lists all volumes on the given volume store`
func (m *MockVolumeStore) VolumesList(op trace.Operation) ([]*volume.Volume, error) {
	var i int
	list := make([]*volume.Volume, len(m.db))
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

func (m *MockVolumeStore) NewDataSink(op trace.Operation, id string) (spl.DataSink, error) {
	return nil, nil
}

func (m *MockVolumeStore) NewDataSource(op trace.Operation, id string) (spl.DataSource, error) {
	return nil, nil
}

func (m *MockVolumeStore) URL(op trace.Operation, id string) (*url.URL, error) {
	return nil, nil
}

func (m *MockVolumeStore) Owners(op trace.Operation, url *url.URL, filter func(vm *mo.VirtualMachine) bool) ([]*vm.VirtualMachine, error) {
	return nil, nil
}

// GetImageStore checks to see if a named image store exists and returls the
// URL to it if so or error.
func (c *MockDataStore) GetImageStore(op trace.Operation, storeName string) (*url.URL, error) {
	_, err := util.ImageStoreNameToURL(storeName)
	if err != nil {
		return nil, err
	}
	return nil, os.ErrNotExist
}

func (c *MockDataStore) CreateImageStore(op trace.Operation, storeName string) (*url.URL, error) {
	u, err := util.ImageStoreNameToURL(storeName)
	if err != nil {
		return nil, err
	}

	return u, nil
}

func (c *MockDataStore) DeleteImageStore(op trace.Operation, storeName string) error {
	return nil
}

func (c *MockDataStore) ListImageStores(op trace.Operation) ([]*url.URL, error) {
	return nil, nil
}

func (c *MockDataStore) Export(op trace.Operation, child, ancestor string, spec *archive.FilterSpec, data bool) (io.ReadCloser, error) {
	return nil, nil
}

func (c *MockDataStore) Import(op trace.Operation, id string, spec *archive.FilterSpec, tarstream io.ReadCloser) error {
	return nil
}

func (c *MockDataStore) NewDataSink(op trace.Operation, id string) (spl.DataSink, error) {
	return nil, nil
}

func (c *MockDataStore) NewDataSource(op trace.Operation, id string) (spl.DataSource, error) {
	return nil, nil
}

func (c *MockDataStore) URL(op trace.Operation, id string) (*url.URL, error) {
	return nil, nil
}

func (c *MockDataStore) Owners(op trace.Operation, url *url.URL, filter func(vm *mo.VirtualMachine) bool) ([]*vm.VirtualMachine, error) {
	return nil, nil
}

func (c *MockDataStore) WriteImage(op trace.Operation, parent *image.Image, ID string, meta map[string][]byte, sum string, r io.Reader) (*image.Image, error) {
	storeName, err := util.ImageStoreName(parent.Store)
	if err != nil {
		return nil, err
	}

	selflink, err := util.ImageURL(storeName, ID)
	if err != nil {
		return nil, err
	}

	i := image.Image{
		ID:         ID,
		Store:      parent.Store,
		ParentLink: parent.SelfLink,
		SelfLink:   selflink,
		Metadata:   meta,
	}

	return &i, nil
}
func (c *MockDataStore) WriteMetadata(op trace.Operation, storeName string, ID string, meta map[string][]byte) error {
	return nil
}

// GetImage gets the specified image from the given store by retreiving it from the cache.
func (c *MockDataStore) GetImage(op trace.Operation, store *url.URL, ID string) (*image.Image, error) {
	if ID == constants.ScratchLayerID {
		return &image.Image{Store: store}, nil
	}

	return nil, os.ErrNotExist
}

// ListImages resturns a list of Images for a list of IDs, or all if no IDs are passed
func (c *MockDataStore) ListImages(op trace.Operation, store *url.URL, IDs []string) ([]*image.Image, error) {
	return nil, fmt.Errorf("store (%s) doesn't exist", store.String())
}

func (c *MockDataStore) DeleteImage(op trace.Operation, image *image.Image) (*image.Image, error) {
	return nil, nil
}

func TestCreateImageStore(t *testing.T) {
	s := &StorageHandlersImpl{
		imageCache: image.NewLookupCache(&MockDataStore{}),
	}

	store := &models.ImageStore{
		Name: "testStore",
	}

	params := storage.CreateImageStoreParams{
		Body: store,
	}

	result := s.CreateImageStore(params)
	if !assert.NotNil(t, result) {
		return
	}

	// try to recreate the same image store
	result = s.CreateImageStore(params)
	if !assert.NotNil(t, result) {
		return
	}
	// expect 409 since it already exists
	conflict := &storage.CreateImageStoreConflict{
		Payload: &models.Error{
			Code:    http.StatusConflict,
			Message: "An image store with that name already exists",
		},
	}
	if !assert.Equal(t, conflict, result) {
		return
	}
}

func TestGetImage(t *testing.T) {

	s := &StorageHandlersImpl{
		imageCache: image.NewLookupCache(&MockDataStore{}),
	}

	params := &storage.GetImageParams{
		ID:        testImageID,
		StoreName: testStoreName,
	}

	result := s.GetImage(*params)
	if !assert.NotNil(t, result) {
		return
	}

	op := trace.NewOperation(context.Background(), "test")

	// create the image store
	url, err := s.imageCache.CreateImageStore(op, testStoreName)
	// TODO(jzt): these are testing NameLookupCache, do we need them here?
	if !assert.Nil(t, err, "Error while creating image store") {
		return
	}
	if !assert.Equal(t, testStoreURL.String(), url.String()) {
		return
	}

	// try GetImage again
	result = s.GetImage(*params)
	if !assert.NotNil(t, result) {
		return
	}

	// add image to store
	parent := image.Image{
		ID:         "scratch",
		SelfLink:   nil,
		ParentLink: nil,
		Store:      &testStoreURL,
	}

	expectedMeta := make(map[string][]byte)
	expectedMeta["foo"] = []byte("bar")
	// add the image to the store
	image, err := s.imageCache.WriteImage(op, &parent, testImageID, expectedMeta, testImageSum, nil)
	if !assert.NoError(t, err) || !assert.NotNil(t, image) {
		return
	}

	selflink, err := util.ImageURL(testStoreName, testImageID)
	if !assert.NoError(t, err) {
		return
	}
	sl := selflink.String()

	parentlink, err := util.ImageURL(testStoreName, parent.ID)
	if !assert.NoError(t, err) {
		return
	}
	p := parentlink.String()

	eMeta := make(map[string]string)
	eMeta["foo"] = "bar"
	// expect our image back now that we've created it
	expected := &storage.GetImageOK{
		Payload: &models.Image{
			ID:       image.ID,
			SelfLink: sl,
			Parent:   p,
			Store:    testStoreURL.String(),
			Metadata: eMeta,
		},
	}

	result = s.GetImage(*params)
	if !assert.NotNil(t, result) {
		return
	}
	if !assert.Equal(t, expected, result) {
		return
	}
}

func TestListImages(t *testing.T) {

	s := &StorageHandlersImpl{
		imageCache: image.NewLookupCache(&MockDataStore{}),
	}

	params := &storage.ListImagesParams{
		StoreName: testStoreName,
	}

	outImages := s.ListImages(*params)
	if !assert.NotNil(t, outImages) {
		return
	}

	op := trace.NewOperation(context.Background(), "test")

	// create the image store
	url, err := s.imageCache.CreateImageStore(op, testStoreName)
	if !assert.NoError(t, err) {
		return
	}
	if !assert.NotNil(t, url) {
		return
	}

	// create a set of images
	images := make(map[string]*image.Image)
	parent := image.Image{
		ID: constants.ScratchLayerID,
	}
	parent.Store = &testStoreURL
	for i := 1; i < 50; i++ {
		id := fmt.Sprintf("id-%d", i)
		img, err := s.imageCache.WriteImage(op, &parent, id, nil, testImageSum, nil)
		if !assert.NoError(t, err) {
			return
		}
		if !assert.NotNil(t, img) {
			return
		}
		images[id] = img
	}

	// List all images
	outImages = s.ListImages(*params)
	assert.IsType(t, &storage.ListImagesOK{}, outImages)
	assert.Equal(t, len(outImages.(*storage.ListImagesOK).Payload), len(images))

	for _, img := range outImages.(*storage.ListImagesOK).Payload {
		_, ok := images[img.ID]
		if !assert.True(t, ok) {
			return
		}
	}

	// List specific images
	var ids []string

	// query for odd-numbered image ids
	for i := 1; i < 50; i += 2 {
		ids = append(ids, fmt.Sprintf("id-%d", i))
	}
	params.Ids = ids
	outImages = s.ListImages(*params)
	assert.IsType(t, &storage.ListImagesOK{}, outImages)
	assert.Equal(t, len(ids), len(outImages.(*storage.ListImagesOK).Payload))

	outmap := make(map[string]*models.Image)
	for _, image := range outImages.(*storage.ListImagesOK).Payload {
		outmap[image.ID] = image
	}

	// ensure no even-numbered image ids in our result
	for i := 2; i < 50; i += 2 {
		id := fmt.Sprintf("id-%d", i)
		_, ok := outmap[id]
		if !assert.False(t, ok) {
			return
		}
	}
}

func TestWriteImage(t *testing.T) {
	ic := image.NewLookupCache(&MockDataStore{})

	// create image store
	op := trace.NewOperation(context.Background(), "test")
	_, err := ic.CreateImageStore(op, testStoreName)
	if err != nil {
		return
	}

	s := &StorageHandlersImpl{
		imageCache: ic,
	}

	eMeta := make(map[string]string)
	eMeta["foo"] = "bar"

	name := new(string)
	val := new(string)
	*name = "foo"
	*val = eMeta["foo"]

	params := &storage.WriteImageParams{
		StoreName:   testStoreName,
		ImageID:     testImageID,
		ParentID:    "scratch",
		Sum:         testImageSum,
		Metadatakey: name,
		Metadataval: val,
		ImageFile:   nil,
	}

	parentlink, err := util.ImageURL(testStoreName, params.ParentID)
	if !assert.NoError(t, err) {
		return
	}
	p := parentlink.String()

	selflink, err := util.ImageURL(testStoreName, testImageID)
	if !assert.NoError(t, err) {
		return
	}
	sl := selflink.String()

	expected := &storage.WriteImageCreated{
		Payload: &models.Image{
			ID:       testImageID,
			Parent:   p,
			SelfLink: sl,
			Store:    testStoreURL.String(),
			Metadata: eMeta,
		},
	}

	result := s.WriteImage(*params)
	if !assert.NotNil(t, result) {
		return
	}
	if !assert.Equal(t, expected, result) {
		return
	}
}

func TestVolumeCreate(t *testing.T) {

	op := trace.NewOperation(context.Background(), "test")
	volCache := volume.NewVolumeLookupCache(op)

	testStore := NewMockVolumeStore()
	_, err := volCache.AddStore(op, "testStore", testStore)
	if !assert.NoError(t, err) {
		return
	}

	handler := StorageHandlersImpl{
		volumeCache: volCache,
	}

	model := models.VolumeRequest{}
	model.Store = "testStore"
	model.Name = "testVolume"
	model.Capacity = 1
	model.Driver = "vsphere"
	model.DriverArgs = make(map[string]string)
	model.DriverArgs["stuff"] = "things"
	model.Metadata = make(map[string]string)
	params := storage.NewCreateVolumeParams()
	params.VolumeRequest = &model

	handler.CreateVolume(params)
	testVolume, err := testStore.VolumeGet(op, model.Name)
	if !assert.NoError(t, err) {
		return
	}

	if !assert.NotNil(t, testVolume) {
		return
	}
	testVolumeStoreName, err := util.VolumeStoreName(testVolume.Store)
	if !assert.NoError(t, err) {
		return
	}
	if !assert.Equal(t, "testStore", testVolumeStoreName) {
		return
	}
	if !assert.Equal(t, "testVolume", testVolume.ID) {
		return
	}
}

func TestParseUIDAndGID(t *testing.T) {
	positiveCases := []url.URL{
		{
			Scheme:   "nfs",
			Host:     "testURL",
			RawQuery: "uid=1234&gid=5678",
			Path:     "/test/path",
		},
		{
			Scheme:   "nfs",
			Host:     "testURL",
			RawQuery: "uid=00000000000000&gid=00000000000000000000000000",
			Path:     "/test/path",
		},
		{
			Scheme:   "nfs",
			Host:     "testURL",
			RawQuery: "uid=321321321&gid=123123123",
			Path:     "/test/path",
		},
		{
			Scheme:   "nfs",
			Host:     "testURL",
			RawQuery: "uid=0&gid=0",
			Path:     "/test/path",
		},
		{
			Scheme:   "nfs",
			Host:     "testURL",
			RawQuery: "uid=&gid=",
			Path:     "/test/path",
		},
	}

	negativeCases := []url.URL{
		{
			Scheme:   "nfs",
			Host:     "testURL",
			RawQuery: "uid=Hello&gid=World",
			Path:     "/test/path",
		},
		{
			Scheme:   "nfs",
			Host:     "testURL",
			RawQuery: "uid=ASKJHG#!@#LJK$&gid=!@#$$%#@@!",
			Path:     "/test/path",
		},
		{
			Scheme:   "nfs",
			Host:     "testURL",
			RawQuery: "uid=9999999999999999999999999999999999999999999999999&gid=7777777777777777777777777777777777777777777777777777777",
			Path:     "/test/path",
		},
	}

	for _, v := range positiveCases {

		testUID, testGID, err := parseUIDAndGID(&v)
		assert.Nil(t, err, v.String())
		assert.NotEqual(t, -1, testUID, v.String())
		assert.NotEqual(t, -1, testGID, v.String())
	}

	for _, v := range negativeCases {
		testUID, testGID, err := parseUIDAndGID(&v)
		assert.NotNil(t, err, v.String())
		assert.Equal(t, -1, testUID, v.String())
		assert.Equal(t, -1, testGID, v.String())

	}

}
