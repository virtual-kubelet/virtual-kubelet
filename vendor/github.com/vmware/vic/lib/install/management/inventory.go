// Copyright 2018 VMware, Inc. All Rights Reserved.
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
	"path"
	"strings"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/vmware/vic/lib/config"
	"github.com/vmware/vic/lib/config/executor"
	"github.com/vmware/vic/lib/portlayer/util"
	"github.com/vmware/vic/lib/spec"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/uid"
	"github.com/vmware/vic/pkg/vsphere/compute"
	"github.com/vmware/vic/pkg/vsphere/extraconfig"
	"github.com/vmware/vic/pkg/vsphere/extraconfig/vmomi"
	"github.com/vmware/vic/pkg/vsphere/tasks"
	"github.com/vmware/vic/pkg/vsphere/vm"
)

// inventoryUpdate updates the vSphere inventory structure
// for the VCH and containerVMs
//
// If the VCH resides in the VMFolder then a new folder will
// be created and the VCH and all Containers will be moved into the
// new folder.  If the VCH already resides in a folder other than
// the VMFolder, then no action will be taken.
func (d *Dispatcher) inventoryUpdate(name string) error {
	defer trace.End(trace.Begin(name, d.op))

	d.op.Debugf("Updating VCH Inventory: %s", name)
	var err error

	// get the current appliance folder
	currentFolder, err := d.appliance.Folder(d.op)
	if err != nil {
		d.op.Errorf("Unable to get current folder:  %s", err)
		return err
	}

	if currentFolder.Reference() != d.session.VMFolder.Reference() {
		// slight chance this could be an upgrade retry and the only
		// thing that failed was renaming the VCH folder.  If the currentFolder
		// is the same name then noop
		d.renameFolder(currentFolder, name)
		d.op.Debugf("No inventory update completed for %s", name)
		return nil
	}

	// Get a slice of MoRefs that are the VCHs containers
	refs, err := d.containerRefs(d.vchPool)
	if err != nil {
		d.op.Errorf("Error getting containers for Inventory Update: %s", err)
		return err
	}

	// To avoid a Folder & VCH Name collison we need to create a temp folder name.
	// Utilizing the portlayer naming convention to ensure the max length isn't exceeded
	template := fmt.Sprintf("%s-%s", config.NameToken.String(), config.IDToken.String())
	cfg := &spec.VirtualMachineConfigSpecConfig{
		ID:   uid.New().Truncate().String(),
		Name: name,
	}
	tempName := util.DisplayName(d.op, cfg, template)
	target, err := d.session.VMFolder.CreateFolder(d.op, tempName)
	if err != nil {
		d.op.Errorf("Unable to create target folder during inventory update: %s", err)
		return err
	}
	// best effort a cleaning up any orphaned temp folders
	defer d.folderCleanup(name, fmt.Sprintf("%s-", name))

	// if we have containers move them into the new VCH Folder
	if len(refs) > 0 {
		// Move containers into folder
		d.op.Debugf("Moving %d container(s) into the folder", len(refs))
		_, err = tasks.WaitForResultAndRetryIf(d.op, func(op context.Context) (tasks.Task, error) {
			return target.MoveInto(d.op, refs)
		}, tasks.IsTransientError)
		if err != nil {
			d.op.Errorf("Failure moving containers into folder(%s): %s", tempName, err)
			return err
		}
		d.op.Debugf("%d container(s) moved into folder", len(refs))
	}

	// we must move the appliance after the containers due to the following factors:
	//
	// The transaction boundary for folder.MoveInto is at the object level.  A failure
	// would potentially leave containers outside of the folder, which has consequences once
	// the appliance is in the folder.
	//
	// On startup a VCH determines where to find it's containers based on it's current folder.
	// If that folder is the VMFolder it will find them via resource pools, but if it's
	// any other folder it will only look in that folder.  Thus we only move the VCH appliance
	// when the containers have been relocated.  If the appliance move fails we can recover on
	// the next attempted upgrade or if no further upgrades are attempted vic will continue to
	// function without issue
	d.op.Debugf("Moving %s into the folder", name)
	_, err = tasks.WaitForResultAndRetryIf(d.op, func(op context.Context) (tasks.Task, error) {
		return target.MoveInto(d.op, []types.ManagedObjectReference{d.appliance.Reference()})
	}, tasks.IsTransientError)
	if err != nil {
		d.op.Errorf("Failure moving VCH(%s) into folder(%s): %s", name, tempName, err)
		return err
	}
	d.op.Debugf("%s moved into folder", name)
	//now rename the temp folder
	d.renameFolder(target, name)
	return nil
}

// renameFolder renames the the provided folder to the specified name
func (d *Dispatcher) renameFolder(target *object.Folder, name string) error {
	targetOriginal := target.Name()
	if targetOriginal == name {
		//noop
		return nil
	}

	_, err := tasks.WaitForResultAndRetryIf(d.op, func(op context.Context) (tasks.Task, error) {
		return target.Rename(d.op, name)
	}, tasks.IsTransientError)
	if err != nil {
		d.op.Errorf("Unable to rename folder(%s) to %s during inventory update: %s", targetOriginal, name, err)
		return err
	}
	d.op.Debugf("Folder renamed from %s to %s", targetOriginal, name)
	return nil
}

// folderCleanup searches for folders with a name resembling a temporary folder during
// inventoryUpdate.  This will remove empty folders that were not deleted during a failed upgrade.
func (d *Dispatcher) folderCleanup(name string, tempPattern string) {
	// check for any orphaned upgrade folders
	folders, err := d.session.Finder.FolderList(d.op, path.Join(d.session.VMFolder.InventoryPath, "*"))
	if err != nil {
		d.op.Debugf("Error searching for orphaned upgrade folders: %s", err)
		return
	}
	d.op.Debugf("Searching %d folders for partial name(%s) match", len(folders), tempPattern)
	for _, f := range folders {
		if strings.Contains(f.Name(), tempPattern) {
			d.op.Debugf("Found Orphaned Folder: %s", f.Name())
			// delete if it's not empty
			d.deleteFolder(f)
		}
	}
}

// containerRefs returns an array of ManagedObjectRefrences that are the containerVMs for the pool
func (d *Dispatcher) containerRefs(pool *object.ResourcePool) ([]types.ManagedObjectReference, error) {
	defer trace.End(trace.Begin(pool.InventoryPath, d.op))
	// slice for validated containers
	var containers []types.ManagedObjectReference
	// find the vms for this resource pool
	vms, err := d.children(pool)
	if err != nil {
		return nil, err
	}
	// determine which are containers
	for _, vm := range vms {
		if d.isContainer(vm) {
			d.op.Debugf("Found Container: %s", vm.Summary.Config.Name)
			containers = append(containers, vm.Reference())
		}
	}
	d.op.Debugf("Found %d containers", len(containers))
	return containers, nil
}

// children returns an array of VirtualMachines with properties populated
func (d *Dispatcher) children(pool *object.ResourcePool) ([]mo.VirtualMachine, error) {
	defer trace.End(trace.Begin(pool.InventoryPath, d.op))

	// Get the vmRefs for this resource pool
	refs, err := compute.VM(d.op, d.session, pool)
	if err != nil {
		err = fmt.Errorf("Error retrieving vms: %s", err)
		return nil, err
	}

	return vm.Attributes(d.op, d.session, refs)
}

func (d *Dispatcher) isContainer(vm mo.VirtualMachine) bool {
	var cfg executor.ExecutorConfig
	// convert extraconfig to map
	info := vmomi.OptionValueMap(vm.Config.ExtraConfig)
	// decode to the executorConfig
	extraconfig.Decode(extraconfig.MapSource(info), &cfg)
	id := uid.Parse(cfg.ID)
	if id == uid.NilUID {
		d.op.Debugf("skipping VM %s: could not parse id", vm.Summary.Config.Name)
		return false
	}
	return true
}
