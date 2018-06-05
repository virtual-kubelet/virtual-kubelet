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

package vm

import (
	"container/list"
	"context"
	"fmt"
	"math"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/vic/lib/guest"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/vsphere/extraconfig/vmomi"
	"github.com/vmware/vic/pkg/vsphere/session"
	"github.com/vmware/vic/pkg/vsphere/sys"
	"github.com/vmware/vic/pkg/vsphere/tasks"
	"github.com/vmware/vic/pkg/vsphere/test"
)

func CreateVM(ctx context.Context, session *session.Session, host *object.HostSystem, name string) (*types.ManagedObjectReference, error) {
	// Create the spec config
	specconfig := test.SpecConfig(session, name)

	// Create a linux guest
	linux, err := guest.NewLinuxGuest(ctx, session, specconfig)
	if err != nil {
		return nil, err
	}

	// Create the vm
	task, err := session.VMFolder.CreateVM(ctx, *linux.Spec().Spec(), session.Pool, host)
	if err != nil {
		return nil, err
	}
	info, err := task.WaitForResult(ctx, nil)
	if err != nil {
		return nil, err
	}

	moref := info.Result.(types.ManagedObjectReference)
	// Return the moRef
	return &moref, nil
}

func TestDeleteExceptDisk(t *testing.T) {
	s := os.Getenv("DRONE")
	if s != "" {
		t.Skip("Skipping: test must be run in a VM")
	}

	ctx := context.Background()

	session := test.Session(ctx, t)
	defer session.Logout(ctx)

	host := test.PickRandomHost(ctx, session, t)

	uuid, err := sys.UUID()
	if err != nil {
		t.Fatalf("unable to get UUID for guest - used for VM name: %s", err)
	}
	name := fmt.Sprintf("%s-%d", uuid, rand.Intn(math.MaxInt32))

	moref, err := CreateVM(ctx, session, host, name)
	if err != nil {
		t.Fatalf("ERROR: %s", err)
	}
	// Wrap the result with our version of VirtualMachine
	vm := NewVirtualMachine(ctx, session, *moref)

	folder, err := vm.DatastoreFolderName(ctx)
	if err != nil {
		t.Fatalf("ERROR: %s", err)
	}

	// generate the disk name
	diskName := fmt.Sprintf("%s/%s.vmdk", folder, folder)

	// Delete the VM but not it's disk
	_, err = vm.DeleteExceptDisks(ctx)
	if err != nil {
		t.Fatalf("ERROR: %s", err)
	}

	// check that the disk still exists
	session.Datastore.Stat(ctx, diskName)
	if err != nil {
		t.Fatalf("Disk does not exist")
	}

	// clean up
	dm := object.NewVirtualDiskManager(session.Vim25())

	task, err := dm.DeleteVirtualDisk(context.TODO(), diskName, nil)
	if err != nil {
		t.Fatalf("Unable to locate orphan vmdk: %s", err)
	}

	if err = task.Wait(context.TODO()); err != nil {
		t.Fatalf("Unable to remove orphan vmdk: %s", err)
	}
}

func TestVM(t *testing.T) {

	s := os.Getenv("DRONE")
	if s != "" {
		t.Skip("Skipping: test must be run in a VM")
	}

	ctx := context.Background()

	session := test.Session(ctx, t)
	defer session.Logout(ctx)

	host := test.PickRandomHost(ctx, session, t)

	uuid, err := sys.UUID()
	if err != nil {
		t.Fatalf("unable to get UUID for guest - used for VM name: %s", err)
		return
	}
	name := fmt.Sprintf("%s-%d", uuid, rand.Intn(math.MaxInt32))

	moref, err := CreateVM(ctx, session, host, name)
	if err != nil {
		t.Fatalf("ERROR: %s", err)
	}
	// Wrap the result with our version of VirtualMachine
	vm := NewVirtualMachine(ctx, session, *moref)

	// Check the state
	state, err := vm.PowerState(ctx)
	if err != nil {
		t.Fatalf("ERROR: %s", err)
	}

	assert.Equal(t, types.VirtualMachinePowerStatePoweredOff, state)

	// Check VM name
	rname, err := vm.ObjectName(ctx)
	if err != nil {
		t.Errorf("Failed to load VM name: %s", err)
	}
	assert.Equal(t, name, rname)

	// Get VM UUID
	ruuid, err := vm.UUID(ctx)
	if err != nil {
		t.Errorf("Failed to load VM UUID: %s", err)
	}
	t.Logf("Got UUID: %s", ruuid)

	err = vm.fixVM(trace.FromContext(ctx, "TestVM"))
	if err != nil {
		t.Errorf("Failed to fix vm: %s", err)
	}
	newVM, err := session.Finder.VirtualMachine(ctx, name)
	if err != nil {
		t.Errorf("Failed to find fixed vm: %s", err)
	}
	assert.Equal(t, vm.Reference(), newVM.Reference())

	// VM properties
	var ovm mo.VirtualMachine
	if err = vm.Properties(ctx, newVM.Reference(), []string{"config"}, &ovm); err != nil {
		t.Errorf("Failed to get vm properties: %s", err)
	}

	// Destroy the vm
	task, err := vm.Destroy(ctx)
	if err != nil {
		t.Fatalf("ERROR: %s", err)
	}
	_, err = task.WaitForResult(ctx, nil)
	if err != nil {
		t.Fatalf("ERROR: %s", err)
	}
}

func TestVMFailureWithTimeout(t *testing.T) {
	ctx := context.Background()

	session := test.Session(ctx, t)
	defer session.Logout(ctx)

	host := test.PickRandomHost(ctx, session, t)

	ctx, cancel := context.WithTimeout(ctx, 1*time.Microsecond)
	defer cancel()

	uuid, err := sys.UUID()
	if err != nil {
		t.Fatalf("unable to get UUID for guest - used for VM name: %s", err)
		return
	}
	name := fmt.Sprintf("%s-%d", uuid, rand.Intn(math.MaxInt32))

	_, err = CreateVM(ctx, session, host, name)
	if err != nil && err != context.DeadlineExceeded {
		t.Fatalf("ERROR: %s", err)
	}
}

func TestVMAttributes(t *testing.T) {

	ctx := context.Background()

	session := test.Session(ctx, t)
	defer session.Logout(ctx)

	host := test.PickRandomHost(ctx, session, t)

	uuid, err := sys.UUID()
	if err != nil {
		t.Fatalf("unable to get UUID for guest - used for VM name: %s", err)
		return
	}
	ID := fmt.Sprintf("%s-%d", uuid, rand.Intn(math.MaxInt32))

	moref, err := CreateVM(ctx, session, host, ID)
	if err != nil {
		t.Fatalf("ERROR: %s", err)
	}
	// Wrap the result with our version of VirtualMachine
	vm := NewVirtualMachine(ctx, session, *moref)

	folder, err := vm.DatastoreFolderName(ctx)
	if err != nil {
		t.Fatalf("ERROR: %s", err)
	}

	name, err := vm.ObjectName(ctx)
	if err != nil {
		t.Fatalf("ERROR: %s", err)
	}
	assert.Equal(t, name, folder)
	task, err := vm.VirtualMachine.PowerOn(ctx)
	if err != nil {
		t.Fatalf("ERROR: %s", err)
	}
	_, err = task.WaitForResult(ctx, nil)
	if err != nil {
		t.Fatalf("ERROR: %s", err)
	}

	if guest, err := vm.FetchExtraConfig(ctx); err != nil {
		t.Fatalf("ERROR: %s", err)
	} else {
		assert.NotEmpty(t, guest)
	}
	defer func() {
		// Destroy the vm
		task, err := vm.PowerOff(ctx)
		if err != nil {
			t.Fatalf("ERROR: %s", err)
		}
		_, err = task.WaitForResult(ctx, nil)
		if err != nil {
			t.Fatalf("ERROR: %s", err)
		}
		task, err = vm.Destroy(ctx)
		if err != nil {
			t.Fatalf("ERROR: %s", err)
		}
		_, err = task.WaitForResult(ctx, nil)
		if err != nil {
			t.Fatalf("ERROR: %s", err)
		}
	}()
}

func TestWaitForKeyInExtraConfig(t *testing.T) {
	ctx := context.Background()

	m := simulator.ESX()
	defer m.Remove()
	err := m.Create()
	if err != nil {
		t.Fatal(err)
	}

	server := m.Service.NewServer()
	defer server.Close()

	config := &session.Config{
		Service: server.URL.String(),
	}

	s, err := session.NewSession(config).Connect(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if s, err = s.Populate(ctx); err != nil {
		t.Fatal(err)
	}

	vms, err := s.Finder.VirtualMachineList(ctx, "*")
	if err != nil {
		t.Fatal(err)
	}

	vm := NewVirtualMachineFromVM(ctx, s, vms[0])

	opt := &types.OptionValue{Key: "foo", Value: "bar"}
	obj := simulator.Map.Get(vm.Reference()).(*simulator.VirtualMachine)

	val, err := vm.WaitForKeyInExtraConfig(ctx, opt.Key)

	if err == nil {
		t.Error("expected error")
	}

	obj.Config.ExtraConfig = append(obj.Config.ExtraConfig, opt)
	obj.Summary.Runtime.PowerState = types.VirtualMachinePowerStatePoweredOn

	val, err = vm.WaitForKeyInExtraConfig(ctx, opt.Key)
	if err != nil {
		t.Fatal(err)
	}

	if val != opt.Value {
		t.Errorf("%s != %s", val, opt.Value)
	}
}

func createSnapshotTree(prefix string, deep int, wide int) []types.VirtualMachineSnapshotTree {
	var result []types.VirtualMachineSnapshotTree
	if deep == 0 {
		return nil
	}
	for i := 1; i <= wide; i++ {
		nodeID := fmt.Sprintf("%s%d", prefix, i)
		node := types.VirtualMachineSnapshotTree{
			Snapshot: types.ManagedObjectReference{
				Type:  "Snapshot",
				Value: nodeID,
			},
			Name: nodeID,
		}
		node.ChildSnapshotList = createSnapshotTree(nodeID, deep-1, wide)
		result = append(result, node)
	}
	return result
}

func TestBfsSnapshotTree(t *testing.T) {
	ref := &types.ManagedObjectReference{
		Type:  "Snapshot",
		Value: "12131",
	}
	rootList := createSnapshotTree("", 5, 5)

	ctx := context.Background()

	session := test.Session(ctx, t)
	defer session.Logout(ctx)
	vm := NewVirtualMachine(ctx, session, *ref)
	q := list.New()
	for _, c := range rootList {
		q.PushBack(c)
	}

	compareID := func(node types.VirtualMachineSnapshotTree) bool {
		if node.Snapshot == *ref {
			t.Logf("Found match")
			return true
		}
		return false
	}
	current := vm.bfsSnapshotTree(q, compareID)
	if current == nil {
		t.Errorf("Should found current snapshot")
	}
	q = list.New()
	for _, c := range rootList {
		q.PushBack(c)
	}

	ref = &types.ManagedObjectReference{
		Type:  "Snapshot",
		Value: "185",
	}
	current = vm.bfsSnapshotTree(q, compareID)
	if current != nil {
		t.Errorf("Should not found snapshot")
	}

	name := "12131"
	compareName := func(node types.VirtualMachineSnapshotTree) bool {
		if node.Name == name {
			t.Logf("Found match")
			return true
		}
		return false
	}
	q = list.New()
	for _, c := range rootList {
		q.PushBack(c)
	}
	found := vm.bfsSnapshotTree(q, compareName)
	if found == nil {
		t.Errorf("Should found snapshot %q", name)
	}
	q = list.New()
	for _, c := range rootList {
		q.PushBack(c)
	}
	name = "185"
	found = vm.bfsSnapshotTree(q, compareName)
	if found != nil {
		t.Errorf("Should not found snapshot")
	}
}

// TestProperties test vm.properties happy path and fix vm path
func TestIsFixing(t *testing.T) {
	mo := types.ManagedObjectReference{Type: "vm", Value: "12"}
	v := object.NewVirtualMachine(nil, mo)
	vm := NewVirtualMachineFromVM(nil, nil, v)
	assert.False(t, vm.IsFixing(), "new vm should not in fixing status")
	vm.EnterFixingState()
	assert.True(t, vm.IsFixing(), "vm should be in fixing status")
	vm.EnterFixingState()
	assert.True(t, vm.IsFixing(), "vm should be in fixing status")
	vm.LeaveFixingState()
	assert.False(t, vm.IsFixing(), "vm should not be in fixing status")
}

// TestProperties test vm.properties happy path and fix vm path
func TestProperties(t *testing.T) {
	ctx := context.Background()

	// Nothing VC specific in this test, so we use the simpler ESX model
	model := simulator.ESX()
	model.Autostart = false
	defer model.Remove()
	err := model.Create()
	if err != nil {
		t.Fatal(err)
	}

	server := model.Service.NewServer()
	defer server.Close()
	client, err := govmomi.NewClient(ctx, server.URL, true)
	if err != nil {
		t.Fatal(err)
	}

	// Any VM will do
	finder := find.NewFinder(client.Client, false)
	vmo, err := finder.VirtualMachine(ctx, "/ha-datacenter/vm/*_VM0")
	if err != nil {
		t.Fatal(err)
	}

	config := &session.Config{
		Service:        server.URL.String(),
		Insecure:       true,
		Keepalive:      time.Duration(5) * time.Minute,
		DatacenterPath: "",
		DatastorePath:  "/ha-datacenter/datastore/*",
		HostPath:       "/ha-datacenter/host/*/*",
		PoolPath:       "/ha-datacenter/host/*/Resources",
	}

	s, err := session.NewSession(config).Connect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	s.Populate(ctx)
	vmm := NewVirtualMachine(ctx, s, vmo.Reference())
	// Test the success path
	var o mo.VirtualMachine
	err = vmm.Properties(ctx, vmo.Reference(), []string{"config", "summary", "resourcePool", "parentVApp"}, &o)
	if err != nil {
		t.Fatal(err)
	}

	//	// Inject invalid connection state to vm
	ref := simulator.Map.Get(vmo.Reference()).(*simulator.VirtualMachine)
	ref.Summary.Config.VmPathName = ref.Config.Files.VmPathName
	ref.Summary.Runtime.ConnectionState = types.VirtualMachineConnectionStateInvalid

	err = vmm.Properties(ctx, vmo.Reference(), []string{"config", "summary"}, &o)
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, o.Summary.Runtime.ConnectionState != types.VirtualMachineConnectionStateInvalid, "vm state should be fixed")
}

// TestWaitForResult covers the success path and invalid vm fix path
func TestWaitForResult(t *testing.T) {
	ctx := context.Background()

	// Nothing VC specific in this test, so we use the simpler ESX model
	model := simulator.ESX()
	model.Autostart = false
	defer model.Remove()
	err := model.Create()
	if err != nil {
		t.Fatal(err)
	}

	server := model.Service.NewServer()
	defer server.Close()

	client, err := govmomi.NewClient(ctx, server.URL, true)
	if err != nil {
		t.Fatal(err)
	}

	// Any VM will do
	finder := find.NewFinder(client.Client, false)
	vmo, err := finder.VirtualMachine(ctx, "/ha-datacenter/vm/*_VM0")
	if err != nil {
		t.Fatal(err)
	}

	config := &session.Config{
		Service:        server.URL.String(),
		Insecure:       true,
		Keepalive:      time.Duration(5) * time.Minute,
		DatacenterPath: "",
		DatastorePath:  "/ha-datacenter/datastore/*",
		HostPath:       "/ha-datacenter/host/*/*",
		PoolPath:       "/ha-datacenter/host/*/Resources",
	}

	s, err := session.NewSession(config).Connect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	s.Populate(ctx)
	vmm := NewVirtualMachine(ctx, s, vmo.Reference())
	// Test the success path
	_, err = vmm.WaitForResult(ctx, func(ctx context.Context) (tasks.Task, error) {
		return vmm.VirtualMachine.PowerOn(ctx)
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = vmm.WaitForResult(ctx, func(ctx context.Context) (tasks.Task, error) {
		return vmm.PowerOff(ctx)
	})
	if err != nil {
		t.Fatal(err)
	}

	ref := simulator.Map.Get(vmm.Reference()).(*simulator.VirtualMachine)

	// Test task failed, but vm is not in invalid state
	called := 0
	_, err = vmm.WaitForResult(ctx, func(ctx context.Context) (tasks.Task, error) {
		called++
		return vmm.PowerOff(ctx)
	})
	if err == nil {
		t.Fatal("Should have error")
	}
	assert.True(t, called == 1, "task should not be retried")

	// Test task failure with invalid state vm
	ref.Summary.Config.VmPathName = ref.Config.Files.VmPathName
	ref.Summary.Runtime.ConnectionState = types.VirtualMachineConnectionStateInvalid
	called = 0
	_, err = vmm.WaitForResult(ctx, func(ctx context.Context) (tasks.Task, error) {
		called++
		return vmm.PowerOff(ctx)
	})
	assert.True(t, called == 2, "task should be retried once")
	assert.True(t, !vmm.IsInvalidState(ctx), "vm state should be fixed")
}

// SetUpdateStatus sets the VCH upgrade/configure status.
func SetUpdateStatus(ctx context.Context, updateStatus string, vm *VirtualMachine) error {
	info := make(map[string]string)
	info[UpdateStatus] = updateStatus

	s := &types.VirtualMachineConfigSpec{
		ExtraConfig: vmomi.OptionValueFromMap(info, true),
	}

	_, err := vm.WaitForResult(ctx, func(ctx context.Context) (tasks.Task, error) {
		return vm.Reconfigure(ctx, *s)
	})
	if err != nil {
		return err
	}

	return nil
}

// TestVCHUpdateStatus tests if VCHUpdateStatus() could obtain the correct VCH upgrade/configure status
func TestVCHUpdateStatus(t *testing.T) {
	ctx := context.Background()

	// Nothing VC specific in this test, so we use the simpler ESX model
	model := simulator.ESX()
	defer model.Remove()
	err := model.Create()
	if err != nil {
		t.Fatal(err)
	}

	server := model.Service.NewServer()
	defer server.Close()
	client, err := govmomi.NewClient(ctx, server.URL, true)
	if err != nil {
		t.Fatal(err)
	}

	// Any VM will do
	finder := find.NewFinder(client.Client, false)
	vmo, err := finder.VirtualMachine(ctx, "/ha-datacenter/vm/*_VM0")
	if err != nil {
		t.Fatal(err)
	}

	config := &session.Config{
		Service:        server.URL.String(),
		Insecure:       true,
		Keepalive:      time.Duration(5) * time.Minute,
		DatacenterPath: "",
		DatastorePath:  "/ha-datacenter/datastore/*",
		HostPath:       "/ha-datacenter/host/*/*",
		PoolPath:       "/ha-datacenter/host/*/Resources",
	}

	s, err := session.NewSession(config).Connect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	s.Populate(ctx)
	vmm := NewVirtualMachine(ctx, s, vmo.Reference())

	updateStatus, err := vmm.VCHUpdateStatus(ctx)
	if err != nil {
		t.Fatalf("ERROR: %s", err)
	}
	assert.False(t, updateStatus, "updateStatus should be false if UpdateInProgress is not set in the VCH's ExtraConfig")

	// Set UpdateInProgress to false
	SetUpdateStatus(ctx, "false", vmm)

	updateStatus, err = vmm.VCHUpdateStatus(ctx)
	if err != nil {
		t.Fatalf("ERROR: %s", err)
	}
	assert.False(t, updateStatus, "updateStatus should be false since UpdateInProgress is set to false")

	// Set UpdateInProgress to true
	SetUpdateStatus(ctx, "true", vmm)

	updateStatus, err = vmm.VCHUpdateStatus(ctx)
	if err != nil {
		t.Fatalf("ERROR: %s", err)
	}
	assert.True(t, updateStatus, "updateStatus should be true since UpdateInProgress is set to true")

	// Set UpdateInProgress to NonBool
	SetUpdateStatus(ctx, "NonBool", vmm)

	updateStatus, err = vmm.VCHUpdateStatus(ctx)
	if assert.Error(t, err, "An error was expected") {
		assert.Contains(t, err.Error(), "failed to parse", "Error msg should contain 'failed to parse' since UpdateInProgress is set to NonBool")
	}
}

// TestSetVCHUpdateStatus tests if SetVCHUpdateStatus() could set the VCH upgrade/configure status correctly
func TestSetVCHUpdateStatus(t *testing.T) {
	ctx := context.Background()

	// Nothing VC specific in this test, so we use the simpler ESX model
	model := simulator.ESX()
	defer model.Remove()
	err := model.Create()
	if err != nil {
		t.Fatal(err)
	}

	server := model.Service.NewServer()
	defer server.Close()
	client, err := govmomi.NewClient(ctx, server.URL, true)
	if err != nil {
		t.Fatal(err)
	}

	// Any VM will do
	finder := find.NewFinder(client.Client, false)
	vmo, err := finder.VirtualMachine(ctx, "/ha-datacenter/vm/*_VM0")
	if err != nil {
		t.Fatal(err)
	}

	config := &session.Config{
		Service:        server.URL.String(),
		Insecure:       true,
		Keepalive:      time.Duration(5) * time.Minute,
		DatacenterPath: "",
		DatastorePath:  "/ha-datacenter/datastore/*",
		HostPath:       "/ha-datacenter/host/*/*",
		PoolPath:       "/ha-datacenter/host/*/Resources",
	}

	s, err := session.NewSession(config).Connect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	s.Populate(ctx)
	vmm := NewVirtualMachine(ctx, s, vmo.Reference())

	// Set UpdateInProgress to true and then check status
	err = vmm.SetVCHUpdateStatus(ctx, true)
	if err != nil {
		t.Fatalf("ERROR: %s", err)
	}

	info, err := vmm.FetchExtraConfig(ctx)
	if err != nil {
		t.Fatalf("ERROR: %s", err)
	}

	v, ok := info[UpdateStatus]
	if ok {
		assert.Equal(t, "true", v, "UpdateInProgress should be true")
	} else {
		t.Fatal("ERROR: UpdateInProgress does not exist in ExtraConfig")
	}

	// Set UpdateInProgress to false and then check status
	err = vmm.SetVCHUpdateStatus(ctx, false)
	if err != nil {
		t.Fatalf("ERROR: %s", err)
	}

	info, err = vmm.FetchExtraConfig(ctx)
	if err != nil {
		t.Fatalf("ERROR: %s", err)
	}

	v, ok = info[UpdateStatus]
	if ok {
		assert.Equal(t, "false", v, "UpdateInProgress should be false")
	} else {
		t.Fatal("ERROR: UpdateInProgress does not exist in ExtraConfig")
	}
}
