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

package image

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/vic/lib/archive"
	"github.com/vmware/vic/lib/constants"
	"github.com/vmware/vic/lib/portlayer/storage"
	"github.com/vmware/vic/lib/portlayer/util"
	"github.com/vmware/vic/pkg/index"
	"github.com/vmware/vic/pkg/retry"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/vsphere/tasks"
	"github.com/vmware/vic/pkg/vsphere/vm"
)

var ErrCorruptImageStore = errors.New("Corrupt image store")

// NameLookupCache the global view of all of the image stores.  To avoid unnecessary
// lookups, the image cache keeps an in memory map of the store URI to the map
// of images on disk.
type NameLookupCache struct {

	// The individual store locations -> Index
	storeCache map[url.URL]*index.Index
	// Guard against concurrent writes to the storeCache map
	storeCacheLock sync.Mutex

	// The image store implementation.  This mutates the actual disk images.
	DataStore ImageStorer
}

func NewLookupCache(ds ImageStorer) *NameLookupCache {
	return &NameLookupCache{
		DataStore:  ds,
		storeCache: make(map[url.URL]*index.Index),
	}
}

// isRetry will check the error for retryability - if so reset the cache
func (c *NameLookupCache) isRetry(op trace.Operation, err error) bool {
	if tasks.IsRetryError(op, err) {
		op.Debugf("%s is retryable, resetting store cache", err)
		c.storeCache = make(map[url.URL]*index.Index)
		return true
	}
	return false
}

// GetImageStore checks to see if a named image store exists and returns the
// URL to it if so or error.
func (c *NameLookupCache) GetImageStore(op trace.Operation, storeName string) (*url.URL, error) {
	defer trace.End(trace.Begin(fmt.Sprintf("StoreName: %s", storeName), op))
	store, err := util.ImageStoreNameToURL(storeName)
	if err != nil {
		return nil, err
	}

	c.storeCacheLock.Lock()
	defer c.storeCacheLock.Unlock()

	// check the cache
	_, ok := c.storeCache[*store]

	if !ok {
		op.Infof("Refreshing image cache from datastore.")
		// Store isn't in the cache.  Look it up in the datastore.
		storeName, err := util.ImageStoreName(store)
		if err != nil {
			return nil, err
		}

		// If the store doesn't exist, we'll fall out here.
		_, err = c.DataStore.GetImageStore(op, storeName)
		if err != nil {
			return nil, err
		}

		indx := index.NewIndex()

		c.storeCache[*store] = indx

		// Add Scratch
		scratch, err := c.DataStore.GetImage(op, store, constants.ScratchLayerID)
		if err != nil {
			op.Errorf("ImageCache Error: looking up scratch on %s: %s", store.String(), err)
			if c.isRetry(op, err) {
				return nil, err
			}
			// potentially a recoverable error
			return nil, ErrCorruptImageStore
		}

		if err = indx.Insert(scratch); err != nil {
			return nil, err
		}

		// XXX after creating the indx and populating the map, we can put the rest in a go routine

		images, err := c.DataStore.ListImages(op, store, nil)
		if err != nil {
			// if error is retryable we'll reset the cache
			c.isRetry(op, err)
			return nil, err
		}

		op.Debugf("Found %d images", len(images))

		// Build image map to simplify tree traversal.
		imageMap := make(map[string]*Image, len(images))
		for _, img := range images {
			if img.ID == constants.ScratchLayerID {
				continue
			}
			imageMap[img.Self()] = img
		}

		for k := range imageMap {
			parentTree(op, k, indx, imageMap)
		}
	}

	return store, nil
}

// parentTree adds images into the cache starting from the parent.
func parentTree(op trace.Operation, imgLink string, idx *index.Index, imageMap map[string]*Image) {
	img, ok := imageMap[imgLink]
	if !ok {
		return
	}

	if img.Parent() != img.Self() {
		op.Debugf("Looking for parent %s for %s", img.Parent(), img.Self())
		parentTree(op, img.Parent(), idx, imageMap)
	}

	if err := idx.Insert(img); err != nil {
		op.Errorf("Could not insert image %s: %v", imgLink, err)
	} else {
		op.Infof("Added image %s on datastore.", imgLink)
	}

	delete(imageMap, imgLink)
}

func (c *NameLookupCache) CreateImageStore(op trace.Operation, storeName string) (*url.URL, error) {
	store, err := util.ImageStoreNameToURL(storeName)
	if err != nil {
		return nil, err
	}

	// GetImageStore Operation is able to be retried...
	getStore := func() error {
		// Check for existence and rehydrate the cache if it exists on disk.
		_, err = c.GetImageStore(op, storeName)
		return err
	}
	// is the error retryable
	isRetry := func(err error) bool {
		return tasks.IsRetryError(op, err)
	}

	config := retry.NewBackoffConfig()
	config.InitialInterval = time.Second * 15
	config.MaxInterval = time.Second * 30
	config.MaxElapsedTime = time.Minute * 3

	// attempt to get the image store
	err = retry.DoWithConfig(getStore, isRetry, config)
	if err == nil {
		// no error means that the image store exists and we can
		// safely return
		return nil, os.ErrExist
	}
	// if the image store doesn't exist or is corrupt we will continue,
	// otherwise fail here
	if err != os.ErrNotExist && err != ErrCorruptImageStore {
		op.Errorf("Error getting image store %s: %s", storeName, err)
		return nil, err
	}

	c.storeCacheLock.Lock()
	defer c.storeCacheLock.Unlock()

	store, err = c.DataStore.CreateImageStore(op, storeName)
	if err != nil {
		return nil, err
	}

	// Create the root image
	scratch, err := c.DataStore.WriteImage(op, &Image{Store: store}, constants.ScratchLayerID, nil, "", nil)
	if err != nil {
		// if we failed here, remove the image store
		op.Infof("Removing failed image store %s", storeName)
		if e := c.DataStore.DeleteImageStore(op, storeName); e != nil {
			op.Errorf("image store cleanup failed: %s", e.Error())
		}

		return nil, err
	}

	indx := index.NewIndex()
	c.storeCache[*store] = indx
	if err = indx.Insert(scratch); err != nil {
		return nil, err
	}

	return store, nil
}

// ListImageStores returns a list of strings representing all existing image stores
func (c *NameLookupCache) ListImageStores(op trace.Operation) ([]*url.URL, error) {
	c.storeCacheLock.Lock()
	defer c.storeCacheLock.Unlock()

	stores := make([]*url.URL, 0, len(c.storeCache))
	for key := range c.storeCache {
		stores = append(stores, &key)
	}
	return stores, nil
}

func (c *NameLookupCache) WriteImage(op trace.Operation, parent *Image, ID string, meta map[string][]byte, sum string, r io.Reader) (*Image, error) {
	// Check the parent exists (at least in the cache).
	p, err := c.GetImage(op, parent.Store, parent.ID)
	if err != nil {
		return nil, fmt.Errorf("parent (%s) doesn't exist in %s: %s", parent.ID, parent.Store.String(), err)
	}

	// Check the image doesn't already exist in the cache.  A miss in this will trigger a datastore lookup.
	i, err := c.GetImage(op, p.Store, ID)
	if err == nil && i != nil {
		// TODO(FA) check sums to make sure this is the right image

		return i, nil
	}

	// Definitely not in cache or image store, create image.
	i, err = c.DataStore.WriteImage(op, p, ID, meta, sum, r)
	if err != nil {
		op.Errorf("WriteImage of %s failed with: %s", ID, err)
		return nil, err
	}

	c.storeCacheLock.Lock()
	indx := c.storeCache[*parent.Store]
	c.storeCacheLock.Unlock()

	// Add the new image to the cache
	if err = indx.Insert(i); err != nil {
		return nil, err
	}

	return i, nil
}

func (c *NameLookupCache) Export(op trace.Operation, store *url.URL, id, ancestor string, spec *archive.FilterSpec, data bool) (io.ReadCloser, error) {
	return c.DataStore.Export(op, id, ancestor, spec, data)
}

func (c *NameLookupCache) Import(op trace.Operation, store *url.URL, diskID string, spec *archive.FilterSpec, tarStream io.ReadCloser) error {
	return c.DataStore.Import(op, diskID, spec, tarStream)
}

func (c *NameLookupCache) NewDataSource(op trace.Operation, id string) (storage.DataSource, error) {
	return c.DataStore.NewDataSource(op, id)
}

func (c *NameLookupCache) URL(op trace.Operation, id string) (*url.URL, error) {
	return c.DataStore.URL(op, id)
}

func (c *NameLookupCache) Owners(op trace.Operation, url *url.URL, filter func(vm *mo.VirtualMachine) bool) ([]*vm.VirtualMachine, error) {
	return c.DataStore.Owners(op, url, filter)
}

// GetImage gets the specified image from the given store by retreiving it from the cache.
func (c *NameLookupCache) GetImage(op trace.Operation, store *url.URL, ID string) (*Image, error) {
	op.Debugf("Getting image %s from %s", ID, store.String())

	storeName, err := util.ImageStoreName(store)
	if err != nil {
		return nil, err
	}

	// Check the store exists
	if _, err = c.GetImageStore(op, storeName); err != nil {
		return nil, err
	}

	c.storeCacheLock.Lock()
	indx := c.storeCache[*store]
	c.storeCacheLock.Unlock()

	imgURL, err := util.ImageURL(storeName, ID)
	if err != nil {
		return nil, err
	}
	node, err := c.storeCache[*store].Get(imgURL.String())

	var img *Image
	if err != nil {
		if err == index.ErrNodeNotFound {
			op.Debugf("Image %s not in cache, retreiving from datastore", ID)
			// Not in the cache.  Try to load it.
			img, err = c.DataStore.GetImage(op, store, ID)
			if err != nil {
				return nil, err
			}

			if err = indx.Insert(img); err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	} else {
		img, _ = node.(*Image)
	}

	return img, nil
}

// ListImages returns a list of Images for a list of IDs, or all if no IDs are passed
func (c *NameLookupCache) ListImages(op trace.Operation, store *url.URL, IDs []string) ([]*Image, error) {
	// Filter the results
	imageList := make([]*Image, 0, len(IDs))

	if len(IDs) > 0 {
		for _, id := range IDs {
			i, err := c.GetImage(op, store, id)
			if err == nil {
				imageList = append(imageList, i)
			}
		}

	} else {

		storeName, err := util.ImageStoreName(store)
		if err != nil {
			return nil, err
		}
		// Check the store exists before we start iterating it.  This will populate the cache if it's empty.
		if _, err := c.GetImageStore(op, storeName); err != nil {
			return nil, err
		}

		// get the relevant cache
		c.storeCacheLock.Lock()
		indx := c.storeCache[*store]
		c.storeCacheLock.Unlock()

		images, err := indx.List()
		if err != nil {
			return nil, err
		}

		for _, v := range images {
			img, _ := v.(*Image)
			// filter out scratch
			if img.ID == constants.ScratchLayerID {
				continue
			}

			imageList = append(imageList, img)
		}
	}

	return imageList, nil
}

// DeleteImage deletes an image from the image store.  If it is in use or is
// being inheritted from, then this will return an error.
func (c *NameLookupCache) DeleteImage(op trace.Operation, image *Image) (*Image, error) {
	// prevent deletes of scratch
	if image.ID == constants.ScratchLayerID {
		return nil, nil
	}

	op.Infof("DeleteImage: deleting %s", image.Self())

	// Check the image exists.  This will rehydrate the cache if necessary.
	img, err := c.GetImage(op, image.Store, image.ID)
	if err != nil {
		op.Errorf("DeleteImage: %s", err)
		return nil, err
	}

	// get the relevant cache
	c.storeCacheLock.Lock()
	indx := c.storeCache[*img.Store]
	c.storeCacheLock.Unlock()

	hasChildren, err := indx.HasChildren(img.Self())
	if err != nil {
		op.Errorf("DeleteImage: %s", err)
		return nil, err
	}

	if hasChildren {
		return nil, &ErrImageInUse{img.Self() + " in use by child images"}
	}

	// The datastore will tell us if the image is attached
	if _, err = c.DataStore.DeleteImage(op, img); err != nil {
		op.Errorf("%s", err)
		return nil, err
	}

	// Remove the image from the cache
	if _, err = indx.Delete(img.Self()); err != nil {
		op.Errorf("%s", err)
		return nil, err
	}

	return img, nil
}

// DeleteBranch deletes a branch of images, starting from nodeID, up to the
// first node with degree greater than 1.  keepNodes is the array of images to
// keep (and their branches).
func (c *NameLookupCache) DeleteBranch(op trace.Operation, image *Image, keepNodes []*url.URL) ([]*Image, error) {
	op.Infof("DeleteBranch: deleting branch starting at %s", image.Self())

	var deletedImages []*Image

	// map of images to keep
	keep := make(map[url.URL]int)
	for _, elem := range keepNodes {
		op.Debugf("DeleteBranch:  keep node %s", elem.String())
		keep[*elem] = 0
	}

	// Check if the error is actually an error.  If we deleted something,
	// then eat the error.  This should really only return an error if the leaf
	// has issues.
	checkErr := func(err error, deleted []*Image) ([]*Image, error) {
		if err != nil {
			if len(deleted) == 0 {
				// we failed deleting any elements.
				return nil, err
			}
		}

		if len(deleted) == 0 {
			// This can't happen.  deleteNode should have returned an err
			op.Debugf("No images deleted!!")
		}

		// we deleted a section of a branch
		return deleted, nil
	}

	for {
		if _, ok := keep[*image.SelfLink]; ok {
			return checkErr(fmt.Errorf("%s can't be deleted", image.Self()), deletedImages)
		}

		deletedImage, err := c.DeleteImage(op, image)
		if err != nil {
			op.Debugf(err.Error())
			return checkErr(err, deletedImages)
		}

		deletedImages = append(deletedImages, deletedImage)

		// iterate to the parent
		parent, err := Parse(deletedImage.ParentLink)
		if err != nil {
			return deletedImages, err
		}

		// set image to the parent
		image, err = c.GetImage(op, parent.Store, parent.ID)
		if err != nil {
			return deletedImages, err
		}

		if image.ID == constants.ScratchLayerID {
			op.Infof("DeleteBranch: Done deleting images")
			break
		}
	}

	return deletedImages, nil
}
