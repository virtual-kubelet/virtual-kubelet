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
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/vic/lib/config/executor"
	"github.com/vmware/vic/lib/constants"
	"github.com/vmware/vic/lib/portlayer/storage"
	"github.com/vmware/vic/lib/portlayer/storage/volume"
	"github.com/vmware/vic/lib/portlayer/storage/vsphere"
	"github.com/vmware/vic/lib/portlayer/util"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/vsphere/datastore"
	"github.com/vmware/vic/pkg/vsphere/disk"
	"github.com/vmware/vic/pkg/vsphere/session"
)

const (
	// TODO: this was shared with image store hence the disjoint naming. Should be updated
	// but migration/upgrade implications are unclear
	metaDataDir = "imageMetadata"
)

var (
	// Set to false for unit tests
	DetachAll = true
)

// VolumeStore caches Volume references to volumes in the system.
type VolumeStore struct {
	disk.Vmdk

	// Service url to this VolumeStore
	SelfLink *url.URL
}

func NewVolumeStore(op trace.Operation, storeName string, s *session.Session, ds *datastore.Helper) (*VolumeStore, error) {
	// Create the volume dir if it doesn't already exist
	if _, err := ds.Mkdir(op, true, constants.VolumesDir); err != nil && !os.IsExist(err) {
		return nil, err
	}

	dm, err := disk.NewDiskManager(op, s, storage.Config.ContainerView)
	if err != nil {
		return nil, err
	}

	if DetachAll {
		if err = dm.DetachAll(op); err != nil {
			return nil, err
		}
	}

	u, err := util.VolumeStoreNameToURL(storeName)
	if err != nil {
		return nil, err
	}

	v := &VolumeStore{
		Vmdk: disk.Vmdk{
			Manager: dm,
			Helper:  ds,
			Session: s,
		},
		SelfLink: u,
	}

	return v, nil
}

// Returns the path to the vol relative to the given store.  The dir structure
// for a vol in the datastore is `<configured datastore path>/volumes/<vol ID>/<vol ID>.vmkd`.
// Everything up to "volumes" is taken care of by the datastore wrapper.
func (v *VolumeStore) volDirPath(ID string) string {
	return path.Join(constants.VolumesDir, ID)
}

// Returns the path to the metadata directory for a volume
func (v *VolumeStore) volMetadataDirPath(ID string) string {
	return path.Join(v.volDirPath(ID), metaDataDir)
}

// Returns the path to the vmdk itself (in datastore URL format)
func (v *VolumeStore) volDiskDSPath(ID string) *object.DatastorePath {
	return &object.DatastorePath{
		Datastore: v.Helper.RootURL.Datastore,
		Path:      path.Join(v.Helper.RootURL.Path, v.volDirPath(ID), ID+".vmdk"),
	}
}

func (v *VolumeStore) VolumeCreate(op trace.Operation, ID string, store *url.URL, capacityKB uint64, info map[string][]byte) (*volume.Volume, error) {

	// Create the volume directory in the store.
	if _, err := v.Mkdir(op, false, v.volDirPath(ID)); err != nil {
		return nil, err
	}

	// Get the path to the disk in datastore uri format
	volDiskDSPath := v.volDiskDSPath(ID)

	config := disk.NewPersistentDisk(volDiskDSPath).WithCapacity(int64(capacityKB))
	// Create the disk
	vmdisk, err := v.CreateAndAttach(op, config)
	if err != nil {
		return nil, err
	}
	defer v.Detach(op, vmdisk.VirtualDiskConfig)
	vol, err := volume.NewVolume(store, ID, info, vmdisk, executor.CopyNew)
	if err != nil {
		return nil, err
	}

	// Make the filesystem and set its label
	if err = vmdisk.Mkfs(op, vol.Label); err != nil {
		return nil, err
	}

	// mask lost+found from containerVM
	opts := []string{"noatime"}
	path, err := vmdisk.Mount(op, opts)
	if err != nil {
		return nil, err
	}
	defer vmdisk.Unmount(op)

	// #nosec
	err = os.Mkdir(filepath.Join(path, disk.VolumeDataDir), 0755)
	if err != nil {
		return nil, err
	}

	// Persist the metadata
	metaDataDir := v.volMetadataDirPath(ID)
	if err = vsphere.WriteMetadata(op, v.Helper, metaDataDir, info); err != nil {
		return nil, err
	}

	op.Infof("volumestore: %s (%s)", ID, vol.SelfLink)
	return vol, nil
}

func (v *VolumeStore) VolumeDestroy(op trace.Operation, vol *volume.Volume) error {
	volDir := v.volDirPath(vol.ID)

	op.Infof("VolumeStore: Deleting %s", volDir)
	if err := v.Rm(op, volDir); err != nil {
		op.Errorf("VolumeStore: delete error: %s", err.Error())
		return err
	}
	return nil
}

func (v *VolumeStore) VolumeGet(op trace.Operation, ID string) (*volume.Volume, error) {
	// We can't get the volume directly without looking up what datastore it's on.
	return nil, fmt.Errorf("not supported: use VolumesList")
}

func (v *VolumeStore) VolumesList(op trace.Operation) ([]*volume.Volume, error) {
	volumes := []*volume.Volume{}

	res, err := v.Ls(op, constants.VolumesDir)
	if err != nil {
		return nil, fmt.Errorf("error listing vols: %s", err)
	}

	for _, f := range res.File {
		ID := f.GetFileInfo().Path

		// Get the path to the disk in datastore uri format
		volDiskDSPath := v.volDiskDSPath(ID)

		config := disk.NewPersistentDisk(volDiskDSPath)
		dev, err := disk.NewVirtualDisk(op, config, v.Manager.Disks)
		if err != nil {
			return nil, err
		}

		metaDataDir := v.volMetadataDirPath(ID)
		meta, err := vsphere.GetMetadata(op, v.Helper, metaDataDir)
		if err != nil {
			return nil, err
		}

		vol, err := volume.NewVolume(v.SelfLink, ID, meta, dev, executor.CopyNew)
		if err != nil {
			return nil, err
		}

		volumes = append(volumes, vol)
	}

	return volumes, nil
}

func (v *VolumeStore) URL(op trace.Operation, id string) (*url.URL, error) {
	path := v.volDiskDSPath(id).String()
	if path == "" {
		return nil, fmt.Errorf("unable to translate %s into datastore path", id)
	}

	return &url.URL{
		Scheme: "ds",
		Path:   path,
	}, nil
}
