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

package compute

import (
	"context"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/vmware/vic/pkg/errors"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/vsphere/session"
	"github.com/vmware/vic/pkg/vsphere/vm"
)

// ResourcePool struct defines the ResourcePool which provides additional
// VIC specific methods over object.ResourcePool as well as keeps some state
type ResourcePool struct {
	*object.ResourcePool

	*session.Session
}

// NewResourcePool returns a New ResourcePool object
func NewResourcePool(ctx context.Context, session *session.Session, moref types.ManagedObjectReference) *ResourcePool {
	return &ResourcePool{
		ResourcePool: object.NewResourcePool(
			session.Vim25(),
			moref,
		),
		Session: session,
	}
}

// VM returns a slice of MoRefs that are the virtual machines of the provided resource pool
func VM(op trace.Operation, session *session.Session, pool *object.ResourcePool) ([]types.ManagedObjectReference, error) {
	var mrp mo.ResourcePool
	err := session.Retrieve(op, []types.ManagedObjectReference{pool.Reference()}, []string{"vm"}, &mrp)
	if err != nil {
		op.Errorf("Error retrieving VMs for resource pool %s: %s", pool.Name(), err)
		return nil, err
	}
	return mrp.Vm, nil
}

// GetChildrenVMs returns a slice of VirtualMachines that are the pools VMs
func (rp *ResourcePool) GetChildrenVMs(op trace.Operation) ([]*vm.VirtualMachine, error) {
	var vms []*vm.VirtualMachine
	refs, err := VM(op, rp.Session, rp.ResourcePool)
	if err != nil {
		return nil, err
	}
	for _, o := range refs {
		v := vm.NewVirtualMachine(op, rp.Session, o)
		vms = append(vms, v)
	}
	return vms, nil
}

// GetChildVM searches the pool for a VM by name and returns a VirtualMachine
func (rp *ResourcePool) GetChildVM(ctx context.Context, name string) (*vm.VirtualMachine, error) {
	op := trace.FromContext(ctx, name)

	searchIndex := object.NewSearchIndex(rp.Vim25())
	child, err := searchIndex.FindChild(op, rp.Reference(), name)
	if err != nil {
		return nil, errors.Errorf("Unable to find VM(%s): %s", name, err.Error())
	}
	if child == nil {
		return nil, nil
	}
	// instantiate the vm object
	return vm.NewVirtualMachine(op, rp.Session, child.Reference()), nil
}

func (rp *ResourcePool) GetCluster(ctx context.Context) (*object.ComputeResource, error) {
	op := trace.FromContext(ctx, rp.Name())

	var err error
	var mrp mo.ResourcePool

	if err = rp.Properties(op, rp.Reference(), []string{"owner"}, &mrp); err != nil {
		op.Errorf("Unable to get cluster of resource pool %s: %s", rp.Name(), err)
		return nil, err
	}

	return object.NewComputeResource(rp.Vim25(), mrp.Owner), nil
}

func (rp *ResourcePool) GetDatacenter(ctx context.Context) (*object.Datacenter, error) {
	op := trace.FromContext(ctx, rp.Name())

	dcRef, err := rp.getLowestAncestor(op, "Datacenter")
	if err != nil || dcRef == nil {
		op.Errorf("Unable to get datacenter ancestor of rp %s: %s", rp.Name(), err)
		return nil, errors.Errorf("Unable to get datacenter ancestor of rp %s: %s", rp.Name(), err)
	}

	return object.NewDatacenter(rp.Vim25(), *dcRef), nil
}

func (rp *ResourcePool) getAncestors(op trace.Operation, inType string) ([]types.ManagedObjectReference, error) {
	client := rp.Session.Vim25()

	ancestors, err := mo.Ancestors(op, client, client.ServiceContent.PropertyCollector, rp.Reference())
	if err != nil {
		op.Errorf("Unable to get ancestors of rp %s: %s", rp.Name(), err)
		return nil, err
	}

	outAncestors := make([]types.ManagedObjectReference, 0, len(ancestors))
	for _, ancestor := range ancestors {
		if ancestor.Self.Type == inType {
			a := ancestor.Self
			outAncestors = append(outAncestors, a)
		}
	}

	return outAncestors, nil
}

func (rp *ResourcePool) getLowestAncestor(op trace.Operation, inType string) (*types.ManagedObjectReference, error) {
	ancestors, err := rp.getAncestors(op, inType)
	if err != nil {
		op.Errorf("Unable to get ancestors of rp %s: %s", rp.Name(), err)
		return nil, err
	}

	if len(ancestors) == 0 {
		return nil, nil
	}

	index := len(ancestors) - 1
	return &ancestors[index], nil
}
