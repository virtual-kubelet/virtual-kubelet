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
	"io"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/vic/lib/portlayer/storage"
	"github.com/vmware/vic/lib/portlayer/util"
	"github.com/vmware/vic/pkg/index"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/vsphere/disk"
)

// ImageStorer is an interface to store images in the Image Store
type ImageStorer interface {

	// CreateImageStore creates a location to store images and creates a root
	// disk which serves as the parent of all layers.
	//
	// storeName - The name of the image store to be created.  This must be
	// unique.
	//
	// Returns the URL of the created store
	CreateImageStore(op trace.Operation, storeName string) (*url.URL, error)

	// DeleteImageStore is used to cleanup the image store.  This can only be
	// called once there are no images left in the image store.
	DeleteImageStore(op trace.Operation, storeName string) error

	// Gets the url to an image store via name
	GetImageStore(op trace.Operation, storeName string) (*url.URL, error)

	// ListImageStores lists the available image stores
	ListImageStores(op trace.Operation) ([]*url.URL, error)

	// WriteImage creates a new image layer from the given parent.  Eg
	// parentImage + newLayer = new Image built from parent
	//
	// parent - The parent image to create the new image from.
	// ID - textual ID for the image to be written
	// meta - metadata associated with the image
	// sum - expected sha266 sum of the image content.
	// r - the image tar to be written
	WriteImage(op trace.Operation, parent *Image, ID string, meta map[string][]byte, sum string, r io.Reader) (*Image, error)

	// GetImage queries the image store for the specified image.
	//
	// store - The image store to query name - The name of the image (optional)
	// ID - textual ID for the image to be retrieved
	GetImage(op trace.Operation, store *url.URL, ID string) (*Image, error)

	// ListImages returns a list of Images given a list of image IDs, or all
	// images in the image store if no param is passed.
	ListImages(op trace.Operation, store *url.URL, IDs []string) ([]*Image, error)

	// DeleteImage deletes an image from the image store.  If the image is in
	// use either by way of inheritance or because it's attached to a
	// container, this will return an error.
	DeleteImage(op trace.Operation, image *Image) (*Image, error)

	storage.Resolver
	storage.Importer
	storage.Exporter
}

// Image is the handle to identify an image layer on the backing store.  The
// URI namespace used to identify the Image in the storage layer has the
// following path scheme:
//
// `/storage/<image store identifier, usually the vch uuid>/<image id>`
//
type Image struct {
	// ID is the identifier for this layer.  Usually a SHA
	ID string

	// SelfLink is the URL for this layer.  Filled in by the runtime.
	SelfLink *url.URL

	// ParentLink is the URL for the parent.  It's the VMDK this snapshot inherits from.
	ParentLink *url.URL

	// Store is the URL for the image store the image can be found on.
	Store *url.URL

	// Metadata associated with the image.
	Metadata map[string][]byte

	// Disk is the underlying disk implementation
	Disk *disk.VirtualDisk

	// DatastorePath is the dspath for actually using this image
	// NOTE: this should be replaced by structure accessors for the data and updated storage
	// interfaces that use _one_ variant of url/path for identifying images, volumes and stores.
	// URL was only suggested as an existing structure that could be leveraged when object.DatastorePath
	// was note available. The suggestion seems to have spawned monstruous unnecessary complexity.
	DatastorePath *object.DatastorePath
}

func (i *Image) Copy() index.Element {

	// #nosec: Errors unhandled.
	selflink, _ := url.Parse(i.SelfLink.String())
	// #nosec: Errors unhandled.
	store, _ := url.Parse(i.Store.String())

	var parent *url.URL
	if i.ParentLink != nil {
		// #nosec: Errors unhandled.
		parent, _ = url.Parse(i.ParentLink.String())
	}

	c := &Image{
		ID:         i.ID,
		SelfLink:   selflink,
		ParentLink: parent,
		Store:      store,
		DatastorePath: &object.DatastorePath{
			Datastore: i.DatastorePath.Datastore,
			Path:      i.DatastorePath.Path,
		},
	}

	if i.Metadata != nil {
		c.Metadata = make(map[string][]byte)

		for k, v := range i.Metadata {
			buf := make([]byte, len(v))
			copy(buf, v)
			c.Metadata[k] = buf
		}
	}
	return c
}

// Returns the Selflink of the image
func (i *Image) Self() string {
	return i.SelfLink.String()
}

// Returns a link to the parent.  Returns link to self if there is no parent
func (i *Image) Parent() string {
	if i.ParentLink != nil {
		return i.ParentLink.String()
	}
	return i.Self()
}

func Parse(u *url.URL) (*Image, error) {
	// Check the path isn't malformed.
	if !filepath.IsAbs(u.Path) {
		return nil, errors.New("invalid uri path")
	}

	segments := strings.Split(filepath.Clean(u.Path), "/")
	if segments[0] == "" {
		segments = segments[1:]
	}

	if segments[0] != util.StorageURLPath {
		return nil, errors.New("not a storage path")
	}

	if len(segments) < 3 {
		return nil, errors.New("uri path mismatch")
	}

	store, err := util.ImageStoreNameToURL(segments[2])
	if err != nil {
		return nil, err
	}

	id := segments[3]

	var SelfLink url.URL
	SelfLink = *u

	i := &Image{
		ID:       id,
		SelfLink: &SelfLink,
		Store:    store,
	}

	return i, nil
}
