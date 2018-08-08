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
	"fmt"
	"path"

	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/vic/lib/config"
	"github.com/vmware/vic/lib/install/validate"
	"github.com/vmware/vic/lib/migration"
	"github.com/vmware/vic/pkg/errors"
	"github.com/vmware/vic/pkg/retry"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/vsphere/compute"
	"github.com/vmware/vic/pkg/vsphere/extraconfig"
	"github.com/vmware/vic/pkg/vsphere/extraconfig/vmomi"
	"github.com/vmware/vic/pkg/vsphere/tasks"
	"github.com/vmware/vic/pkg/vsphere/vm"
)

const (
	vchIDType = "VirtualMachine"
)

func (d *Dispatcher) NewVCHFromID(id string) (*vm.VirtualMachine, error) {
	defer trace.End(trace.Begin(id, d.op))

	var err error

	moref := &types.ManagedObjectReference{
		Type:  vchIDType,
		Value: id,
	}
	ref, err := d.session.Finder.ObjectReference(d.op, *moref)
	if err != nil {
		if !isManagedObjectNotFoundError(err) {
			err = errors.Errorf("Failed to query appliance (%q): %s", moref, err)
			return nil, err
		}
		d.op.Debug("Appliance is not found")
		return nil, fmt.Errorf("id %q could not be found", id)
	}
	ovm, ok := ref.(*object.VirtualMachine)
	if !ok {
		d.op.Errorf("Failed to find VM %q: %s", moref, err)
		return nil, err
	}
	d.appliance = vm.NewVirtualMachine(d.op, d.session, ovm.Reference())

	// check if it's VCH
	if ok, err = d.isVCH(d.appliance); err != nil {
		d.op.Error(err)
		return nil, err
	}
	if !ok {
		err = errors.Errorf("Not a VCH")
		d.op.Error(err)
		return nil, err
	}
	d.vchPool, err = d.appliance.ResourcePool(d.op)
	if err != nil {
		d.op.Errorf("Failed to get VM parent resource pool: %s", err)
		return nil, err
	}

	rp := compute.NewResourcePool(d.op, d.session, d.vchPool.Reference())
	if d.session.Cluster, err = rp.GetCluster(d.op); err != nil {
		d.op.Debugf("Unable to get the cluster for the VCH's resource pool: %s", err)
	}
	d.InitDiagnosticLogsFromVCH(d.appliance)
	return d.appliance, nil
}

func (d *Dispatcher) NewVCHFromComputePath(computePath string, name string, v *validate.Validator) (*vm.VirtualMachine, error) {
	defer trace.End(trace.Begin(fmt.Sprintf("path %q, name %q", computePath, name), d.op))

	var err error

	parent, err := v.ResourcePoolHelper(d.op, computePath)
	if err != nil {
		return nil, err
	}
	d.vchPoolPath = path.Join(parent.InventoryPath, name)
	if d.isVC {
		vapp, err := d.findVirtualApp(d.vchPoolPath)
		if err != nil {
			d.op.Errorf("Failed to get VCH virtual app %q: %s", d.vchPoolPath, err)
			return nil, err
		}
		if vapp != nil {
			d.vchPool = vapp.ResourcePool
		}
	}
	if d.vchPool == nil {
		d.vchPool, err = d.session.Finder.ResourcePool(d.op, d.vchPoolPath)
		if err != nil {
			// we didn't find the ResourcePool with a name matching the appliance, so
			// lets look for just the resource pool -- this could be at the cluster level
			d.vchPoolPath = parent.InventoryPath
			d.vchPool, err = d.session.Finder.ResourcePool(d.op, d.vchPoolPath)
			if err != nil {
				d.op.Errorf("Failed to find VCH resource pool %q: %s", d.vchPoolPath, err)
				return nil, err
			}
		}
	}

	// creating a pkg/vsphere resource pool for use of convenience method
	rp := compute.NewResourcePool(d.op, d.session, d.vchPool.Reference())

	if d.session.Cluster, err = rp.GetCluster(d.op); err != nil {
		d.op.Debugf("Unable to get the cluster for the VCH's resource pool: %s", err)
	}

	if d.appliance, err = rp.GetChildVM(d.op, name); err != nil {
		d.op.Errorf("Failed to get VCH VM: %s", err)
		return nil, err
	}
	if d.appliance == nil {
		err = errors.Errorf("Didn't find VM %q in resource pool %q", name, rp.Reference())
		d.op.Error(err)
		return nil, err
	}
	d.appliance.InventoryPath = path.Join(d.vchPoolPath, name)

	// check if it's VCH
	var ok bool
	if ok, err = d.isVCH(d.appliance); err != nil {
		d.op.Error(err)
		return nil, err
	}
	if !ok {
		err = errors.Errorf("Not a VCH")
		d.op.Error(err)
		return nil, err
	}

	d.InitDiagnosticLogsFromVCH(d.appliance)
	return d.appliance, nil
}

// GetVCHConfig queries VCH configuration and decrypts secret information
func (d *Dispatcher) GetVCHConfig(vm *vm.VirtualMachine) (*config.VirtualContainerHostConfigSpec, error) {
	defer trace.End(trace.Begin("", d.op))

	//this is the appliance vm
	mapConfig, err := vm.FetchExtraConfigBaseOptions(d.op)
	if err != nil {
		err = errors.Errorf("Failed to get VM extra config of %q: %s", vm.Reference(), err)
		d.op.Error(err)
		return nil, err
	}

	kv := vmomi.OptionValueMap(mapConfig)
	vchConfig, err := d.decryptVCHConfig(vm, kv)
	if err != nil {
		err = errors.Errorf("Failed to decode VM configuration %q: %s", vm.Reference(), err)
		d.op.Error(err)
		return nil, err
	}

	if vchConfig.IsCreating() {
		vmRef := vm.Reference()
		vchConfig.SetMoref(&vmRef)
	}
	return vchConfig, nil
}

// GetNoSecretVCHConfig queries vch configure from vm configuration, without decrypting secret information
// this method is used to accommodate old vch version without secret information
func (d *Dispatcher) GetNoSecretVCHConfig(vm *vm.VirtualMachine) (*config.VirtualContainerHostConfigSpec, error) {
	defer trace.End(trace.Begin("", d.op))

	//this is the appliance vm
	mapConfig, err := vm.FetchExtraConfigBaseOptions(d.op)
	if err != nil {
		err = errors.Errorf("Failed to get VM extra config of %q: %s", vm.Reference(), err)
		d.op.Error(err)
		return nil, err
	}

	kv := vmomi.OptionValueMap(mapConfig)
	vchConfig := &config.VirtualContainerHostConfigSpec{}
	extraconfig.Decode(extraconfig.MapSource(kv), vchConfig)

	if vchConfig.IsCreating() {
		vmRef := vm.Reference()
		vchConfig.SetMoref(&vmRef)
	}
	return vchConfig, nil
}

// FetchAndMigrateVCHConfig queries VCH guestinfo, and try to migrate older version data to latest if the data is old
func (d *Dispatcher) FetchAndMigrateVCHConfig(vm *vm.VirtualMachine) (*config.VirtualContainerHostConfigSpec, error) {
	defer trace.End(trace.Begin("", d.op))

	//this is the appliance vm
	mapConfig, err := vm.FetchExtraConfigBaseOptions(d.op)
	if err != nil {
		err = errors.Errorf("Failed to get VM extra config of %q: %s", vm.Reference(), err)
		return nil, err
	}

	kv := vmomi.OptionValueMap(mapConfig)
	newMap, migrated, err := migration.MigrateApplianceConfig(d.op, d.session, kv)
	if err != nil {
		err = errors.Errorf("Failed to migrate config of %q: %s", vm.Reference(), err)
		return nil, err
	}
	if !migrated {
		d.op.Debugf("No need to migrate configuration for %q", vm.Reference())
	}
	return d.decryptVCHConfig(vm, newMap)
}

// SearchVCHs searches for VCHs in one of the following areas:
//
// computePath:  if a compute resource is provided then the search will be limited
// to the resource pools of the compute resources found at the path
//
// datacenter:  if no compute resource is provided then search across all compute
// resources in the sessions datacenter.  This requires the user to specify the datacenter
// via the target flag
//
// all datacenters:  if the other options aren't available then search across every compute
// resource in each vSphere datacenter
func (d *Dispatcher) SearchVCHs(computePath string) ([]*vm.VirtualMachine, error) {
	defer trace.End(trace.Begin(computePath, d.op))
	if computePath != "" {
		return d.search(computePath)
	}
	if d.session.Datacenter != nil {
		return d.search(path.Join(d.session.Datacenter.InventoryPath, "..."))
	}

	dcs, err := d.session.Finder.DatacenterList(d.op, "*")
	if err != nil {
		err = errors.Errorf("Failed to get datacenter list: %s", err)
		return nil, err
	}

	var vchs []*vm.VirtualMachine
	for _, dc := range dcs {
		d.session.Finder.SetDatacenter(dc)
		dcVCHs, err := d.search(path.Join(dc.InventoryPath, "..."))
		if err != nil {
			// we will just warn for now
			d.op.Warnf("Error searching the datacenter(%s): %s", dc.Name(), err)
			continue
		}
		vchs = append(vchs, dcVCHs...)
	}
	return vchs, nil
}

// search finds the compute resources based on the search path and then retrieves
// the resouce pools for that compute resource.  Once a list of resource pools is
// collected search will iterate over the pools and search for VCHs in each pool
func (d *Dispatcher) search(searchPath string) ([]*vm.VirtualMachine, error) {
	defer trace.End(trace.Begin(searchPath, d.op))
	var vchs []*vm.VirtualMachine
	// find compute resources for the search path
	resources, err := d.session.Finder.ComputeResourceList(d.op, searchPath)
	if err != nil {
		if _, ok := err.(*find.NotFoundError); ok {
			return nil, nil
		}
		return nil, err
	}

	// iterate over clusters and search each resourcePool in the cluster
	for _, c := range resources {
		pools, err := d.listResourcePools(c.InventoryPath)
		if err != nil {
			e := fmt.Errorf("Unable to find resource pools for compute(%s): %s", c.Name(), err)
			return nil, e
		}
		// now search the pools for VCHs
		vms := d.searchResourcePools(pools)
		vchs = append(vchs, vms...)
	}
	return vchs, nil
}

// listResourcePool returns a list of all resource pools under a compute path
// The func will rety on an ObjectNotFound error because sometimes it's due to concurrent
// operations on the resource pool
func (d *Dispatcher) listResourcePools(searchPath string) ([]*object.ResourcePool, error) {
	var err error
	var pools []*object.ResourcePool

	// under some circumstances, such as when there is concurrent vic-machine delete operation running in the background,
	// listing resource pools might fail because some VCH pool is being destroyed at the same time.
	// If that happens, we retry and list pools again
	err = retry.Do(func() error {
		pools, err = d.session.Finder.ResourcePoolList(d.op, path.Join(searchPath, "..."))
		if _, ok := err.(*find.NotFoundError); ok {
			return nil
		}
		return err
	}, func(err error) bool {
		return tasks.IsTransientError(d.op, err) || tasks.IsNotFoundError(err)
	})

	return pools, err
}

// searchResourcePools searches for VCHs in the provided list of resource pools
func (d *Dispatcher) searchResourcePools(pools []*object.ResourcePool) []*vm.VirtualMachine {
	var vchs []*vm.VirtualMachine
	// The number of resource pools found for the compute resource determines how
	// to search for any VCHs.  If there is only one pool then look at all VMs in the
	// resource pool for VCH metadata.  If there are multiple pools then look only at VMs
	// that have the same name as the resource pool.
	multiPool := false
	if len(pools) > 1 {
		multiPool = true
	}
	// iterate over the compute resources pools and search for VMs and vApps
	for _, pool := range pools {
		children, err := d.findVCHs(pool, multiPool)
		// #nosec: Errors unhandled.
		if err != nil {
			d.op.Warnf("Failed to get VCH from resource pool %q: %s", pool.InventoryPath, err)
		} else {
			vchs = append(vchs, children...)
		}

		// search for a vApp
		vappPath := path.Join(pool.InventoryPath, "*")
		vapps, err := d.session.Finder.VirtualAppList(d.op, vappPath)
		if err != nil {
			if _, ok := err.(*find.NotFoundError); !ok {
				d.op.Errorf("Failed to query vapp %q: %s", vappPath, err)
			}
		}
		for _, vapp := range vapps {
			children, err := d.findVCHs(vapp.ResourcePool, multiPool)
			if err != nil {
				d.op.Warnf("Failed to get VCH from vApp resource pool %q: %s", pool.InventoryPath, err)
				continue
			}
			vchs = append(vchs, children...)
		}

	}
	return vchs
}

// findVCHs finds any VCH in the specified pool.  If the compute resource has multiple pools, then look
// for any VMs with the same name as the resource pool.  If the compute resource only had a single pool then
// evaluate each VM in the pool for VCH metadata
func (d *Dispatcher) findVCHs(pool *object.ResourcePool, multiPool bool) ([]*vm.VirtualMachine, error) {
	defer trace.End(trace.Begin(pool.InventoryPath, d.op))

	// check if pool itself contains VCH vm.
	var vms []*vm.VirtualMachine
	var vchs []*vm.VirtualMachine
	var err error
	computeResource := compute.NewResourcePool(d.op, d.session, pool.Reference())
	// The compute resource had multiple pools, so the assumption is that any VCH that exists will be the same
	// name as it's resource pool
	if multiPool {
		vm, err := computeResource.GetChildVM(d.op, pool.Name())
		if err != nil {
			return nil, errors.Errorf("Failed to query children VM in resource pool %q: %s", pool.InventoryPath, err)
		}
		if vm != nil {
			vms = append(vms, vm)
		}

	} else {
		// We only had one pool, so we'll look at all the VMs in that pool
		vms, err = computeResource.GetChildrenVMs(d.op)
		if err != nil {
			return nil, errors.Errorf("Failed to query children VM in resource pool %q: %s", pool.InventoryPath, err)
		}
	}
	// iterate over VMs and determine if we've got any VCHs
	for _, v := range vms {
		// override the VM inventory path (which is folder based)
		// for the resource pool path
		v.InventoryPath = path.Join(pool.InventoryPath, v.Name())
		// #nosec: Errors unhandled.
		if ok, _ := d.isVCH(v); ok {
			d.op.Debugf("%q is VCH", v.InventoryPath)
			vchs = append(vchs, v)
		}
	}

	return vchs, nil
}
