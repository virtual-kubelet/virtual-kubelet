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
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"testing"

	"github.com/Sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/vic/lib/constants"
	"github.com/vmware/vic/lib/portlayer/exec"
	"github.com/vmware/vic/lib/portlayer/storage/image"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/vsphere/datastore"
	"github.com/vmware/vic/pkg/vsphere/datastore/test"
	"github.com/vmware/vic/pkg/vsphere/disk"
	"github.com/vmware/vic/pkg/vsphere/session"
)

func setup(t *testing.T) (*image.NameLookupCache, *session.Session, string, error) {
	logrus.SetLevel(logrus.DebugLevel)
	trace.Logger.Level = logrus.DebugLevel
	DetachAll = false

	client := test.Session(context.TODO(), t)
	if client == nil {
		return nil, nil, "", fmt.Errorf("skip")
	}

	storeURL := &url.URL{
		Path: datastore.TestName("imageTests"),
		Host: client.DatastorePath}

	op := trace.NewOperation(context.Background(), "setup")
	vsImageStore, err := NewImageStore(op, client, storeURL)
	if err != nil {
		if err.Error() == "can't find the hosting vm" {
			t.Skip("Skipping: test must be run in a VM")
		} else {
			t.Log(err.Error())
		}
		return nil, nil, "", err
	}

	s := image.NewLookupCache(vsImageStore)

	return s, client, storeURL.Path, nil
}

func TestRestartImageStore(t *testing.T) {
	t.Skip("this test needs TLC")

	// Start the image store once
	cacheStore, client, parentPath, err := setup(t)
	if !assert.NoError(t, err) {
		return
	}

	origVsStore := cacheStore.DataStore.(*ImageStore)
	defer cleanup(t, client, origVsStore, parentPath)

	storeName := "bogusStoreName"
	op := trace.NewOperation(context.Background(), "test")
	origStore, err := cacheStore.CreateImageStore(op, storeName)
	if !assert.NoError(t, err) || !assert.NotNil(t, origStore) {
		return
	}

	imageStoreURL := &url.URL{
		Path: constants.StorageParentDir,
		Host: client.DatastorePath}

	// now start it again
	restartedVsStore, err := NewImageStore(op, client, imageStoreURL)
	if !assert.NoError(t, err) || !assert.NotNil(t, restartedVsStore) {
		return
	}

	// Check we didn't create a new UUID directory (relevant if vsan)
	if !assert.Equal(t, origVsStore.RootURL, restartedVsStore.RootURL) {
		return
	}

	restartedStore, err := restartedVsStore.GetImageStore(op, storeName)
	if !assert.NoError(t, err) || !assert.NotNil(t, restartedStore) {
		return
	}

	if !assert.Equal(t, origStore.String(), restartedStore.String()) {
		return
	}
}

// Create an image store then test it exists
func TestCreateAndGetImageStore(t *testing.T) {
	vsis, client, parentPath, err := setup(t)
	if !assert.NoError(t, err) {
		return
	}

	// Nuke the parent image store directory
	defer rm(t, client, client.Datastore.Path(parentPath))

	storeName := "bogusStoreName"
	op := trace.NewOperation(context.Background(), "test")
	u, err := vsis.CreateImageStore(op, storeName)
	if !assert.NoError(t, err) || !assert.NotNil(t, u) {
		return
	}

	u, err = vsis.GetImageStore(op, storeName)
	if !assert.NoError(t, err) || !assert.NotNil(t, u) {
		return
	}

	// Negative test.  Check for a dir that doesn't exist
	u, err = vsis.GetImageStore(op, storeName+"garbage")
	if !assert.Error(t, err) || !assert.Nil(t, u) {
		return
	}

	// Test for a store that already exists
	u, err = vsis.CreateImageStore(op, storeName)
	if !assert.Error(t, err) || !assert.Nil(t, u) || !assert.Equal(t, err, os.ErrExist) {
		return
	}
}

func TestListImageStore(t *testing.T) {
	vsis, client, parentPath, err := setup(t)
	if !assert.NoError(t, err) {
		return
	}

	// Nuke the parent image store directory
	defer rm(t, client, client.Datastore.Path(parentPath))

	op := trace.NewOperation(context.Background(), "test")

	count := 3
	for i := 0; i < count; i++ {
		storeName := fmt.Sprintf("storeName%d", i)
		u, err := vsis.CreateImageStore(op, storeName)
		if !assert.NoError(t, err) || !assert.NotNil(t, u) {
			return
		}
	}

	images, err := vsis.ListImageStores(op)
	if !assert.NoError(t, err) || !assert.Equal(t, len(images), count) {
		return
	}
}

// Creates a tar archive in memory for each layer and uses this to test image creation of layers
func TestCreateImageLayers(t *testing.T) {
	numLayers := 4

	cacheStore, client, parentPath, err := setup(t)
	if !assert.NoError(t, err) {
		return
	}

	vsStore := cacheStore.DataStore.(*ImageStore)
	defer cleanup(t, client, vsStore, parentPath)

	op := trace.NewOperation(context.Background(), "test")

	storeURL, err := cacheStore.CreateImageStore(op, "testStore")
	if !assert.NoError(t, err) {
		return
	}

	// Get an image that doesn't exist and check for error
	grbg, err := cacheStore.GetImage(op, storeURL, "garbage")
	if !assert.Error(t, err) || !assert.Nil(t, grbg) {
		return
	}

	// base this image off scratch
	parent, err := cacheStore.GetImage(op, storeURL, constants.ScratchLayerID)
	if !assert.NoError(t, err) {
		return
	}

	// Keep a list of all files we're extracting via layers so we can verify
	// they exist in the leaf layer.  Ext adds lost+found, so add it here.
	expectedFilesOnDisk := []string{"lost+found"}

	// Keep a list of images we created
	expectedImages := make(map[string]*image.Image)
	expectedImages[parent.ID] = parent

	for layer := 0; layer < numLayers; layer++ {

		dirName := fmt.Sprintf("dir%d", layer)
		// Add some files to the archive.
		var files = []tarFile{
			{dirName, tar.TypeDir, ""},
			{dirName + "/readme.txt", tar.TypeReg, "This archive contains some text files."},
			{dirName + "/gopher.txt", tar.TypeReg, "Gopher names:\nGeorge\nGeoffrey\nGonzo"},
			{dirName + "/todo.txt", tar.TypeReg, "Get animal handling license."},
		}

		for _, i := range files {
			expectedFilesOnDisk = append(expectedFilesOnDisk, i.Name)
		}

		// meta for the image
		meta := make(map[string][]byte)
		meta[dirName+"_meta"] = []byte("Some Meta")
		meta[dirName+"_moreMeta"] = []byte("Some More Meta")
		meta[dirName+"_scorpions"] = []byte("Here I am, rock you like a hurricane")

		// Tar the files
		buf, terr := tarFiles(files)
		if !assert.NoError(t, terr) {
			return
		}

		// Calculate the checksum
		h := sha256.New()
		h.Write(buf.Bytes())
		sum := fmt.Sprintf("sha256:%x", h.Sum(nil))

		// Write the image via the cache (which writes to the vsphere impl)
		writtenImage, terr := cacheStore.WriteImage(op, parent, dirName, meta, sum, buf)
		if !assert.NoError(t, terr) || !assert.NotNil(t, writtenImage) {
			return
		}

		expectedImages[dirName] = writtenImage

		// Get the image directly via the vsphere image store impl.
		vsImage, terr := vsStore.GetImage(op, parent.Store, dirName)
		if !assert.NoError(t, terr) || !assert.NotNil(t, vsImage) {
			return
		}

		assert.Equal(t, writtenImage, vsImage)

		// make the next image a child of the one we just created
		parent = writtenImage
	}

	// Test list images on the datastore
	listedImages, err := vsStore.ListImages(op, parent.Store, nil)
	if !assert.NoError(t, err) || !assert.NotNil(t, listedImages) {
		return
	}
	for _, img := range listedImages {
		if !assert.Equal(t, expectedImages[img.ID].Store.String(), img.Store.String()) {
			return
		}
		if !assert.Equal(t, expectedImages[img.ID].SelfLink.String(), img.SelfLink.String()) {
			return
		}
	}

	// verify the disk's data by attaching the last layer rdonly
	roDisk, err := mountLayerRO(vsStore, parent)
	if !assert.NoError(t, err) {
		return
	}

	p, err := roDisk.MountPath()
	if !assert.NoError(t, err) {
		return
	}

	rodiskcleanupfunc := func() {
		if roDisk != nil {
			if roDisk.Mounted() {
				roDisk.Unmount(op)
			}
			if roDisk.Attached() {
				vsStore.Detach(op, roDisk.VirtualDiskConfig)
			}
		}
		os.RemoveAll(p)
	}

	filesFoundOnDisk := []string{}
	// Diff the contents of the RO file of the last child (with all of the contents)
	err = filepath.Walk(p, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		f := path[len(p):]
		if f != "" {
			// strip the slash
			filesFoundOnDisk = append(filesFoundOnDisk, f[1:])
		}
		return nil
	})
	if !assert.NoError(t, err) {
		return
	}

	rodiskcleanupfunc()
	sort.Strings(filesFoundOnDisk)
	sort.Strings(expectedFilesOnDisk)

	if !assert.Equal(t, expectedFilesOnDisk, filesFoundOnDisk) {
		return
	}

	// Try to delete an intermediate image (should fail)
	exec.NewContainerCache()
	_, err = cacheStore.DeleteImage(op, expectedImages["dir1"])
	if !assert.Error(t, err) || !assert.True(t, image.IsErrImageInUse(err)) {
		return
	}

	// Try to delete a leaf (should pass)
	leaf := expectedImages["dir"+strconv.Itoa(numLayers-1)]
	_, err = cacheStore.DeleteImage(op, leaf)
	if !assert.NoError(t, err) {
		return
	}

	// Get the delete image directly via the vsphere image store impl.
	deletedImage, err := vsStore.GetImage(op, parent.Store, leaf.ID)
	if !assert.Error(t, err) || !assert.Nil(t, deletedImage) || !assert.True(t, os.IsNotExist(err)) {
		return
	}
}

func TestBrokenPull(t *testing.T) {

	cacheStore, client, parentPath, err := setup(t)
	if !assert.NoError(t, err) {
		return
	}

	vsStore := cacheStore.DataStore.(*ImageStore)

	defer cleanup(t, client, vsStore, parentPath)

	op := trace.NewOperation(context.Background(), "test")

	storeURL, err := cacheStore.CreateImageStore(op, "testStore")
	if !assert.NoError(t, err) {
		return
	}

	// base this image off scratch
	parent, err := cacheStore.GetImage(op, storeURL, constants.ScratchLayerID)
	if !assert.NoError(t, err) {
		return
	}

	imageID := "dir0"

	// Add some files to the archive.
	var files = []tarFile{
		{imageID, tar.TypeDir, ""},
		{imageID + "/readme.txt", tar.TypeReg, "This archive contains some text files."},
		{imageID + "/gopher.txt", tar.TypeReg, "Gopher names:\nGeorge\nGeoffrey\nGonzo"},
		{imageID + "/todo.txt", tar.TypeReg, "Get animal handling license."},
	}

	// meta for the image
	meta := make(map[string][]byte)
	meta[imageID+"_meta"] = []byte("Some Meta")
	meta[imageID+"_moreMeta"] = []byte("Some More Meta")
	meta[imageID+"_scorpions"] = []byte("Here I am, rock you like a hurricane")

	// Tar the files
	buf, terr := tarFiles(files)
	if !assert.NoError(t, terr) {
		return
	}

	// Calculate the checksum
	h := sha256.New()
	h.Write(buf.Bytes())
	actualsum := fmt.Sprintf("sha256:%x", h.Sum(nil))

	// Write the image via the cache (which writes to the vsphere impl).  We're passing a bogus sum so the image should fail to save.
	writtenImage, err := cacheStore.WriteImage(op, parent, imageID, meta, "bogusSum", new(bytes.Buffer))
	if !assert.Error(t, err) || !assert.Nil(t, writtenImage) {
		return
	}

	// Now try again with the right sum and there shouldn't be an error.
	writtenImage, err = cacheStore.WriteImage(op, parent, imageID, meta, actualsum, buf)
	if !assert.NoError(t, err) || !assert.NotNil(t, writtenImage) {
		return
	}
}

// Creates numLayers layers in parallel using the same parent to exercise parallel reconfigures
func TestParallel(t *testing.T) {
	numLayers := 10

	cacheStore, client, parentPath, err := setup(t)
	if !assert.NoError(t, err) {
		return
	}

	vsStore := cacheStore.DataStore.(*ImageStore)
	defer cleanup(t, client, vsStore, parentPath)

	op := trace.NewOperation(context.Background(), "test")
	storeURL, err := cacheStore.CreateImageStore(op, "testStore")
	if !assert.NoError(t, err) {
		return
	}

	// base this image off scratch
	parent, err := cacheStore.GetImage(op, storeURL, constants.ScratchLayerID)
	if !assert.NoError(t, err) {
		return
	}

	wg := sync.WaitGroup{}
	wg.Add(numLayers)
	for i := 0; i < numLayers; i++ {
		go func(idx int) {
			defer wg.Done()

			imageID := fmt.Sprintf("testStore-%d", idx)

			op := trace.NewOperation(context.Background(), imageID)
			// Write the image via the cache (which writes to the vsphere impl).  We're passing a bogus sum so the image should fail to save.
			writtenImage, err := cacheStore.WriteImage(op, parent, imageID, nil, "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", new(bytes.Buffer))
			if !assert.NoError(t, err) || !assert.NotNil(t, writtenImage) {
				t.FailNow()
				return
			}
		}(i)
	}
	wg.Wait()
}

func TestInProgressCleanup(t *testing.T) {

	cacheStore, client, parentPath, err := setup(t)
	if !assert.NoError(t, err) {
		return
	}

	vsStore := cacheStore.DataStore.(*ImageStore)

	defer cleanup(t, client, vsStore, parentPath)

	op := trace.NewOperation(context.Background(), "test")

	storeURL, err := cacheStore.CreateImageStore(op, "testStore")
	if !assert.NoError(t, err) {
		return
	}

	// base this image off scratch
	parent, err := cacheStore.GetImage(op, storeURL, constants.ScratchLayerID)
	if !assert.NoError(t, err) {
		return
	}

	// create a test image
	imageID := "testImage"

	// meta for the image
	meta := make(map[string][]byte)
	meta[imageID+"_meta"] = []byte("Some Meta")

	// Tar the files
	buf, err := tarFiles([]tarFile{})
	if !assert.NoError(t, err) {
		return
	}

	// Calculate the checksum
	h := sha256.New()
	h.Write(buf.Bytes())
	sum := fmt.Sprintf("sha256:%x", h.Sum(nil))

	writtenImage, err := cacheStore.WriteImage(op, parent, imageID, meta, sum, buf)
	if !assert.NoError(t, err) || !assert.NotNil(t, writtenImage) {
		return
	}

	// nuke the done file.
	rm(t, client, path.Join(vsStore.RootURL.String(), vsStore.imageDirPath("testStore", imageID), manifest))

	// ensure GetImage doesn't find this image now
	if _, err = vsStore.GetImage(op, storeURL, imageID); !assert.Error(t, err) {
		return
	}

	// call cleanup
	if err = vsStore.cleanup(op, storeURL); !assert.NoError(t, err) {
		return
	}

	// Make sure list is now empty.
	listedImages, err := vsStore.ListImages(op, parent.Store, nil)
	if !assert.NoError(t, err) || !assert.Equal(t, len(listedImages), 1) || !assert.Equal(t, listedImages[0].ID, constants.ScratchLayerID) {
		return
	}
}

type tarFile struct {
	Name string
	Type byte
	Body string
}

func tarFiles(files []tarFile) (*bytes.Buffer, error) {
	// Create a buffer to write our archive to.
	buf := new(bytes.Buffer)

	// Create a new tar archive.
	tw := tar.NewWriter(buf)

	// Write data to the tar as if it came from the hub
	for _, file := range files {
		hdr := &tar.Header{
			Name:     file.Name,
			Mode:     0777,
			Typeflag: file.Type,
			Size:     int64(len(file.Body)),
		}

		if err := tw.WriteHeader(hdr); err != nil {
			return nil, err
		}

		if file.Type == tar.TypeDir {
			continue
		}

		if _, err := tw.Write([]byte(file.Body)); err != nil {
			return nil, err
		}
	}

	// Make sure to check the error on Close.
	if err := tw.Close(); err != nil {
		return nil, err
	}

	return buf, nil
}

func mountLayerRO(v *ImageStore, parent *image.Image) (*disk.VirtualDisk, error) {
	roName := v.imageDiskDSPath("testStore", parent.ID)
	roName.Path = roName.Path + "-ro.vmdk"

	parentDsURI := v.imageDiskDSPath("testStore", parent.ID)

	op := trace.NewOperation(context.TODO(), "ro")

	config := disk.NewNonPersistentDisk(roName).WithParent(parentDsURI)
	roDisk, err := v.CreateAndAttach(op, config)
	if err != nil {
		return nil, err
	}

	_, err = roDisk.Mount(op, nil)
	if err != nil {
		return nil, err
	}

	return roDisk, nil
}

func rm(t *testing.T, client *session.Session, name string) {
	t.Logf("deleting %s", name)
	fm := object.NewFileManager(client.Vim25())
	task, err := fm.DeleteDatastoreFile(context.TODO(), name, client.Datacenter)
	if !assert.NoError(t, err) {
		return
	}
	_, _ = task.WaitForResult(context.TODO(), nil)
}

// Nuke the files and then the parent dir.  Unfortunately, because this is
// vsan, we need to delete the files in the directories first (maybe
// because they're linked vmkds) before we can delete the parent directory.
func cleanup(t *testing.T, client *session.Session, vsStore *ImageStore, parentPath string) {
	res, err := vsStore.LsDirs(context.TODO(), "")
	if err != nil {
		t.Logf("error: %s", err)
		return
	}

	for _, dir := range res.HostDatastoreBrowserSearchResults {
		for _, f := range dir.File {
			fpath := f.GetFileInfo().Path

			rm(t, client, path.Join(dir.FolderPath, fpath))
		}
		rm(t, client, dir.FolderPath)
	}

	rm(t, client, client.Datastore.Path(parentPath))
}
