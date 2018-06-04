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

package disk

import (
	"path"
	"testing"

	"context"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/mount"
	"github.com/stretchr/testify/assert"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/vic/lib/guest"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/vsphere/datastore"
)

// Create a disk, make an ext filesystem on it, set the label, mount it,
// unmount it, then clean up.
func TestCreateFS(t *testing.T) {
	log.SetLevel(log.InfoLevel)
	if testing.Verbose() {
		log.SetLevel(log.DebugLevel)
	}

	session := Session(context.Background(), t)
	if session == nil {
		return
	}

	op := trace.NewOperation(context.TODO(), t.Name())

	vchvm, err := guest.GetSelf(op, session)
	if err != nil {
		t.Skip("Not in a vm")
	}
	view := ContainerView(op, session, vchvm)
	if view == nil {
		t.Skip("Can't create a view")
	}

	imagestore := &object.DatastorePath{
		Datastore: session.Datastore.Name(),
		Path:      datastore.TestName(t.Name()),
	}

	// file manager
	fm := object.NewFileManager(session.Vim25())
	// create a directory in the datastore
	err = fm.MakeDirectory(context.TODO(), imagestore.String(), nil, true)
	if !assert.NoError(t, err) {
		return
	}

	// Nuke the image store
	defer func() {
		task, err := fm.DeleteDatastoreFile(context.TODO(), imagestore.String(), nil)
		if !assert.NoError(t, err) {
			return
		}
		_, err = task.WaitForResult(context.TODO(), nil)
		if !assert.NoError(t, err) {
			return
		}
	}()

	// create a diskmanager
	vdm, err := NewDiskManager(op, session, view)
	if !assert.NoError(t, err) || !assert.NotNil(t, vdm) {
		return
	}

	diskSize := int64(1 << 10)
	scratch := &object.DatastorePath{
		Datastore: session.Datastore.Name(),
		Path:      path.Join(imagestore.Path, "scratch.vmdk"),
	}

	config := NewPersistentDisk(scratch).WithCapacity(diskSize)
	d, err := vdm.CreateAndAttach(op, config)
	if !assert.NoError(t, err) {
		return
	}

	// make the filesysetem
	if err = d.Mkfs(op, "foo"); !assert.NoError(t, err) {
		return
	}

	// set the label
	if err = d.SetLabel(op, "foo"); !assert.NoError(t, err) {
		return
	}

	// do the mount
	dir, err := d.Mount(op, nil)
	if !assert.NoError(t, err) {
		return
	}

	// boom
	if mounted, err := mount.Mounted(dir); !assert.NoError(t, err) || !assert.True(t, mounted) {
		return
	}

	//  clean up
	err = d.Unmount(op)
	if !assert.NoError(t, err) {
		return
	}

	err = vdm.Detach(op, config)
	if !assert.NoError(t, err) {
		return
	}
}

func TestAttachFS(t *testing.T) {
	log.SetLevel(log.InfoLevel)
	if testing.Verbose() {
		log.SetLevel(log.DebugLevel)
	}

	session := Session(context.Background(), t)
	if session == nil {
		return
	}

	op := trace.NewOperation(context.TODO(), t.Name())

	vchvm, err := guest.GetSelf(op, session)
	if err != nil {
		t.Skip("Not in a vm")
	}
	view := ContainerView(op, session, vchvm)
	if view == nil {
		t.Skip("Can't create a view")
	}

	imagestore := &object.DatastorePath{
		Datastore: session.Datastore.Name(),
		Path:      datastore.TestName(t.Name()),
	}

	// file manager
	fm := object.NewFileManager(session.Vim25())
	// create a directory in the datastore
	err = fm.MakeDirectory(context.TODO(), imagestore.String(), nil, true)
	if !assert.NoError(t, err) {
		return
	}

	// Nuke the image store
	defer func() {
		task, err := fm.DeleteDatastoreFile(context.TODO(), imagestore.String(), nil)
		if !assert.NoError(t, err) {
			return
		}
		_, err = task.WaitForResult(context.TODO(), nil)
		if !assert.NoError(t, err) {
			return
		}
	}()

	// create a diskmanager
	vdm, err := NewDiskManager(op, session, view)
	if !assert.NoError(t, err) || !assert.NotNil(t, vdm) {
		return
	}

	diskSize := int64(1 << 10)
	scratch := &object.DatastorePath{
		Datastore: session.Datastore.Name(),
		Path:      path.Join(imagestore.Path, "scratch.vmdk"),
	}

	config := NewPersistentDisk(scratch).WithCapacity(diskSize)
	d, err := vdm.CreateAndAttach(op, config)
	if !assert.NoError(t, err) {
		return
	}

	// make the filesysetem
	if err = d.Mkfs(op, "foo"); !assert.NoError(t, err) {
		return
	}

	// set the label
	if err = d.SetLabel(op, "foo"); !assert.NoError(t, err) {
		return
	}

	// do the mount
	dir, err := d.Mount(op, nil)
	if !assert.NoError(t, err) {
		return
	}

	// boom
	if mounted, err := mount.Mounted(dir); !assert.NoError(t, err) || !assert.True(t, mounted) {
		return
	}

	//  clean up
	err = d.Unmount(op)
	if !assert.NoError(t, err) {
		return
	}

	err = vdm.Detach(op, config)
	if !assert.NoError(t, err) {
		return
	}

	child := &object.DatastorePath{
		Datastore: session.Datastore.Name(),
		Path:      path.Join(imagestore.Path, "child.vmdk"),
	}

	config = NewPersistentDisk(child).WithParent(scratch)
	d, err = vdm.CreateAndAttach(op, config)
	if !assert.NoError(t, err) {
		return
	}

	// do the mount
	dir, err = d.Mount(op, nil)
	if !assert.NoError(t, err) {
		return
	}

	// boom
	if mounted, err := mount.Mounted(dir); !assert.NoError(t, err) || !assert.True(t, mounted) {
		return
	}

	//  clean up
	err = d.Unmount(op)
	if !assert.NoError(t, err) {
		return
	}

	err = vdm.Detach(op, config)
	if !assert.NoError(t, err) {
		return
	}

	for i := 0; i < 5; i++ {
		config = NewPersistentDisk(child)
		d, err = vdm.CreateAndAttach(op, config)
		if !assert.NoError(t, err) {
			return
		}

		// do the mount
		dir, err = d.Mount(op, nil)
		if !assert.NoError(t, err) {
			return
		}

		// boom
		if mounted, err := mount.Mounted(dir); !assert.NoError(t, err) || !assert.True(t, mounted) {
			return
		}

		//  clean up
		err = d.Unmount(op)
		if !assert.NoError(t, err) {
			return
		}

		err = vdm.Detach(op, config)
		if !assert.NoError(t, err) {
			return
		}
	}
}
