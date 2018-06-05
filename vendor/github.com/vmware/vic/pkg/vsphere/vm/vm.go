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

package vm

import (
	"container/list"
	"context"
	"errors"
	"fmt"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25/methods"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/vmware/vic/pkg/retry"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/vsphere/compute/placement"
	"github.com/vmware/vic/pkg/vsphere/extraconfig/vmomi"
	"github.com/vmware/vic/pkg/vsphere/performance"
	"github.com/vmware/vic/pkg/vsphere/session"
	"github.com/vmware/vic/pkg/vsphere/tasks"
)

const (
	DestroyTask  = "Destroy_Task"
	UpdateStatus = "UpdateInProgress"
)

type InvalidState struct {
	r types.ManagedObjectReference
}

func (i *InvalidState) Error() string {
	return fmt.Sprintf("vm %s is invalid", i.r.String())
}

// VirtualMachine struct defines the VirtualMachine which provides additional
// VIC specific methods over object.VirtualMachine as well as keeps some state
type VirtualMachine struct {
	// TODO: Wrap Internal VirtualMachine struct when we have it
	// *internal.VirtualMachine

	*object.VirtualMachine

	*session.Session

	// fxing is 1 means this vm is fixing for it's in invalid status. 0 means not in fixing status
	fixing int32
}

// Attributes will retrieve the properties for the provided references.  A variadic arg of attribs
// is provided to allow overriding the default properties retrieved
func Attributes(ctx context.Context, session *session.Session, refs []types.ManagedObjectReference, attribs ...string) ([]mo.VirtualMachine, error) {
	defer trace.End(trace.Begin(fmt.Sprintf("populating %d refs", len(refs))))
	var vms []mo.VirtualMachine
	// default attributes to retrieve
	props := []string{"config", "runtime.powerState", "summary"}
	// if attributes provided then use those instead of default
	if len(attribs) > 0 {
		props = attribs
	}
	// retrieve the properties via the session property collector
	err := session.Retrieve(ctx, refs, props, &vms)
	return vms, err
}

// NewVirtualMachine returns a NewVirtualMachine object
func NewVirtualMachine(ctx context.Context, session *session.Session, moref types.ManagedObjectReference) *VirtualMachine {
	vm := NewVirtualMachineFromVM(ctx, session, object.NewVirtualMachine(session.Vim25(), moref))
	// best effort to get the correct path
	if session.Finder != nil {
		e, _ := session.Finder.Element(ctx, moref)
		if e != nil {
			vm.VirtualMachine.InventoryPath = e.Path
		}
	}
	return vm
}

// NewVirtualMachineFromVM returns a NewVirtualMachine object
func NewVirtualMachineFromVM(ctx context.Context, session *session.Session, vm *object.VirtualMachine) *VirtualMachine {
	return &VirtualMachine{
		VirtualMachine: vm,
		Session:        session,
	}
}

// VMPathNameAsURL returns the full datastore path of the VM as a url. The datastore name is in the host
// portion, the path is in the Path field, the scheme is set to "ds"
func (vm *VirtualMachine) VMPathNameAsURL(ctx context.Context) (url.URL, error) {
	op := trace.FromContext(ctx, "VMPathNameAsURL")

	var mvm mo.VirtualMachine

	if err := vm.Properties(op, vm.Reference(), []string{"config.files.vmPathName"}, &mvm); err != nil {
		op.Errorf("Unable to get managed config for VM: %s", err)
		return url.URL{}, err
	}

	if mvm.Config == nil {
		return url.URL{}, errors.New("failed to get datastore path - config not found")
	}

	path := path.Dir(mvm.Config.Files.VmPathName)
	val := url.URL{
		Scheme: "ds",
	}

	// split the dsPath into the url components
	if ix := strings.Index(path, "] "); ix != -1 {
		val.Host = path[strings.Index(path, "[")+1 : ix]
		val.Path = path[ix+2:]
	}

	return val, nil
}

// DatastoreFolderName returns the name of the namespace(vsan) or directory(vmfs) that holds the VM
// this equates to the normal directory that contains the vmx file, stripped of any parent path
func (vm *VirtualMachine) DatastoreFolderName(ctx context.Context) (string, error) {
	op := trace.FromContext(ctx, "DatastoreFolderName")

	u, err := vm.VMPathNameAsURL(op)
	if err != nil {
		return "", err
	}

	return path.Base(u.Path), nil
}

// Folder returns a reference to the parent folder that owns the vm
func (vm *VirtualMachine) Folder(op trace.Operation) (*object.Folder, error) {
	name, err := vm.ObjectName(op)
	if err != nil {
		op.Errorf("Unable to get VM Name to acquire folder: %s", err)
		return nil, err
	}
	// find the VM by name - this is to ensure we have a
	// consistent inventory path
	v, err := vm.Session.Finder.VirtualMachine(op, name)
	if err != nil {
		op.Errorf("Unable to find VM(name: %s) to acquire folder: %s", name, err)
		return nil, err
	}
	inv := path.Dir(v.InventoryPath)
	// Parent to determine if VM or vApp
	p, err := vm.Parent(op)
	if err != nil {
		op.Errorf("Unable to get VM Parent to acquire folder: %s", err)
		return nil, err
	}
	// if vApp then move up a slot in the inventory path
	if p.Type == "VirtualApp" {
		inv = path.Dir(inv)
	}
	// find the vm folder
	folderRef, err := vm.Session.Finder.Folder(op, inv)
	if err != nil {
		e := fmt.Errorf("Error finding folder: %s", err)
		op.Errorf(e.Error())
		return nil, e
	}

	return folderRef, nil
}

func (vm *VirtualMachine) getNetworkName(op trace.Operation, nic types.BaseVirtualEthernetCard) (string, error) {
	if card, ok := nic.GetVirtualEthernetCard().Backing.(*types.VirtualEthernetCardDistributedVirtualPortBackingInfo); ok {
		pg := card.Port.PortgroupKey
		pgref := object.NewDistributedVirtualPortgroup(vm.Session.Vim25(), types.ManagedObjectReference{
			Type:  "DistributedVirtualPortgroup",
			Value: pg,
		})

		var pgo mo.DistributedVirtualPortgroup
		err := pgref.Properties(op, pgref.Reference(), []string{"config"}, &pgo)
		if err != nil {
			op.Errorf("Failed to query portgroup %s for %s", pg, err)
			return "", err
		}
		return pgo.Config.Name, nil
	}
	return nic.GetVirtualEthernetCard().DeviceInfo.GetDescription().Summary, nil
}

func (vm *VirtualMachine) FetchExtraConfigBaseOptions(ctx context.Context) ([]types.BaseOptionValue, error) {
	op := trace.FromContext(ctx, "FetchExtraConfigBaseOptions")

	var err error

	var mvm mo.VirtualMachine

	if err = vm.Properties(op, vm.Reference(), []string{"config.extraConfig"}, &mvm); err != nil {
		op.Errorf("Unable to get vm config: %s", err)
		return nil, err
	}

	return mvm.Config.ExtraConfig, nil
}

func (vm *VirtualMachine) FetchExtraConfig(ctx context.Context) (map[string]string, error) {
	op := trace.FromContext(ctx, "FetchExtraConfig")

	info := make(map[string]string)

	v, err := vm.FetchExtraConfigBaseOptions(op)
	if err != nil {
		return nil, err
	}

	for _, bov := range v {
		ov := bov.GetOptionValue()
		value, _ := ov.Value.(string)
		info[ov.Key] = value
	}
	return info, nil
}

// WaitForExtraConfig waits until key shows up with the expected value inside the ExtraConfig
func (vm *VirtualMachine) WaitForExtraConfig(ctx context.Context, waitFunc func(pc []types.PropertyChange) bool) error {
	op := trace.FromContext(ctx, "WaitForExtraConfig")

	// Get the default collector
	p := property.DefaultCollector(vm.Vim25())

	// Wait on config.extraConfig
	// https://www.vmware.com/support/developer/vc-sdk/visdk2xpubs/ReferenceGuide/vim.vm.ConfigInfo.html
	return property.Wait(op, p, vm.Reference(), []string{"config.extraConfig", object.PropRuntimePowerState}, waitFunc)
}

func (vm *VirtualMachine) WaitForKeyInExtraConfig(ctx context.Context, key string) (string, error) {
	op := trace.FromContext(ctx, "WaitForKeyInExtraConfig")

	var detail string
	var poweredOff error

	waitFunc := func(pc []types.PropertyChange) bool {
		for _, c := range pc {
			if c.Op != types.PropertyChangeOpAssign {
				continue
			}

			switch v := c.Val.(type) {
			case types.ArrayOfOptionValue:
				for _, value := range v.OptionValue {
					// check the status of the key and return true if it's been set to non-nil
					if key == value.GetOptionValue().Key {
						detail = value.GetOptionValue().Value.(string)
						if detail != "" && detail != "<nil>" {
							// ensure we clear any tentative error
							poweredOff = nil

							return true
						}
						break // continue the outer loop as we may have a powerState change too
					}
				}
			case types.VirtualMachinePowerState:
				// Give up if the vm has powered off
				if v != types.VirtualMachinePowerStatePoweredOn {
					msg := "powered off"
					if v == types.VirtualMachinePowerStateSuspended {
						// Unlikely, but possible if the VM was suspended out-of-band
						msg = string(v)
					}
					poweredOff = fmt.Errorf("container VM has unexpectedly %s", msg)
				}
			}
		}

		return poweredOff != nil
	}

	err := vm.WaitForExtraConfig(op, waitFunc)
	if err == nil && poweredOff != nil {
		err = poweredOff
	}

	if err != nil {
		return "", err
	}
	return detail, nil
}

func (vm *VirtualMachine) UUID(ctx context.Context) (string, error) {
	op := trace.FromContext(ctx, "UUID")

	var err error
	var mvm mo.VirtualMachine

	if err = vm.Properties(op, vm.Reference(), []string{"summary.config"}, &mvm); err != nil {
		op.Errorf("Unable to get vm summary.config property: %s", err)
		return "", err
	}

	return mvm.Summary.Config.Uuid, nil
}

// DeleteExceptDisks destroys the VM after detaching all virtual disks
func (vm *VirtualMachine) DeleteExceptDisks(ctx context.Context) (*types.TaskInfo, error) {

	op := trace.FromContext(ctx, "DeleteExceptDisks")
	vmName, err := vm.ObjectName(op)
	if err != nil {
		vmName = vm.String()
		op.Debugf("Failed to get VM name, using moref %q instead due to: %s", vmName, err)
	}

	firstTime := true
	err = retry.Do(func() error {
		op.Debugf("Getting list of the devices for VM %q", vmName)
		devices, err := vm.Device(op)
		if err != nil {
			return err
		}

		disks := devices.SelectByType(&types.VirtualDisk{})
		if len(disks) > 0 {
			op.Debugf("Removing disks from VM %q", vmName)
			firstTime = false
			return vm.RemoveDevice(op, true, disks...)
		}

		if firstTime {
			op.Debugf("Disk list is empty for VM %q", vmName)
		} else {
			op.Debugf("All VM %q disks were removed at first call, but the result yielded an error that caused a retry", "%q")
		}

		return nil

	}, func(err error) bool {
		return tasks.IsRetryError(op, err)
	})

	if err != nil {
		return nil, err
	}

	op.Debugf("Destroying VM %q", vmName)
	info, err := vm.WaitForResult(op, func(ctx context.Context) (tasks.Task, error) {
		return vm.Destroy(ctx)
	})

	if err == nil || !tasks.IsMethodDisabledError(err) {
		return info, err
	}

	// If destroy method is disabled on this VM, re-enable it and retry
	op.Debugf("Destroy is disabled. Enabling destroy for VM %q", vmName)
	err = retry.Do(func() error {
		return vm.EnableDestroy(op)
	}, tasks.IsConcurrentAccessError)

	if err != nil {
		return nil, err
	}

	op.Debugf("Retrying destroy of VM %q again", vmName)
	return vm.WaitForResult(op, func(ctx context.Context) (tasks.Task, error) {
		return vm.Destroy(ctx)
	})
}

func (vm *VirtualMachine) VMPathName(ctx context.Context) (string, error) {
	op := trace.FromContext(ctx, "VMPathName")

	var err error
	var mvm mo.VirtualMachine

	if err = vm.Properties(op, vm.Reference(), []string{"config.files.vmPathName"}, &mvm); err != nil {
		op.Errorf("Unable to get vm config.files property: %s", err)
		return "", err
	}
	return mvm.Config.Files.VmPathName, nil
}

// GetCurrentSnapshotTree returns current snapshot, with tree information
func (vm *VirtualMachine) GetCurrentSnapshotTree(ctx context.Context) (*types.VirtualMachineSnapshotTree, error) {
	op := trace.FromContext(ctx, "GetCurrentSnapshotTree")

	var err error
	var mvm mo.VirtualMachine

	if err = vm.Properties(op, vm.Reference(), []string{"snapshot"}, &mvm); err != nil {
		op.Infof("Unable to get vm properties: %s", err)
		return nil, err
	}
	if mvm.Snapshot == nil {
		// no snapshot at all
		return nil, nil
	}

	current := mvm.Snapshot.CurrentSnapshot
	q := list.New()
	for _, c := range mvm.Snapshot.RootSnapshotList {
		q.PushBack(c)
	}

	compareID := func(node types.VirtualMachineSnapshotTree) bool {
		if node.Snapshot == *current {
			return true
		}
		return false
	}
	return vm.bfsSnapshotTree(q, compareID), nil
}

// GetCurrentSnapshotTreeByName returns current snapshot, with tree information
func (vm *VirtualMachine) GetSnapshotTreeByName(ctx context.Context, name string) (*types.VirtualMachineSnapshotTree, error) {
	op := trace.FromContext(ctx, "GetSnapshotTreeByName")

	var err error
	var mvm mo.VirtualMachine

	if err = vm.Properties(op, vm.Reference(), []string{"snapshot"}, &mvm); err != nil {
		op.Infof("Unable to get vm properties: %s", err)
		return nil, err
	}
	if mvm.Snapshot == nil {
		// no snapshot at all
		return nil, nil
	}

	q := list.New()
	for _, c := range mvm.Snapshot.RootSnapshotList {
		q.PushBack(c)
	}

	compareName := func(node types.VirtualMachineSnapshotTree) bool {
		if node.Name == name {
			return true
		}
		return false
	}
	return vm.bfsSnapshotTree(q, compareName), nil
}

// Finds a snapshot tree based on comparator function 'compare' via a breadth first search of the snapshot tree attached to the VM
func (vm *VirtualMachine) bfsSnapshotTree(q *list.List, compare func(node types.VirtualMachineSnapshotTree) bool) *types.VirtualMachineSnapshotTree {
	if q.Len() == 0 {
		return nil
	}

	e := q.Front()
	tree := q.Remove(e).(types.VirtualMachineSnapshotTree)
	if compare(tree) {
		return &tree
	}
	for _, c := range tree.ChildSnapshotList {
		q.PushBack(c)
	}
	return vm.bfsSnapshotTree(q, compare)
}

// IsConfigureSnapshot is the helper func that returns true if node is a snapshot with specified name prefix
func IsConfigureSnapshot(node *types.VirtualMachineSnapshotTree, prefix string) bool {
	return node != nil && strings.HasPrefix(node.Name, prefix)
}

func (vm *VirtualMachine) registerVM(op trace.Operation, path, name string,
	vapp, pool, host *types.ManagedObjectReference, vmfolder *object.Folder) (*object.Task, error) {
	op.Debugf("Register VM %s", name)

	if vapp == nil {
		var hostObject *object.HostSystem
		if host != nil {
			hostObject = object.NewHostSystem(vm.Vim25(), *host)
		}
		poolObject := object.NewResourcePool(vm.Vim25(), *pool)
		return vmfolder.RegisterVM(op, path, name, false, poolObject, hostObject)
	}

	req := types.RegisterChildVM_Task{
		This: vapp.Reference(),
		Path: path,
		Host: host,
	}

	if name != "" {
		req.Name = name
	}

	res, err := methods.RegisterChildVM_Task(op, vm.Vim25(), &req)
	if err != nil {
		return nil, err
	}

	return object.NewTask(vm.Vim25(), res.Returnval), nil
}

func (vm *VirtualMachine) IsFixing() bool {
	return vm.fixing > 0
}

func (vm *VirtualMachine) EnterFixingState() {
	atomic.AddInt32(&vm.fixing, 1)
}

func (vm *VirtualMachine) LeaveFixingState() {
	atomic.StoreInt32(&vm.fixing, 0)
}

// FixInvalidState fix vm invalid state issue through unregister & register
func (vm *VirtualMachine) fixVM(op trace.Operation) error {
	op.Debugf("Fix invalid state VM: %s", vm.Reference())

	properties := []string{"summary.config", "summary.runtime.host", "resourcePool", "parentVApp"}
	op.Debugf("Get vm properties %s", properties)
	var mvm mo.VirtualMachine
	if err := vm.VirtualMachine.Properties(op, vm.Reference(), properties, &mvm); err != nil {
		op.Errorf("Unable to get vm properties: %s", err)
		return err
	}

	name := mvm.Summary.Config.Name
	op.Debugf("Unregister VM %s", name)
	vm.EnterFixingState()
	if err := vm.Unregister(op); err != nil {
		op.Errorf("Unable to unregister vm %q: %s", name, err)

		// Leave fixing state since it will not be reset in the remove event handler
		vm.LeaveFixingState()
		return err
	}

	task, err := vm.registerVM(op, mvm.Summary.Config.VmPathName, name, mvm.ParentVApp, mvm.ResourcePool, mvm.Summary.Runtime.Host, vm.Session.VCHFolder)
	if err != nil {
		op.Errorf("Unable to register VM %q back: %s", name, err)
		return err
	}
	info, err := task.WaitForResult(op, nil)
	if err != nil {
		return err
	}
	// re-register vm will change vm reference, so reset the object reference here
	if info.Error != nil {
		return errors.New(info.Error.LocalizedMessage)
	}

	// set new registered vm attribute back
	newRef := info.Result.(types.ManagedObjectReference)
	common := object.NewCommon(vm.Vim25(), newRef)
	common.InventoryPath = vm.InventoryPath
	vm.Common = common
	return nil
}

func (vm *VirtualMachine) needsFix(op trace.Operation, err error) bool {
	if err == nil {
		return false
	}
	if vm.IsInvalidState(op) {
		op.Debugf("vm %s is invalid", vm.Reference())
		return true
	}

	return false
}

func (vm *VirtualMachine) IsInvalidState(ctx context.Context) bool {
	op := trace.FromContext(ctx, "IsInvalidState")

	var o mo.VirtualMachine
	if err := vm.VirtualMachine.Properties(op, vm.Reference(), []string{"summary.runtime.connectionState"}, &o); err != nil {
		op.Debugf("Failed to get vm properties: %s", err)
		return false
	}
	if o.Summary.Runtime.ConnectionState == types.VirtualMachineConnectionStateInvalid {
		return true
	}
	return false
}

// IsInvalidPowerStateError is an error certifier function for errors coming back from vsphere. It checks for an InvalidPowerStateFault
func (vm *VirtualMachine) IsInvalidPowerStateError(err error) bool {
	if soap.IsVimFault(err) {
		_, ok1 := soap.ToVimFault(err).(*types.InvalidPowerState)
		_, ok2 := soap.ToVimFault(err).(*types.InvalidPowerStateFault)
		return ok1 || ok2
	}

	if soap.IsSoapFault(err) {
		_, ok1 := soap.ToSoapFault(err).VimFault().(types.InvalidPowerState)
		_, ok2 := soap.ToSoapFault(err).VimFault().(types.InvalidPowerStateFault)
		// sometimes we get the correct fault but wrong type
		return ok1 || ok2 || soap.ToSoapFault(err).String == "vim.fault.InvalidPowerState" ||
			soap.ToSoapFault(err).String == "vim.fault.InvalidPowerState"
	}
	return false
}

// WaitForResult is designed to handle VM invalid state error for any VM operations.
// It will call tasks.WaitForResult to retry if there is task in progress error.
func (vm *VirtualMachine) WaitForResult(ctx context.Context, f func(context.Context) (tasks.Task, error)) (*types.TaskInfo, error) {
	op := trace.FromContext(ctx, "WaitForResult")

	info, err := tasks.WaitForResult(op, f)
	if err == nil || !vm.needsFix(op, err) {
		return info, err
	}

	op.Debugf("Try to fix task failure %s", err)
	if nerr := vm.fixVM(op); nerr != nil {
		op.Errorf("Failed to fix task failure: %s", nerr)
		return info, err
	}
	op.Debug("Fixed")
	return tasks.WaitForResult(op, f)
}

func (vm *VirtualMachine) Properties(ctx context.Context, r types.ManagedObjectReference, ps []string, o *mo.VirtualMachine) error {
	// lets ensure we have an operation
	op := trace.FromContext(ctx, "VM Properties")
	defer trace.End(trace.Begin(fmt.Sprintf("VM(%s) Properties(%s)", r, ps), op))

	contains := false
	for _, v := range ps {
		if v == "summary" || v == "summary.runtime" {
			contains = true
			break
		}
	}

	if !contains {
		ps = append(ps, "summary.runtime.connectionState")
	}

	op.Debugf("properties: %s", ps)

	if err := vm.VirtualMachine.Properties(op, r, ps, o); err != nil {
		return err
	}
	if o.Summary.Runtime.ConnectionState != types.VirtualMachineConnectionStateInvalid {
		return nil
	}
	op.Infof("vm %s is in invalid state", r)
	if err := vm.fixVM(op); err != nil {
		op.Errorf("Failed to fix vm %s: %s", vm.Reference(), err)
		return &InvalidState{r: vm.Reference()}
	}

	return vm.VirtualMachine.Properties(op, vm.Reference(), ps, o)
}

func (vm *VirtualMachine) Parent(ctx context.Context) (*types.ManagedObjectReference, error) {
	op := trace.FromContext(ctx, "Parent")

	var mvm mo.VirtualMachine

	if err := vm.Properties(op, vm.Reference(), []string{"parentVApp", "resourcePool"}, &mvm); err != nil {
		op.Errorf("Unable to get VM parent: %s", err)
		return nil, err
	}
	if mvm.ParentVApp != nil {
		return mvm.ParentVApp, nil
	}
	return mvm.ResourcePool, nil
}

func (vm *VirtualMachine) DatastoreReference(ctx context.Context) ([]types.ManagedObjectReference, error) {
	op := trace.FromContext(ctx, "DatastoreReference")

	var mvm mo.VirtualMachine

	if err := vm.Properties(op, vm.Reference(), []string{"datastore"}, &mvm); err != nil {
		op.Errorf("Unable to get VM datastore: %s", err)
		return nil, err
	}
	return mvm.Datastore, nil
}

// VCHUpdateStatus tells if an upgrade/configure has already been started based on the UpdateInProgress flag in ExtraConfig
// It returns the error if the vm operation does not succeed
func (vm *VirtualMachine) VCHUpdateStatus(ctx context.Context) (bool, error) {
	op := trace.FromContext(ctx, "VCHUpdateStatus")

	info, err := vm.FetchExtraConfig(op)
	if err != nil {
		op.Errorf("Unable to get vm ExtraConfig: %s", err)
		return false, err
	}

	if v, ok := info[UpdateStatus]; ok {
		status, err := strconv.ParseBool(v)
		if err != nil {
			//  If error occurs, the bool return value does not matter for the caller.
			return false, fmt.Errorf("failed to parse %s to bool: %s", v, err)
		}
		return status, nil
	}

	// If UpdateStatus is not found, it might be the case that no upgrade/configure has been done to this VCH before
	return false, nil
}

// SetVCHUpdateStatus sets the VCH update status in ExtraConfig
func (vm *VirtualMachine) SetVCHUpdateStatus(ctx context.Context, status bool) error {
	op := trace.FromContext(ctx, "SetVCHUpdateStatus")

	info := make(map[string]string)
	info[UpdateStatus] = strconv.FormatBool(status)

	s := &types.VirtualMachineConfigSpec{
		ExtraConfig: vmomi.OptionValueFromMap(info, true),
	}

	_, err := vm.WaitForResult(op, func(op context.Context) (tasks.Task, error) {
		return vm.Reconfigure(op, *s)
	})

	return err
}

// DisableDestroy attempts to disable the VirtualMachine.Destroy_Task method on the VM
// After destroy is disabled for the VM, any session other than the session that disables the destroy can't evoke the method
// The "VM delete" option will be grayed out on VC UI
// Requires the "Global.Disable" VC privilege
func (vm *VirtualMachine) DisableDestroy(ctx context.Context) error {
	// For nonVC, OOB deletion won't disrupt disk images (ESX doesn't delete parent disks)
	// See https://github.com/vmware/vic/issues/2928
	if !vm.IsVC() {
		return nil
	}

	op := trace.FromContext(ctx, "DisableDestroy")

	m := object.NewAuthorizationManager(vm.Vim25())

	method := []object.DisabledMethodRequest{
		{
			Method: DestroyTask,
			Reason: "Managed by VIC Engine",
		},
	}

	obj := []types.ManagedObjectReference{vm.Reference()}

	err := m.DisableMethods(op, obj, method, "VIC")
	if err != nil {
		op.Warnf("Failed to disable method %s for %s: %s", method[0].Method, obj[0], err)
		return err
	}

	return nil
}

// EnableDestroy attempts to enable the VirtualMachine.Destroy_Task method for all sessions
func (vm *VirtualMachine) EnableDestroy(ctx context.Context) error {
	// For nonVC, OOB deletion won't disrupt disk images (ESX doesn't delete parent disks)
	// See https://github.com/vmware/vic/issues/2928
	if !vm.IsVC() {
		return nil
	}

	op := trace.FromContext(ctx, "EnableDestroy")

	m := object.NewAuthorizationManager(vm.Vim25())

	obj := []types.ManagedObjectReference{vm.Reference()}

	err := m.EnableMethods(op, obj, []string{DestroyTask}, "VIC")
	if err != nil {
		op.Warnf("Failed to enable Destroy_Task for %s: %s", obj[0], err)
		return err
	}

	return nil
}

// RemoveSnapshot removes the provided snapshot
func (vm *VirtualMachine) RemoveSnapshot(op trace.Operation, snapshot *types.ManagedObjectReference, removeChildren bool, consolidate *bool) (*object.Task, error) {
	req := types.RemoveSnapshot_Task{
		This:           snapshot.Reference(),
		RemoveChildren: removeChildren,
		Consolidate:    consolidate,
	}

	res, err := methods.RemoveSnapshot_Task(op, vm.Vim25(), &req)
	if err != nil {
		return nil, err
	}

	return object.NewTask(vm.Vim25(), res.Returnval), nil
}

// PowerOn powers on a VM. If the environment is VC without DRS enabled, it will attempt to relocate  the VM
// to the most suitable host in the cluster. If relocation or subsequent power-on fail, it will attempt the next
// best host, and repeat this process until a successful power-on is achieved or there are no more hosts to try.
func (vm *VirtualMachine) PowerOn(op trace.Operation) error {
	// if we aren't in VC, we don't need to recommend or use DRS-aware power-on
	if !vm.IsVC() {
		_, err := vm.WaitForResult(op, func(op context.Context) (tasks.Task, error) {
			return vm.VirtualMachine.PowerOn(op)
		})
		return err
	}

	h, err := vm.Cluster.Hosts(op)
	if err != nil {
		return err
	}

	// we only recommend if the VM is in a cluster with more than one host
	cls := vm.InCluster(op) && len(h) > 1
	op.Debugf("%s resides in a multi-host cluster: %t", vm.Reference().String(), cls)

	// or if DRS is disabled
	drs := vm.Session.DRSEnabled != nil && *vm.Session.DRSEnabled
	op.Debugf("DRS enabled: %t", drs)

	if !cls || drs {
		// if we aren't in a multi-host cluster or if we are and DRS is enabled,
		// there is no reason to recommend a host, so bail early and power-on.
		return vm.powerOnDRS(op)
	}

	// otherwise place the VM before powering it on
	hmp := performance.NewHostMetricsProvider(vm.Session.Vim25())
	rhp, err := placement.NewRankedHostPolicy(op, vm.Cluster, hmp)
	if err != nil {
		return err
	}

	// first we check if the host on which the VM was created is adequate for power-on
	if rhp.CheckHost(op, vm.VirtualMachine) {
		// if so, just power-on, don't bother recommending a better host
		return vm.powerOnDRS(op)
	}

	op.Debugf("Host is not adequate for power-on, getting placement recommendation")

	var hosts, subset []*object.HostSystem

	f := func() error {
		hosts, err = rhp.RecommendHost(op, subset)
		if err != nil {
			return fmt.Errorf("error recommending host: %s", err.Error())
		}
		return nil
	}

	conf := retry.NewBackoffConfig()
	conf.MaxInterval = 1 * time.Second
	conf.MaxElapsedTime = 20 * time.Second

	err = retry.DoWithConfig(f, retry.OnError, conf)
	if err != nil {
		return err
	}

	for len(hosts) > 0 {
		op.Debugf("hosts: %s", hosts)
		op.Infof("Placement recommended %s", hosts[0].Reference().String())

		var err error
		if err = vm.relocate(op, hosts[0]); err != nil {
			op.Warnf("VM relocation failed: %s", err.Error())
		} else if err = vm.powerOnDRS(op); err != nil {
			op.Warnf("VM power-on failed: %s", err.Error())
		} else {
			return nil
		}

		// if relocation or powerOn fails, something has changed and we need a new recommendation
		subset = hosts[1:]

		if err = retry.DoWithConfig(f, retry.OnError, conf); err != nil {
			return err
		}

	}
	op.Warnf("vm placement failed: no available hosts. Attempting power-on.")
	return vm.powerOnDRS(op)
}

func (vm *VirtualMachine) powerOnDRS(op trace.Operation) error {
	option := &types.OptionValue{
		Key:   string(types.ClusterPowerOnVmOptionOverrideAutomationLevel),
		Value: string(types.DrsBehaviorFullyAutomated),
	}

	t, err := vm.WaitForResult(op, func(op context.Context) (tasks.Task, error) {
		return vm.Datacenter.PowerOnVM(op, []types.ManagedObjectReference{vm.Reference()}, option)
	})
	if err != nil {
		return err
	}

	switch r := t.Result.(type) {
	case types.ClusterPowerOnVmResult:
		attempts := len(r.Attempted)
		if attempts != 1 {
			return fmt.Errorf("Attempted to power on the wrong number of VMs. Expected 1, attempted %d", attempts)
		}
		info := r.Attempted[0]
		task := object.NewTask(vm.Session.Vim25(), *info.Task)
		_, err := task.WaitForResult(op, nil)
		return err
	default:
		return fmt.Errorf("Unexpected return type when attempting to power on VM: %T", r)
	}
}

func (vm *VirtualMachine) relocate(op trace.Operation, host *object.HostSystem) error {
	h, err := vm.HostSystem(op)
	if err != nil {
		return err
	}
	src := h.Reference()
	dst := host.Reference()

	// NOP if dest host and src host are the same
	if src.String() == dst.String() {
		op.Debugf("Skipping relocate - source and destination hosts are the same")
		return nil
	}

	op.Debugf("Attempting to relocate %s from host %s to host %s", vm.Reference().String(), src.String(), dst.String())

	spec := types.VirtualMachineRelocateSpec{Host: &dst}
	_, err = vm.WaitForResult(op, func(op context.Context) (tasks.Task, error) {
		return vm.Relocate(op, spec, types.VirtualMachineMovePriorityDefaultPriority)
	})
	if err != nil {
		return err
	}

	op.Infof("VM %s successfully relocated", vm.Reference().String())

	return nil
}

// InCluster returns true if the VM belongs to a cluster
func (vm *VirtualMachine) InCluster(op trace.Operation) bool {
	cls := vm.Cluster.Reference().Type == "ClusterComputeResource"
	op.Debugf("vm compute resource: %s", vm.Cluster.Name())
	return cls
}
