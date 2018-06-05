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
	"context"
	"fmt"
	"net/url"
	"os"
	"sync"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/view"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/vic/pkg/errors"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/vsphere/datastore"
	"github.com/vmware/vic/pkg/vsphere/guest"
	"github.com/vmware/vic/pkg/vsphere/session"
	"github.com/vmware/vic/pkg/vsphere/tasks"
	"github.com/vmware/vic/pkg/vsphere/vm"
)

const (
	// You can assign the device to (1:z ), where 1 is SCSI controller 1 and z is a virtual device node from 0 to 15.
	// https://pubs.vmware.com/vsphere-65/index.jsp#com.vmware.vsphere.vm_admin.doc/GUID-5872D173-A076-42FE-8D0B-9DB0EB0E7362.html
	MaxAttachedDisks = 16
)

// Manager manages disks for the vm it runs on.  The expectation is this is run
// from a VM on a vsphere instance.  This VM creates disks on ESX, attaches
// them to itself, writes to them, then detaches them.
type Manager struct {
	// We can't have more than this number of disks attached.
	maxAttached chan bool

	// reference to the vm this is running on.
	vm *vm.VirtualMachine

	// VirtualDiskManager that is used to create vmdks directly on datastore
	// from https://pubs.vmware.com/vsphere-65/index.jsp?topic=%2Fcom.vmware.vspsdk.apiref.doc%2Fvim.VirtualDiskManager.html
	// Most VirtualDiskManager APIs will be DEPRECATED as of vSphere 6.5. Please use VStorageObjectManager APIs to manage Virtual disks.
	vdMngr *object.VirtualDiskManager

	// ContainerView - https://pubs.vmware.com/vsphere-6-0/index.jsp#com.vmware.wssdk.apiref.doc/vim.view.ContainerView.html
	view *view.ContainerView

	// The controller on this vm.
	controller *types.ParaVirtualSCSIController

	// The PCI + SCSI device /dev node string format the disks can be attached with
	byPathFormat string

	// serialize reconfigure operations
	mu sync.Mutex

	// map of URIs to VirtualDisk structs so that we can return the same instance to the caller, required for ref counting
	Disks map[uint64]*VirtualDisk
	// used for locking the disk cache
	disksLock sync.Mutex
}

// NewDiskManager creates a new Manager instance associated with the caller VM
func NewDiskManager(op trace.Operation, session *session.Session, v *view.ContainerView) (*Manager, error) {
	defer trace.End(trace.Begin(""))

	vm, err := guest.GetSelf(op, session)
	if err != nil {
		return nil, errors.Trace(err)
	}

	// create handle to the docker daemon VM as we need to mount disks on it
	controller, byPathFormat, err := verifyParavirtualScsiController(op, vm)
	if err != nil {
		op.Errorf("scsi controller verification failed: %s", err.Error())
		return nil, err
	}

	return &Manager{
		maxAttached:  make(chan bool, MaxAttachedDisks),
		vm:           vm,
		vdMngr:       object.NewVirtualDiskManager(vm.Vim25()),
		view:         v,
		controller:   controller,
		byPathFormat: byPathFormat,
		Disks:        make(map[uint64]*VirtualDisk),
	}, nil
}

// toSpec converts the given config to VirtualDisk spec
func (m *Manager) toSpec(config *VirtualDiskConfig) *types.VirtualDisk {
	backing := &types.VirtualDiskFlatVer2BackingInfo{
		DiskMode:        string(config.DiskMode),
		ThinProvisioned: types.NewBool(true),
		VirtualDeviceFileBackingInfo: types.VirtualDeviceFileBackingInfo{
			FileName: config.DatastoreURI.String(),
		},
	}

	if config.UUID != "" {
		backing.Uuid = config.UUID
	}

	disk := &types.VirtualDisk{
		VirtualDevice: types.VirtualDevice{
			Key:           -1,
			ControllerKey: m.controller.Key,
			UnitNumber:    new(int32),
			Backing:       backing,
		},
		// As of vSphere API 5.5 capacityInKB is deprecated. Documentation suggest using capacityInBytes but we can't unset CapacityInKB and its default value 0 causes problems
		// ... Exception thrown during reconfigure: (vim.vm.ConfigSpec) {
		// ...
		// -->             unitNumber = -1,
		// -->             capacityInKB = 0,
		// -->             capacityInBytes = 8192000000,
		// -->             shares = (vim.SharesInfo) null,
		// ...
		CapacityInBytes: config.CapacityInKB * 1024,
		CapacityInKB:    config.CapacityInKB,
	}

	if config.ParentDatastoreURI != nil {
		backing.Parent = &types.VirtualDiskFlatVer2BackingInfo{
			VirtualDeviceFileBackingInfo: types.VirtualDeviceFileBackingInfo{
				FileName: config.ParentDatastoreURI.String(),
			},
		}

		// Capacity needs to be 0 as we inherit it from the parent
		disk.CapacityInBytes = 0
		disk.CapacityInKB = 0
	}

	// It's possible the VCH has a disk already attached.
	*disk.VirtualDevice.UnitNumber = -1

	return disk
}

// CreateAndAttach creates a new VMDK, attaches it and ensures that the device becomes visible to the caller.
// Returns a VirtualDisk corresponding to the created and attached disk.
func (m *Manager) CreateAndAttach(op trace.Operation, config *VirtualDiskConfig) (*VirtualDisk, error) {
	defer trace.End(trace.Begin(config.DatastoreURI.String()))

	// Get or create entry in disk cache
	m.disksLock.Lock()
	d, err := NewVirtualDisk(op, config, m.Disks)
	if err != nil {
		m.disksLock.Unlock()

		op.Errorf("Unable to create disk entry: %s", err)
		return nil, err
	}
	// take disk lock before we release the cache lock - this prevents the disk being removed from the cache
	// before we get a chance to adjust refcounts
	d.l.Lock()
	defer d.l.Unlock()

	m.disksLock.Unlock()

	// check if the disk is attached from the perspective of the cache entry
	if d.DevicePath != "" {
		// this is a horrificaly misnamed call - it's incrementing the reference count
		d.setAttached(op, "")
		return d, nil
	}

	op.Infof("Create/attach vmdk %s from parent %s", config.DatastoreURI, config.ParentDatastoreURI)

	// we use findDiskByFilename to check if the disk is already attached
	// if it is then it's indicative of an error because it wasn't found in the cache, but this lets us recover
	_, ferr := findDiskByFilename(op, m.vm, d.DatastoreURI.String(), d.IsPersistent())
	if os.IsNotExist(ferr) {
		if err := m.attach(op, config); err != nil {
			return nil, errors.Trace(err)
		}
	} else {
		op.Errorf("Failed to determine if disk is already attached: %s", err)
		// this will be tidied up if/when the waitForDevice fails
	}

	op.Debugf("Mapping vmdk to pci device %s", config.DatastoreURI)
	devicePath, err := m.devicePathByURI(op, config.DatastoreURI, d.IsPersistent())
	if err != nil {
		return nil, errors.Trace(err)
	}

	blockDev, err := waitForDevice(op, devicePath)
	if err != nil {
		op.Errorf("waitForDevice failed for %s with %s", d.DatastoreURI, errors.ErrorStack(err))
		// ensure that the disk is detached if it's the publish that's failed

		disk, findErr := findDiskByFilename(op, m.vm, d.DatastoreURI.String(), d.IsPersistent())
		if findErr != nil {
			op.Debugf("findDiskByFilename(%s) failed with %s", d.DatastoreURI, errors.ErrorStack(findErr))
		}

		if detachErr := m.detach(op, disk); detachErr != nil {
			op.Debugf("detach(%s) failed with %s", d.DatastoreURI, errors.ErrorStack(detachErr))
		}

		return nil, errors.Trace(err)
	}

	err = d.setAttached(op, blockDev)

	return d, err
}

// Create creates a disk without a parent (and doesn't attach it).
func (m *Manager) Create(op trace.Operation, config *VirtualDiskConfig) (*VirtualDisk, error) {
	defer trace.End(trace.Begin(config.DatastoreURI.String()))

	var err error

	d, err := NewVirtualDisk(op, config, m.Disks)
	if err != nil {
		return nil, errors.Trace(err)
	}
	d.l.Lock()
	defer d.l.Unlock()

	spec := &types.FileBackedVirtualDiskSpec{
		VirtualDiskSpec: types.VirtualDiskSpec{
			DiskType:    string(types.VirtualDiskTypeThin),
			AdapterType: string(types.VirtualDiskAdapterTypeLsiLogic),
		},
		CapacityKb: config.CapacityInKB,
	}

	op.Infof("Creating vmdk for layer or volume %s", d.DatastoreURI)
	err = tasks.Wait(op, func(ctx context.Context) (tasks.Task, error) {
		return m.vdMngr.CreateVirtualDisk(ctx, d.DatastoreURI.String(), nil, spec)
	})

	if err != nil {
		return nil, errors.Trace(err)
	}

	return d, nil
}

// Gets a disk given a datastore path URI to the vmdk
func (m *Manager) Get(op trace.Operation, config *VirtualDiskConfig) (*VirtualDisk, error) {
	defer trace.End(trace.Begin(config.DatastoreURI.String()))

	d, err := NewVirtualDisk(op, config, m.Disks)
	if err != nil {
		return nil, errors.Trace(err)
	}

	d.l.Lock()
	defer d.l.Unlock()

	d.ParentDatastoreURI, err = m.DiskParent(op, config)
	return d, err
}

// DiskParent returns the parent for an existing disk, based on the disk datastore URI in the config,
// and ignoring any parent specified in the config.
// datastore path will be nil if the disk has no parent
func (m *Manager) DiskParent(op trace.Operation, config *VirtualDiskConfig) (*object.DatastorePath, error) {
	defer trace.End(trace.Begin(config.DatastoreURI.String()))

	info, err := m.vdMngr.QueryVirtualDiskInfo(op, config.DatastoreURI.String(), m.vm.Datacenter, true)
	if err != nil {
		op.Errorf("Error querying parents (%s): %s", config.DatastoreURI, err.Error())
		return nil, err
	}

	// the last elem in the info list is the disk we just looked up.
	p := info[len(info)-1]

	if p.Parent != "" {
		ppth, err := datastore.PathFromString(p.Parent)
		if err != nil {
			op.Errorf("Error converting parent to datastore URI (%s): %s", p.Parent, err)
			return nil, err
		}
		return ppth, nil
	}

	// no parent
	return nil, nil
}

// TODO(FA) this doesn't work since delta disks get set with `deletable =
// false` when they become parents.  This needs some thought and will require
// some answers from a larger context.
//func (m *DiskManager) Delete(ctx context.Context, d *VirtualDisk) error {
//	defer trace.End(trace.Begin(d.DatastoreURI))
//
//	log.Infof("Deleting %s", d.DatastoreURI)
//
//	d.l.Lock()
//	defer d.l.Unlock()
//
//	if d.isAttached() {
//		return fmt.Errorf("cannot delete %s, still attached (%s)", d.DatastoreURI, d.devicePath)
//	}
//
//	// TODO(FA) Check if disk is a parent.
//
//	vdm := object.NewVirtualDiskManager(m.vm.Client())
//	task, err := vdm.DeleteVirtualDisk(ctx, d.DatastoreURI, nil)
//	if err != nil {
//		return err
//	}
//
//	err = task.Wait(ctx)
//	if err != nil {
//		return errors.Trace(err)
//	}
//
//	return nil
// }

// Attach attempts to attach a virtual disk
func (m *Manager) attach(op trace.Operation, config *VirtualDiskConfig) error {
	defer trace.End(trace.Begin(""))

	disk := m.toSpec(config)

	deviceList := object.VirtualDeviceList{}
	deviceList = append(deviceList, disk)

	changeSpec, err := deviceList.ConfigSpec(types.VirtualDeviceConfigSpecOperationAdd)
	if err != nil {
		return err
	}

	machineSpec := types.VirtualMachineConfigSpec{}
	machineSpec.DeviceChange = append(machineSpec.DeviceChange, changeSpec...)

	// ensure we abide by max attached disks limits
	m.maxAttached <- true

	m.mu.Lock()
	defer m.mu.Unlock()

	// make sure the op is still valid as the above line could block for a long time
	select {
	case <-op.Done():
		return op.Err()
	default:
	}

	_, err = m.vm.WaitForResult(op, func(ctx context.Context) (tasks.Task, error) {
		t, er := m.vm.Reconfigure(ctx, machineSpec)

		if t != nil {
			op.Debugf("Attach reconfigure task=%s", t.Reference())
		}

		return t, er
	})

	if err != nil {
		select {
		case <-m.maxAttached:
		default:
		}

		op.Errorf("vmdk storage driver failed to attach disk: %s", errors.ErrorStack(err))
		return errors.Trace(err)
	}

	return nil
}

// Detach attempts to detach a virtual disk
func (m *Manager) Detach(op trace.Operation, config *VirtualDiskConfig) error {
	defer trace.End(trace.Begin(""))

	// we have to hold the cache lock until we're done deleting the cache entry
	// or until we know we're not going to delete the entry
	m.disksLock.Lock()
	defer m.disksLock.Unlock()

	d, err := NewVirtualDisk(op, config, m.Disks)
	if err != nil {
		return errors.Trace(err)
	}

	d.l.Lock()
	defer d.l.Unlock()

	count := d.attachedRefs.Decrement()
	op.Debugf("decremented attach count for %s: %d", d.DatastoreURI, count)
	if count > 0 {
		return nil
	}

	if err := d.canBeDetached(); err != nil {
		op.Errorf("disk needs to be detached but is still in use: %s", err)
		return errors.Trace(err)
	}

	op.Infof("Detaching disk %s", d.DevicePath)

	disk, err := findDiskByFilename(op, m.vm, d.DatastoreURI.String(), d.IsPersistent())
	if err != nil {
		return errors.Trace(err)
	}

	if err = m.detach(op, disk); err != nil {
		op.Errorf("detach for %s failed with %s", d.DevicePath, errors.ErrorStack(err))
		return errors.Trace(err)
	}

	// this deletes the disk from the disk cache
	d.setDetached(op, m.Disks)

	return nil
}

func (m *Manager) DetachAll(op trace.Operation) error {
	defer trace.End(trace.Begin(""))

	disks, err := findAllDisks(op, m.vm)
	if err != nil {
		return err
	}

	for _, disk := range disks {
		if err2 := m.detach(op, disk); err != nil {
			op.Errorf("error detaching disk: %s", err2.Error())
			// return the last error on the return of this function
			err = err2
			// if we failed here that means we have a disk attached, ensure we abide by max attached disks limits
			m.maxAttached <- true
		}
	}

	return err
}

func (m *Manager) detach(op trace.Operation, disk *types.VirtualDisk) error {
	config := []types.BaseVirtualDeviceConfigSpec{
		&types.VirtualDeviceConfigSpec{
			Device:    disk,
			Operation: types.VirtualDeviceConfigSpecOperationRemove,
		},
	}

	spec := types.VirtualMachineConfigSpec{}
	spec.DeviceChange = config

	m.mu.Lock()
	defer m.mu.Unlock()

	_, err := m.vm.WaitForResult(op, func(ctx context.Context) (tasks.Task, error) {
		t, er := m.vm.Reconfigure(ctx, spec)

		if t != nil {
			op.Debugf("Detach reconfigure task=%s", t.Reference())
		}

		return t, er
	})

	if err == nil {
		select {
		case <-m.maxAttached:
		default:
		}
	}

	return err
}

func (m *Manager) devicePathByURI(op trace.Operation, datastoreURI *object.DatastorePath, persistent bool) (string, error) {
	disk, err := findDiskByFilename(op, m.vm, datastoreURI.String(), persistent)
	if err != nil {
		op.Errorf("findDisk failed for %s with %s", datastoreURI.String(), errors.ErrorStack(err))
		return "", errors.Trace(err)
	}

	sysPath := fmt.Sprintf(m.byPathFormat, *disk.UnitNumber)

	return sysPath, nil
}

// AttachAndMount creates and attaches a vmdk as a non-persistent disk, mounts it, and returns the mount path.
func (m *Manager) AttachAndMount(op trace.Operation, datastoreURI *object.DatastorePath, persistent bool) (string, error) {
	var config *VirtualDiskConfig

	op.Infof("Attach/Mount %s", datastoreURI.String())

	if !persistent {
		config = NewNonPersistentDisk(datastoreURI)
	} else {
		config = NewPersistentDisk(datastoreURI)
	}

	d, err := m.CreateAndAttach(op, config)
	if err != nil {
		return "", err
	}

	// don't update access time - that would cause the diff operation to mutate the filesystem
	opts := []string{"noatime"}

	if !persistent {
		opts = append(opts, "ro")
	}

	return d.Mount(op, opts)

}

// UnmountAndDetach unmounts and detaches a disk, subsequently cleaning the mount path
func (m *Manager) UnmountAndDetach(op trace.Operation, datastoreURI *object.DatastorePath, persistent bool) error {
	var config *VirtualDiskConfig

	if !persistent {
		config = NewNonPersistentDisk(datastoreURI)
	} else {
		config = NewPersistentDisk(datastoreURI)
	}

	d, err := NewVirtualDisk(op, config, m.Disks)
	if err != nil {
		return err
	}

	op.Infof("Unmount and Detach %s:%s", d.mountPath, d.DatastoreURI)

	err = d.Unmount(op)
	derr := m.Detach(op, config)

	if err != nil || derr != nil {
		op.Errorf("Error during unmount or detach, unmount: %s, detach: %s", err, derr)
		// prioritize first error
		if err == nil {
			err = derr
		}
	}

	return err
}

func (m *Manager) InUse(op trace.Operation, config *VirtualDiskConfig, filter func(vm *mo.VirtualMachine) bool) ([]*vm.VirtualMachine, error) {
	defer trace.End(trace.Begin(""))

	mngr := view.NewManager(m.vm.Vim25())

	// Create view of VirtualMachine objects under the VCH's resource pool
	view2, err := mngr.CreateContainerView(op, m.vm.Session.Pool.Reference(), []string{"VirtualMachine"}, true)
	if err != nil {
		op.Errorf("failed to create view: %s", err)
		return nil, err
	}
	defer view2.Destroy(op)

	var mos []mo.VirtualMachine
	// Retrieve needed properties of all machines under this view
	err = view2.Retrieve(op, []string{"VirtualMachine"}, []string{"name", "config.hardware", "runtime.powerState"}, &mos)
	if err != nil {
		return nil, err
	}

	var vms []*vm.VirtualMachine
	// iterate over them to see whether they have the disk we want
	for i := range mos {
		mo := mos[i]
		op.Debugf("Working on vm %q", mo.Name)

		if !filter(&mo) {
			op.Debugf("Filtering out vm %q", mo.Name)
			continue
		}

		for _, device := range mo.Config.Hardware.Device {
			label := device.GetVirtualDevice().DeviceInfo.GetDescription().Label
			db := device.GetVirtualDevice().Backing
			if db == nil {
				continue
			}

			switch t := db.(type) {
			case types.BaseVirtualDeviceFileBackingInfo:
				if config.DatastoreURI.String() == t.GetVirtualDeviceFileBackingInfo().FileName {
					op.Infof("Found active user of target disk %s: %q", label, mo.Name)
					vms = append(vms, vm.NewVirtualMachine(context.Background(), m.vm.Session, mo.Reference()))
				}
			default:
			}
		}
	}
	return vms, nil
}

func (m *Manager) DiskFinder(op trace.Operation, filter func(p string) bool) (string, error) {
	defer trace.End(trace.Begin(""))

	mngr := view.NewManager(m.vm.Vim25())

	// Create view of VirtualMachine objects under the VCH's resource pool
	view2, err := mngr.CreateContainerView(op, m.vm.Session.Pool.Reference(), []string{"VirtualMachine"}, true)
	if err != nil {
		op.Errorf("failed to create view: %s", err)
		return "", err
	}
	defer view2.Destroy(op)

	var mos []mo.VirtualMachine
	// Retrieve needed properties of all machines under this view
	err = view2.Retrieve(op, []string{"VirtualMachine"}, []string{"name", "config.hardware", "runtime.powerState"}, &mos)
	if err != nil {
		return "", err
	}

	// iterate over them to see whether they have the disk we want
	for i := range mos {
		mo := mos[i]

		op.Debugf("Working on vm %q", mo.Name)

		// observed empty fields here when copying to all 14 volumes on a cVM so being paranoid
		if mo.Config == nil || mo.Config.Hardware.Device == nil {
			op.Warnf("Skipping disk presence check for %q: failed to retrieve vm config", mo.Name)
			continue
		}

		for _, device := range mo.Config.Hardware.Device {
			label := device.GetVirtualDevice().DeviceInfo.GetDescription().Label
			db := device.GetVirtualDevice().Backing
			if db == nil {
				continue
			}

			switch t := db.(type) {
			case types.BaseVirtualDeviceFileBackingInfo:
				diskPath := t.GetVirtualDeviceFileBackingInfo().FileName
				if filter(diskPath) {
					op.Infof("Found disk matching filter: (label: %s), %q", label, diskPath)
					return diskPath, nil
				}
			default:
			}
		}
	}
	return "", errors.New("Not found")
}

func (m *Manager) Owners(op trace.Operation, url *url.URL, filter func(vm *mo.VirtualMachine) bool) ([]*vm.VirtualMachine, error) {
	dsPath, err := datastore.PathFromString(url.Path)
	if err != nil {
		return nil, err
	}

	return m.InUse(op, NewPersistentDisk(dsPath), filter)
}
