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
	"crypto/md5" // #nosec: Use of weak cryptographic primitive
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/vmware/vic/lib/config/executor"
	"github.com/vmware/vic/lib/portlayer/storage"
	"github.com/vmware/vic/lib/portlayer/util"
	"github.com/vmware/vic/pkg/trace"
)

type Disk interface {
	// Path to this disk on the VCH
	MountPath() (string, error)

	// Path to the disk on the datastore
	DiskPath() url.URL
}

// VolumeStorer is an interface to create, remove, enumerate, and get Volumes.
type VolumeStorer interface {
	// Creates a volume on the given volume store, of the given size, with the given metadata.
	VolumeCreate(op trace.Operation, ID string, store *url.URL, capacityKB uint64, info map[string][]byte) (*Volume, error)

	// Destroys a volume
	VolumeDestroy(op trace.Operation, vol *Volume) error

	// Lists all volumes
	VolumesList(op trace.Operation) ([]*Volume, error)

	// The interfaces necessary for Import and Export
	storage.Resolver
	storage.Importer
	storage.Exporter
}

// Volume is the handle to identify a volume on the backing store.  The URI
// namespace used to identify the Volume in the storage layer has the following
// path scheme:
//
// `/storage/volumes/<volume store identifier, usually the vch uuid>/<volume id>`
//
type Volume struct {
	// Identifies the volume
	ID string

	// Label is the computed label of the Volume.  This is set by the runtime.
	Label string

	// The volumestore the volume lives on. (e.g the datastore + vch + configured vol directory)
	Store *url.URL

	// Metadata the volume is included with.  Is persisted along side the volume vmdk.
	Info map[string][]byte

	// Namespace in the storage layer to look up this volume.
	SelfLink *url.URL

	// Backing device
	Device Disk

	CopyMode executor.CopyMode
}

// NewVolume creates a Volume
func NewVolume(store *url.URL, ID string, info map[string][]byte, device Disk, copyMode executor.CopyMode) (*Volume, error) {
	storeName, err := util.VolumeStoreName(store)
	if err != nil {
		return nil, err
	}

	selflink, err := util.VolumeURL(storeName, ID)
	if err != nil {
		return nil, err
	}

	// Set the label to the md5 of the ID

	vol := &Volume{
		ID:       ID,
		Label:    Label(ID),
		Store:    store,
		SelfLink: selflink,
		Device:   device,
		Info:     info,
		CopyMode: copyMode,
	}
	return vol, nil
}

// given an ID, compute the volume's label
func Label(ID string) string {

	// e2label's manpage says the label size is 16 chars
	// #nosec: Use of weak cryptographic primitive
	m := md5.Sum([]byte(ID))
	return fmt.Sprintf("%x", m)[:16]
}

func (v *Volume) Parse(u *url.URL) error {
	// Check the path isn't malformed.
	if !filepath.IsAbs(u.Path) {
		return errors.New("invalid uri path")
	}

	segments := strings.Split(filepath.Clean(u.Path), "/")[1:]

	if segments[0] != util.StorageURLPath {
		return errors.New("not a storage path")
	}

	if len(segments) < 3 {
		return errors.New("uri path mismatch")
	}

	store, err := util.VolumeStoreNameToURL(segments[2])
	if err != nil {
		return err
	}

	id := segments[3]

	var SelfLink url.URL
	SelfLink = *u

	v.ID = id
	v.SelfLink = &SelfLink
	v.Store = store

	return nil
}
