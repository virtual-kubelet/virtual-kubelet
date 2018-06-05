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
	"errors"
	"io"
	"net/url"
	"os"

	"github.com/vmware/vic/lib/archive"
	"github.com/vmware/vic/lib/guest"
	"github.com/vmware/vic/lib/portlayer/storage"
	"github.com/vmware/vic/lib/portlayer/storage/volume"
	"github.com/vmware/vic/lib/portlayer/storage/vsphere"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/vsphere/disk"
	"github.com/vmware/vic/pkg/vsphere/vm"
)

func (v *VolumeStore) Import(op trace.Operation, id string, spec *archive.FilterSpec, tarstream io.ReadCloser) error {
	l, err := v.NewDataSink(op, id)
	if err != nil {
		return err
	}

	return l.Import(op, spec, tarstream)
}

// NewDataSource creates and returns an DataSource associated with container storage
func (v *VolumeStore) NewDataSink(op trace.Operation, id string) (storage.DataSink, error) {
	uri, err := v.URL(op, id)
	if err != nil {
		return nil, err
	}

	offlineAttempt := 0
offline:
	offlineAttempt++

	source, err := v.newDataSink(op, uri)
	if err == nil {
		return source, err
	}

	// check for vmdk locked error here
	if !disk.IsLockedError(err) {
		op.Warnf("Unable to mount %s and do not know how to recover from error")
		// continue anyway because maybe there's an online option
	}

	// online - Owners() should filter out the appliance VM
	// #nosec: Errors unhandled.
	owners, _ := v.Owners(op, uri, disk.LockedVMDKFilter)
	if len(owners) == 0 {
		op.Infof("No online owners were found for %s", id)
		return nil, errors.New("unable to create offline data sink and no online owners found")
	}

	for _, o := range owners {
		// sanity check to see if we are the owner - this should catch transitions
		// from container running to diff or commit for example between the offline attempt and here
		uuid, err := o.UUID(op)
		if err == nil {
			// check if the vm is appliance VM if we can successfully get its UUID
			// #nosec: Errors unhandled.
			self, _ := guest.IsSelf(op, uuid)
			if self && offlineAttempt < 2 {
				op.Infof("Appliance is owner of online vmdk - retrying offline source path")
				goto offline
			}
		}

		online, err := v.newOnlineDataSink(op, o, id)
		if online != nil {
			return online, err
		}

		op.Debugf("Failed to create online sink with owner %s: %s", o.Reference(), err)
	}

	return nil, errors.New("unable to create online or offline data sink")
}

func (v *VolumeStore) newDataSink(op trace.Operation, url *url.URL) (storage.DataSink, error) {
	mountPath, cleanFunc, err := v.Mount(op, url, true)
	if err != nil {
		return nil, err
	}

	f, err := os.Open(mountPath)
	if err != nil {
		cleanFunc()
		return nil, err
	}

	op.Debugf("Created mount data sink for access to %s at %s", url, mountPath)
	return storage.NewMountDataSink(op, f, cleanFunc), nil
}

func (v *VolumeStore) newOnlineDataSink(op trace.Operation, owner *vm.VirtualMachine, id string) (storage.DataSink, error) {
	op.Debugf("Constructing toolbox data sink: %s.%s", owner.Reference(), id)

	return &vsphere.ToolboxDataSink{
		VM:    owner,
		ID:    volume.Label(id),
		Clean: func() { return },
	}, nil
}
