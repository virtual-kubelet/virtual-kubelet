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

package management

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"testing"

	log "github.com/Sirupsen/logrus"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/vic/lib/config"
	"github.com/vmware/vic/lib/install/data"
	"github.com/vmware/vic/lib/install/validate"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/vsphere/extraconfig"
	"github.com/vmware/vic/pkg/vsphere/session"
	"github.com/vmware/vic/pkg/vsphere/tasks"
)

func TestFinder(t *testing.T) {
	log.SetLevel(log.DebugLevel)
	trace.Logger.Level = log.DebugLevel
	ctx := context.Background()

	for i, model := range []*simulator.Model{simulator.ESX(), simulator.VPX()} {
		t.Logf("%d", i)
		defer model.Remove()
		if i == 1 {
			model.Datacenter = 2
			model.Cluster = 2
			model.Host = 2
			model.Pool = 0
		}
		err := model.Create()
		if err != nil {
			t.Fatal(err)
		}

		s := model.Service.NewServer()
		defer s.Close()

		s.URL.User = url.UserPassword("user", "pass")
		s.URL.Path = ""
		t.Logf("server URL: %s", s.URL)

		var input *data.Data
		if i == 0 {
			input = getESXData(s.URL)
		} else {
			input = getVPXData(s.URL)
		}
		if err != nil {
			t.Fatal(err)
		}
		validator, err := validate.NewValidator(ctx, input)
		if err != nil {
			t.Errorf("Failed to create validator: %s", err)
		}
		if _, err = validator.ValidateTarget(ctx, input, false); err != nil {
			t.Logf("Got expected error to validate target: %s", err)
		}
		if _, err = validator.ValidateTarget(ctx, input, true); err != nil {
			t.Errorf("Failed to valiate target: %s", err)
		}
		prefix := fmt.Sprintf("p%d-", i)
		if err = createTestData(ctx, validator.Session(), prefix); err != nil {
			t.Errorf("Failed to create test data: %s", err)
		}

		found := testSearchVCHs(ctx, t, validator, false)
		if found != 0 {
			t.Errorf("found %d VCHs, expected %d", found, 0)
		}

		found = testSearchVCHs(ctx, t, validator, true)
		expect := 1 // 1 VCH per Resource pool
		if model.Host != 0 {
			expect *= (model.Host + model.Cluster) * 2
		}
		if found != expect {
			t.Errorf("found %d VCHs, expected %d", found, expect)
		}
	}
}

func testSearchVCHs(ctx context.Context, t *testing.T, v *validate.Validator, expect bool) int {
	d := &Dispatcher{
		session: v.Session(),
		op:      trace.FromContext(ctx, "testSearchVCHs"),
		isVC:    v.Session().IsVC(),
	}

	if expect {
		// Add guestinfo so isVCH() returns true for all VMs
		vms, err := d.session.Finder.VirtualMachineList(d.op, "/...")
		if err != nil {
			t.Fatal(err)
		}

		for _, vm := range vms {
			ref := vm.Reference()
			svm := simulator.Map.Get(ref).(*simulator.VirtualMachine)

			template := config.VirtualContainerHostConfigSpec{}
			key := extraconfig.CalculateKey(template, "ExecutorConfig.ExecutorConfigCommon.ID", "")
			svm.Config.ExtraConfig = []types.BaseOptionValue{&types.OptionValue{
				Key:   key,
				Value: ref.String(),
			}}
		}
	} else {
		_, err := d.SearchVCHs("enoent") // NotFound
		if err != nil {
			t.Error(err)
		}

		n, err := d.SearchVCHs("/")
		if err != nil {
			t.Error(err)
		}

		if len(n) != 0 {
			t.Errorf("unexpected: %d", len(n))
		}
	}

	vchs, err := d.SearchVCHs("")
	if err != nil {
		t.Errorf("Failed to search vchs: %s", err)
	}
	n := len(vchs)
	t.Logf("Found %d VCHs without a compute-path", n)
	nexpect := 1
	if d.isVC {
		nexpect = 2 // 1 for the top-level vApp, VC only
	}

	for _, vm := range vchs {
		// Find with --compute-resource
		// The VM HostSystem's parent will be a cluster or standalone compute resource
		host, err := vm.HostSystem(d.op)
		if err != nil {
			t.Fatal(err)
		}

		c := property.DefaultCollector(vm.VirtualMachine.Client())
		var me mo.ManagedEntity

		err = c.RetrieveOne(d.op, host.Reference(), []string{"parent"}, &me)
		if err != nil {
			t.Fatal(err)
		}

		obj, err := d.session.Finder.Element(d.op, *me.Parent)
		if err != nil {
			t.Fatal(err)
		}

		name := path.Base(obj.Path)

		paths := []string{
			obj.Path,             // "/dc1/cluster1"
			name,                 // "cluster1"
			path.Join(".", name), // "./cluster1"
		}

		for _, path := range paths {
			vchs, err := d.SearchVCHs(path)
			if err != nil {
				t.Errorf("SearchVCHs(%s): %s", path, err)
			}

			if len(vchs) != nexpect {
				t.Errorf("Found %d VCHs with compute-path=%s", len(vchs), path)
			}
		}
	}

	return n
}

func createTestData(ctx context.Context, sess *session.Session, prefix string) error {
	dcs, err := sess.Finder.DatacenterList(ctx, "*")
	if err != nil {
		return err
	}
	kind := rpNode
	if sess.IsVC() {
		kind = vappNode
	}
	for _, dc := range dcs {
		sess.Config.DatacenterPath = dc.InventoryPath
		sess.Populate(ctx)

		resources := &Node{
			Kind: rpNode,
			Name: prefix + "Root",
			Children: []*Node{
				{
					Kind: rpNode,
					Name: prefix + "pool1",
					Children: []*Node{
						{
							Kind: vmNode,
							Name: prefix + "pool1",
						},
						{
							Kind: rpNode,
							Name: prefix + "pool1-2",
							Children: []*Node{
								{
									Kind: kind,
									Name: prefix + "pool1-2-1",
									Children: []*Node{
										{
											Kind: vmNode,
											Name: prefix + "vch1-2-1",
										},
									},
								},
							},
						},
					},
				},
				{
					Kind: vmNode,
					Name: prefix + "vch2",
				},
			},
		}
		if err = createResources(ctx, sess, resources); err != nil {
			return err
		}
		if sess.IsVC() {
			// Test with a top-level VApp
			vapp := &Node{
				Kind: vappNode,
				Name: prefix + "VApp",
			}
			if err = createResources(ctx, sess, vapp); err != nil {
				return err
			}
		}
	}
	return nil
}

type nodeKind string

const (
	vmNode   = nodeKind("VM")
	rpNode   = nodeKind("RP")
	vappNode = nodeKind("VAPP")
)

type Node struct {
	Kind     nodeKind
	Name     string
	Children []*Node
}

func createResources(ctx context.Context, sess *session.Session, node *Node) error {
	rootPools, err := sess.Finder.ResourcePoolList(ctx, "Resources")
	if err != nil {
		return err
	}
	for _, pool := range rootPools {
		base := path.Base(path.Dir(pool.InventoryPath))
		log.Debugf("root pool base name %q", base)
		if err = createNodes(ctx, sess, pool, node, base); err != nil {
			return err
		}
	}
	return nil
}

func createNodes(ctx context.Context, sess *session.Session, pool *object.ResourcePool, node *Node, base string) error {
	log.Debugf("create node %+v", node)
	if node == nil {
		return nil
	}
	spec := types.DefaultResourceConfigSpec()
	node.Name = fmt.Sprintf("%s-%s", base, node.Name)
	switch node.Kind {
	case rpNode:
		child, err := pool.Create(ctx, node.Name, spec)
		if err != nil {
			return err
		}
		for _, childNode := range node.Children {
			return createNodes(ctx, sess, child, childNode, base)
		}
	case vappNode:
		confSpec := simulator.NewVAppConfigSpec()
		vapp, err := pool.CreateVApp(ctx, node.Name, spec, confSpec, nil)
		if err != nil {
			return err
		}
		config := types.VirtualMachineConfigSpec{
			Name:    node.Name,
			GuestId: string(types.VirtualMachineGuestOsIdentifierOtherGuest),
			Files: &types.VirtualMachineFileInfo{
				VmPathName: fmt.Sprintf("[LocalDS_0] %s", node.Name),
			},
		}
		if _, err = tasks.WaitForResult(ctx, func(ctx context.Context) (tasks.Task, error) {
			return vapp.CreateChildVM(ctx, config, nil)
		}); err != nil {
			return err
		}
	case vmNode:
		config := types.VirtualMachineConfigSpec{
			Name:    node.Name,
			GuestId: string(types.VirtualMachineGuestOsIdentifierOtherGuest),
			Files: &types.VirtualMachineFileInfo{
				VmPathName: fmt.Sprintf("[LocalDS_0] %s", node.Name),
			},
		}
		if _, err := tasks.WaitForResult(ctx, func(ctx context.Context) (tasks.Task, error) {
			return sess.VMFolder.CreateVM(ctx, config, pool, nil)
		}); err != nil {
			return err
		}
	default:
		return nil
	}
	return nil
}
