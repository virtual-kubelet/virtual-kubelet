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

package container

import (
	"errors"
	"io"
	"net/url"
	"os"

	"github.com/vmware/vic/lib/archive"
	"github.com/vmware/vic/lib/guest"
	"github.com/vmware/vic/lib/portlayer/storage"
	"github.com/vmware/vic/lib/portlayer/storage/vsphere"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/vsphere/disk"
	"github.com/vmware/vic/pkg/vsphere/vm"
)

func (c *ContainerStore) Export(op trace.Operation, id, ancestor string, spec *archive.FilterSpec, data bool) (io.ReadCloser, error) {
	l, err := c.NewDataSource(op, id)
	if err != nil {
		return nil, err
	}

	if ancestor == "" {
		op.Infof("No ancestor specified so following basic export path")
		return l.Export(op, spec, data)
	}

	// for now we assume ancetor instead of entirely generic left/right
	// this allows us to assume it's an image
	img, err := c.images.URL(op, ancestor)
	if err != nil {
		op.Errorf("Failed to map ancestor %s to image: %s", ancestor, err)

		l.Close()
		return nil, err
	}
	op.Debugf("Mapped ancestor %s to %s", ancestor, img.String())

	r, err := c.newDataSource(op, img, false)
	if err != nil {
		op.Debugf("Unable to get datasource for ancestor: %s", err)

		l.Close()
		return nil, err
	}

	closers := func() error {
		op.Debugf("Callback to io.Closer function for container export")

		l.Close()
		r.Close()

		return nil
	}

	ls := l.Source()
	rs := r.Source()

	fl, lok := ls.(*os.File)
	fr, rok := rs.(*os.File)

	if !lok || !rok {
		go closers()
		return nil, errors.New("mismatched datasource types")
	}

	// if we want data, exclude the xattrs, otherwise assume diff
	xattrs := !data

	tar, err := archive.Diff(op, fl.Name(), fr.Name(), spec, data, xattrs)
	if err != nil {
		go closers()
		return nil, err
	}

	return &storage.ProxyReadCloser{
		ReadCloser: tar,
		Closer:     closers,
	}, nil
}

// NewDataSource creates and returns an DataSource associated with container storage
func (c *ContainerStore) NewDataSource(op trace.Operation, id string) (storage.DataSource, error) {
	uri, err := c.URL(op, id)
	if err != nil {
		return nil, err
	}

	offlineAttempt := 0
offline:
	offlineAttempt++

	// This is persistent to avoid issues with concurrent Stat/Import calls
	source, err := c.newDataSource(op, uri, true)
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
	owners, _ := c.Owners(op, uri, disk.LockedVMDKFilter)
	if len(owners) == 0 {
		op.Infof("No online owners were found for %s", id)
		return nil, errors.New("unable to create offline data source and no online owners found")
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

		online, err := c.newOnlineDataSource(op, o, id)
		if online != nil {
			return online, err
		}

		op.Debugf("Failed to create online datasource with owner %s: %s", o.Reference(), err)
	}

	return nil, errors.New("unable to create online or offline data source")
}

func (c *ContainerStore) newDataSource(op trace.Operation, url *url.URL, persistent bool) (storage.DataSource, error) {
	mountPath, cleanFunc, err := c.Mount(op, url, persistent)
	if err != nil {
		return nil, err
	}

	f, err := os.Open(mountPath)
	if err != nil {
		cleanFunc()
		return nil, err
	}

	op.Debugf("Created mount data source for access to %s at %s", url, mountPath)
	return storage.NewMountDataSource(op, f, cleanFunc), nil
}

func (c *ContainerStore) newOnlineDataSource(op trace.Operation, owner *vm.VirtualMachine, id string) (storage.DataSource, error) {
	op.Debugf("Constructing toolbox data source: %s.%s", owner.Reference(), id)

	return &vsphere.ToolboxDataSource{
		VM:    owner,
		ID:    id,
		Clean: func() { return },
	}, nil
}
