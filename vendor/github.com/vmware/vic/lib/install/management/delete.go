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

package management

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/vic/lib/config"
	"github.com/vmware/vic/pkg/errors"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/vsphere/compute"
	"github.com/vmware/vic/pkg/vsphere/tasks"
	"github.com/vmware/vic/pkg/vsphere/vm"
)

type DeleteContainers int

const (
	AllContainers DeleteContainers = iota
	PoweredOffContainers
)

type DeleteVolumeStores int

const (
	AllVolumeStores DeleteVolumeStores = iota
	NoVolumeStores
)

func (d *Dispatcher) DeleteVCH(conf *config.VirtualContainerHostConfigSpec, containers *DeleteContainers, volumeStores *DeleteVolumeStores) error {
	defer trace.End(trace.Begin(conf.Name, d.op))

	var errs []string

	var err error
	var vmm *vm.VirtualMachine

	if vmm, err = d.findApplianceByID(conf); err != nil {
		return err
	}
	if vmm == nil {
		return nil
	}
	d.appliance = vmm

	d.parentResourcepool, err = d.getComputeResource(vmm, conf)
	if err != nil {
		d.op.Error(err)
		if !d.force {
			d.op.Infof("Specify --force to force delete")
			return err
		}
		// Can't find the RP VCH was created in to delete cVMs, continue anyway
		d.op.Warnf("No container VMs found, but proceeding with delete of VCH due to --force")
		err = nil
	}

	// Proceed to delete containers.
	if d.parentResourcepool != nil {
		if err = d.DeleteVCHInstances(conf, containers); err != nil {
			d.op.Error(err)
			if !d.force {
				// if container delete failed, do not remove anything else
				d.op.Infof("Specify --force to force delete")
				return err
			}
			d.op.Warnf("Proceeding with delete of VCH due to --force")
			err = nil
		}
	}

	if err = d.deleteImages(conf); err != nil {
		errs = append(errs, err.Error())
	}

	d.deleteVolumeStoreIfForced(conf, volumeStores) // logs errors but doesn't ever bail out if it has an issue

	if err = d.deleteNetworkDevices(vmm, conf); err != nil {
		errs = append(errs, err.Error())
	}
	if err = d.removeNetwork(conf); err != nil {
		errs = append(errs, err.Error())
	}
	if len(errs) > 0 {
		// stop here, leave vch appliance there for next time delete
		return errors.New(strings.Join(errs, "\n"))
	}

	err = d.deleteVM(vmm, true)
	if err != nil {
		d.op.Debugf("Error deleting appliance VM %s", err)
		return err
	}

	// delete the VCH folder
	d.deleteFolder(d.session.VCHFolder)

	defaultrp, err := d.session.Cluster.ResourcePool(d.op)
	if err != nil {
		return err
	}

	if d.parentResourcepool != nil && d.parentResourcepool.Reference() == defaultrp.Reference() {
		d.op.Warnf("VCH resource pool is cluster default pool - skipping delete")
		return nil
	}

	if err = d.destroyResourcePoolIfEmpty(conf); err != nil {
		d.op.Warnf("VCH resource pool is not removed: %s", err)
	}

	if err = d.deleteVMGroupIfUsed(conf); err != nil {
		d.op.Warnf("VCH DRS VM group is not removed: %s", err)
	}

	return nil
}

func (d *Dispatcher) getComputeResource(vmm *vm.VirtualMachine, conf *config.VirtualContainerHostConfigSpec) (*compute.ResourcePool, error) {
	var rpRef types.ManagedObjectReference
	var err error

	if len(conf.ComputeResources) == 0 {
		err = errors.Errorf("Cannot find compute resource from configuration")
		return nil, err
	}
	rpRef = conf.ComputeResources[len(conf.ComputeResources)-1]

	ref, err := d.session.Finder.ObjectReference(d.op, rpRef)
	if err != nil {
		err = errors.Errorf("Failed to get VCH resource pool %q: %s", rpRef, err)
		return nil, err
	}
	switch ref.(type) {
	case *object.VirtualApp:
	case *object.ResourcePool:
		//		ok
	default:
		err = errors.Errorf("Unsupported compute resource %q", rpRef)
		return nil, err
	}

	rp := compute.NewResourcePool(d.op, d.session, ref.Reference())
	return rp, nil
}

func (d *Dispatcher) getImageDatastore(vmm *vm.VirtualMachine, conf *config.VirtualContainerHostConfigSpec, force bool) (*object.Datastore, error) {
	var err error
	if conf == nil || len(conf.ImageStores) == 0 {
		if !force {
			err = errors.Errorf("Cannot find image stores from configuration")
			return nil, err
		}
		d.op.Debug("Cannot find image stores from configuration; attempting to find from vm datastore")
		dss, err := vmm.DatastoreReference(d.op)
		if err != nil {
			return nil, errors.Errorf("Failed to query vm datastore: %s", err)
		}
		if len(dss) == 0 {
			return nil, errors.New("No VCH datastore found, cannot continue")
		}
		ds, err := d.session.Finder.ObjectReference(d.op, dss[0])
		if err != nil {
			return nil, errors.Errorf("Failed to search vm datastore %s: %s", dss[0], err)
		}
		return ds.(*object.Datastore), nil
	}
	ds, err := d.session.Finder.Datastore(d.op, conf.ImageStores[0].Host)
	if err != nil {
		err = errors.Errorf("Failed to find image datastore %q", conf.ImageStores[0].Host)
		return nil, err
	}
	return ds, nil
}

// detach all VMDKs attached to vm
func (d *Dispatcher) detachAttachedDisks(v *vm.VirtualMachine) error {
	devices, err := v.Device(d.op)
	if err != nil {
		d.op.Debugf("Couldn't find any devices to detach: %s", err.Error())
		return nil
	}

	disks := devices.SelectByType(&types.VirtualDisk{})
	if disks == nil {
		// nothing attached
		d.op.Debug("No disks found attached to VM")
		return nil
	}

	config := []types.BaseVirtualDeviceConfigSpec{}
	for _, disk := range disks {
		config = append(config,
			&types.VirtualDeviceConfigSpec{
				Device:    disk,
				Operation: types.VirtualDeviceConfigSpecOperationRemove,
			})
	}

	op := trace.NewOperation(d.op, "detach disks before delete")
	_, err = v.WaitForResult(op,
		func(ctx context.Context) (tasks.Task, error) {
			t, er := v.Reconfigure(ctx,
				types.VirtualMachineConfigSpec{DeviceChange: config})
			if t != nil {
				op.Debugf("Detach reconfigure task=%s", t.Reference())
			}
			return t, er
		})

	return err
}

// DeleteVCHInstances will delete all containers in the target resource pool or folder.  Additionally, it will detach
// disks of the target VCH
func (d *Dispatcher) DeleteVCHInstances(conf *config.VirtualContainerHostConfigSpec, containers *DeleteContainers) error {
	defer trace.End(trace.Begin(conf.Name, d.op))

	deletePoweredOnContainers := d.force || (containers != nil && *containers == AllContainers)
	ignoreFailureToFindImageStores := d.force

	d.op.Info("Removing VMs")

	// serializes access to errs
	var mu sync.Mutex
	var errs []string

	var err error
	var children []*vm.VirtualMachine

	vchFolder, err := d.appliance.Folder(d.op)
	if err != nil {
		d.op.Errorf("Failed to obtain the VCH folder: %s", err)
		return fmt.Errorf("Failed to obtain the VCH folder. See vic-machine.log for more details. ")
	}
	// set the VCH Folder on session
	d.session.VCHFolder = vchFolder
	// if we have a vchFolder and it's not the VMFolder then gather the children via the folder
	if vchFolder != nil && vchFolder.Reference() != d.session.VMFolder.Reference() {
		// vch parent inventory folder exists, get the children from it.
		d.op.Debugf("Finding children in VCH Folder: %s", vchFolder.InventoryPath)
		folderChildren, err := vchFolder.Children(d.op)
		if err != nil {
			return err
		}

		for _, child := range folderChildren {
			vmObj, ok := child.(*object.VirtualMachine)
			if ok {
				childVM := vm.NewVirtualMachine(d.op, d.session, vmObj.Reference())
				// The destroy method is disabled for all vic created vms so we need to enable
				cErr := childVM.EnableDestroy(d.op)
				if cErr != nil {
					d.op.Debugf("Unable to enable the destroy task on vm(%s): due to %s", childVM.InventoryPath, cErr)
				}
				children = append(children, childVM)
			}
		}
	} else {
		// Find the children in the RP
		d.op.Debugf("Finding children in VCH resource pool")
		if children, err = d.parentResourcepool.GetChildrenVMs(d.op); err != nil {
			return err
		}
	}

	if d.session.Datastore, err = d.getImageDatastore(d.appliance, conf, ignoreFailureToFindImageStores); err != nil {
		return err
	}

	var wg sync.WaitGroup
	for _, child := range children {
		//Leave VCH appliance there until everything else is removed, cause it has VCH configuration. Then user could retry delete in case of any failure.
		ok, err := d.isVCH(child)
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}

		if ok {
			// Do not delete a VCH in the target RP if it is not the target VCH
			if child.Reference() != d.appliance.Reference() {
				d.op.Debugf("Skipping VCH in the resource pool that is not the targeted VCH: %s", child)
				continue
			}

			// child is the target vch; detach all attached disks so later removal of images is successful
			if err = d.detachAttachedDisks(child); err != nil {
				errs = append(errs, err.Error())
			}
			continue
		}

		ok, err = d.isContainerVM(child)
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}
		if !ok {
			d.op.Debugf("Skipping VM in the resource pool that is not a container VM: %s", child)
			continue
		}

		wg.Add(1)
		go func(child *vm.VirtualMachine) {
			name, err := child.ObjectName(d.op)
			defer wg.Done()
			if err = d.deleteVM(child, deletePoweredOnContainers); err != nil {
				mu.Lock()
				errs = append(errs, err.Error())
				mu.Unlock()
			}

			if name != "" {
				d.op.Debugf("Successfully deleted container: %s", name)
			} else {
				d.op.Debugf("Successfully deleted container: %q", child.Reference())
			}
		}(child)
	}
	wg.Wait()

	if len(errs) > 0 {
		d.op.Debugf("Error deleting container VMs %s", errs)
		return errors.New(strings.Join(errs, "\n"))
	}

	return nil
}

func (d *Dispatcher) deleteNetworkDevices(vmm *vm.VirtualMachine, conf *config.VirtualContainerHostConfigSpec) error {
	defer trace.End(trace.Begin(conf.Name, d.op))

	d.op.Info("Removing appliance VM network devices")

	power, err := vmm.PowerState(d.op)
	if err != nil {
		d.op.Errorf("Failed to get vm power status %q: %s", vmm.Reference(), err)
		return err

	}
	if power != types.VirtualMachinePowerStatePoweredOff {
		if _, err = vmm.WaitForResult(d.op, func(ctx context.Context) (tasks.Task, error) {
			return vmm.PowerOff(ctx)
		}); err != nil {
			d.op.Errorf("Failed to power off existing appliance for %s", err)
			return err
		}
	}

	devices, err := d.networkDevices(vmm)
	if err != nil {
		d.op.Errorf("Unable to get network devices: %s", err)
		return err
	}

	if len(devices) == 0 {
		d.op.Info("No network device attached")
		return nil
	}
	// remove devices
	return vmm.RemoveDevice(d.op, false, devices...)
}

func (d *Dispatcher) networkDevices(vmm *vm.VirtualMachine) ([]types.BaseVirtualDevice, error) {
	defer trace.End(trace.Begin("", d.op))

	var err error
	vmDevices, err := vmm.Device(d.op)
	if err != nil {
		d.op.Errorf("Failed to get vm devices for appliance: %s", err)
		return nil, err
	}
	var devices []types.BaseVirtualDevice
	for _, device := range vmDevices {
		if _, ok := device.(types.BaseVirtualEthernetCard); ok {
			devices = append(devices, device)
		}
	}
	return devices, nil
}
