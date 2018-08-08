// Copyright 2016 VMware, Inc. All Rights Reserved.
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

	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/vic/lib/config"
	"github.com/vmware/vic/lib/install/data"
	"github.com/vmware/vic/pkg/errors"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/vsphere/tasks"
	"github.com/vmware/vic/pkg/vsphere/vm"
)

func (d *Dispatcher) createResourcePool(conf *config.VirtualContainerHostConfigSpec, settings *data.InstallerData) (*object.ResourcePool, error) {
	defer trace.End(trace.Begin("", d.op))

	rp, err := d.session.Finder.ResourcePool(d.op, d.vchPoolPath)
	if err != nil {
		// if we didn't find the resource pool then we will create
		_, ok := err.(*find.NotFoundError)
		if !ok {
			err = errors.Errorf("Failed to query compute resource (%q): %q", d.vchPoolPath, err)
			return nil, err
		}
	} else {
		conf.ComputeResources = append(conf.ComputeResources, rp.Reference())
		return rp, nil
	}

	d.op.Infof("Creating Resource Pool %q", conf.Name)
	resSpec := types.DefaultResourceConfigSpec()
	setResources(&resSpec.CpuAllocation, settings.VCHSize.CPU)
	setResources(&resSpec.MemoryAllocation, settings.VCHSize.Memory)

	rp, err = d.session.Pool.Create(d.op, conf.Name, resSpec)
	if err != nil {
		d.op.Debugf("Failed to create resource pool %q: %s", d.vchPoolPath, err)
		return nil, err
	}

	conf.ComputeResources = append(conf.ComputeResources, rp.Reference())
	return rp, nil
}

// setResources will modify the resourceAllocation spec with the user provided allocation info
func setResources(spec *types.ResourceAllocationInfo, resource types.ResourceAllocationInfo) {
	if resource.Limit != nil {
		// if no limit is requested then set to -1 for unlimited
		if *resource.Limit == int64(0) {
			resource.Limit = types.NewInt64(-1)
		}
		spec.Limit = resource.Limit
	}
	if resource.Reservation != nil {
		spec.Reservation = resource.Reservation
	}
	if resource.Shares != nil {
		// were custom shares specified
		if resource.Shares.Shares != 0 {
			spec.Shares = resource.Shares
		} else {
			// resource shares are zero, so set level to anything except custom
			if resource.Shares.Level != "custom" {
				spec.Shares.Level = resource.Shares.Level
			}
		}

	}
	if resource.ExpandableReservation != nil {
		spec.ExpandableReservation = resource.ExpandableReservation
	}
}

func (d *Dispatcher) destroyResourcePoolIfEmpty(conf *config.VirtualContainerHostConfigSpec) error {
	defer trace.End(trace.Begin("", d.op))

	d.op.Infof("Removing Resource Pool %q", conf.Name)

	if d.parentResourcepool == nil {
		d.op.Warn("Did not find parent VCH resource pool")
		return nil
	}
	var vms []*vm.VirtualMachine
	var err error
	if vms, err = d.parentResourcepool.GetChildrenVMs(d.op); err != nil {
		err = errors.Errorf("Unable to get children vm of resource pool %q: %s", d.parentResourcepool.Name(), err)
		return err
	}
	if len(vms) != 0 {
		err = errors.Errorf("Resource pool is not empty: found %d vms under %q", len(vms), d.parentResourcepool.Name())
		return err
	}
	if _, err := tasks.WaitForResult(d.op, func(ctx context.Context) (tasks.Task, error) {
		return d.parentResourcepool.Destroy(ctx)
	}); err != nil {
		return err
	}
	return nil
}

func (d *Dispatcher) findResourcePool(path string) (*object.ResourcePool, error) {
	defer trace.End(trace.Begin(path, d.op))
	rp, err := d.session.Finder.ResourcePool(d.op, path)
	if err != nil {
		_, ok := err.(*find.NotFoundError)
		if !ok {
			err = errors.Errorf("Failed to query resource pool %q: %s", path, err)
			return nil, err
		}
		return nil, nil
	}
	return rp, nil
}

func (d *Dispatcher) getPoolResourceSettings(pool *object.ResourcePool) (*config.Resources, error) {
	var p mo.ResourcePool
	ps := []string{"config.cpuAllocation", "config.memoryAllocation"}

	if err := pool.Properties(d.op, pool.Reference(), ps, &p); err != nil {
		return nil, err
	}

	res := &config.Resources{
		CPU:    p.Config.CpuAllocation,
		Memory: p.Config.MemoryAllocation,
	}
	return res, nil
}

func updateResourcePoolConfig(ctx context.Context, pool *object.ResourcePool, name string, size *config.Resources) error {
	op := trace.FromContext(ctx, "updateResourcePoolConfig")
	defer trace.End(trace.Begin(fmt.Sprintf("cpu %#v, memory: %#v", size.CPU, size.Memory), op))
	resSpec := types.DefaultResourceConfigSpec()
	// update with user provided configuration
	setResources(&resSpec.CpuAllocation, size.CPU)
	setResources(&resSpec.MemoryAllocation, size.Memory)
	return pool.UpdateConfig(op, name, &resSpec)
}
