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

package disk

import (
	"context"
	"fmt"
	"os"
	"path"
	"testing"

	log "github.com/Sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/vic/lib/guest"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/vsphere/datastore"
	"github.com/vmware/vic/pkg/vsphere/tasks"
)

// TestLazyDetach tests lazy detach functionality to make sure that every ESXi version shows this behaviour
// https://github.com/vmware/vic/issues/5565
func TestLazyDetach(t *testing.T) {
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

	// helper fn
	reconfigure := func(changes []types.BaseVirtualDeviceConfigSpec) error {
		t.Logf("Calling reconfigure")

		machineSpec := types.VirtualMachineConfigSpec{}
		machineSpec.DeviceChange = changes

		_, err := vdm.vm.WaitForResult(op, func(ctx context.Context) (tasks.Task, error) {
			t, er := vdm.vm.Reconfigure(ctx, machineSpec)

			if t != nil {
				op.Debugf("reconfigure task=%s", t.Reference())
			}

			return t, er
		})
		return err
	}

	oddity := "Ground control to Major Tom"
	operation := func(path *object.DatastorePath, read bool) error {
		// this is fundamentally checking persistent disks
		devicePath, err := vdm.devicePathByURI(op, path, true)
		if err != nil {
			return err
		}

		blockDev, err := waitForDevice(op, devicePath)
		if err != nil {
			return err
		}

		f, err := os.OpenFile(blockDev, os.O_RDWR, os.FileMode(0777))
		if err != nil {
			return err
		}
		defer f.Close()

		if read {
			// Try to read the whole string
			b := make([]byte, len(oddity))
			_, err = f.Read(b)
			if err != nil {
				return err
			}
			// Check against the test string
			if oddity != string(b) {
				return fmt.Errorf("Read string is not the same one we wrote")
			}
		} else {
			// Write directly to the disk
			_, err = f.Write([]byte(oddity))
			if err != nil {
				return err
			}
		}

		return f.Sync()
	}

	// 1MB
	diskSize := int64(1 << 10)
	scratch := &object.DatastorePath{
		Datastore: session.Datastore.Name(),
		Path:      path.Join(imagestore.Path, "scratch.vmdk"),
	}
	// config
	config := NewPersistentDisk(scratch).WithCapacity(diskSize)

	// attach + create spec (scratch)
	spec := []types.BaseVirtualDeviceConfigSpec{
		&types.VirtualDeviceConfigSpec{
			Device:        vdm.toSpec(config),
			Operation:     types.VirtualDeviceConfigSpecOperationAdd,
			FileOperation: types.VirtualDeviceConfigSpecFileOperationCreate,
		},
	}
	// reconfigure
	err = reconfigure(spec)
	if !assert.NoError(t, err) {
		return
	}
	t.Logf("scratch created and attached")

	// ref to scratch (needed for detach as initial spec's Key and UnitNumber was unset)
	disk, err := findDiskByFilename(op, vdm.vm, scratch.Path, true)
	if !assert.NoError(t, err) {
		return
	}

	err = operation(scratch, false)
	if !assert.NoError(t, err) {
		return
	}

	// DO NOT DETACH AND START WORKING ON THE CHILD

	// child
	child := &object.DatastorePath{
		Datastore: session.Datastore.Name(),
		Path:      path.Join(imagestore.Path, "child.vmdk"),
	}
	// config
	config = NewPersistentDisk(child).WithParent(scratch)

	// detach (scratch) AND attach + create (child) spec
	spec = []types.BaseVirtualDeviceConfigSpec{
		&types.VirtualDeviceConfigSpec{
			Device:    disk,
			Operation: types.VirtualDeviceConfigSpecOperationRemove,
		},
		&types.VirtualDeviceConfigSpec{
			Device:        vdm.toSpec(config),
			Operation:     types.VirtualDeviceConfigSpecOperationAdd,
			FileOperation: types.VirtualDeviceConfigSpecFileOperationCreate,
		},
	}
	// reconfigure
	err = reconfigure(spec)
	if !assert.NoError(t, err) {
		return
	}
	t.Logf("scratch detached, child created and attached")

	err = operation(child, true)
	if !assert.NoError(t, err) {
		return
	}

	// ref to child (needed for detach as initial spec's Key and UnitNumber was unset)
	disk, err = findDiskByFilename(op, vdm.vm, child.Path, true)
	if !assert.NoError(t, err) {
		return
	}

	// detach  spec (child)
	spec = []types.BaseVirtualDeviceConfigSpec{
		&types.VirtualDeviceConfigSpec{
			Device:    disk,
			Operation: types.VirtualDeviceConfigSpecOperationRemove,
		},
	}
	// reconfigure
	err = reconfigure(spec)
	if !assert.NoError(t, err) {
		return
	}
	t.Logf("child detached")
}
