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

package management

import (
	"context"
	"fmt"
	"io/ioutil"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/vic/lib/config"
	"github.com/vmware/vic/lib/install/data"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/vsphere/extraconfig"
	"github.com/vmware/vic/pkg/vsphere/session"
	"github.com/vmware/vic/pkg/vsphere/test"
	"github.com/vmware/vic/pkg/vsphere/vm"
)

func TestGuestInfoSecret(t *testing.T) {
	op := trace.NewOperation(context.Background(), "TestGuestInfoSecret")

	for i, m := range []*simulator.Model{simulator.ESX(), simulator.VPX()} {

		defer m.Remove()
		err := m.Create()
		if err != nil {
			t.Fatal(err)
		}

		server := m.Service.NewServer()
		defer server.Close()

		var s *session.Session
		if i == 0 {
			s, err = test.SessionWithESX(op, server.URL.String())
		} else {
			s, err = test.SessionWithVPX(op, server.URL.String())
		}
		if err != nil {
			t.Fatal(err)
		}

		name := "my-vm"
		vmx := fmt.Sprintf("%s/%s.vmx", name, name)
		ds := s.Datastore
		secretKey, err := extraconfig.NewSecretKey()
		if err != nil {
			t.Fatal(err)
		}

		spec := types.VirtualMachineConfigSpec{
			Name:    name,
			GuestId: string(types.VirtualMachineGuestOsIdentifierOtherGuest),
			Files: &types.VirtualMachineFileInfo{
				VmPathName: fmt.Sprintf("[%s] %s", ds.Name(), vmx),
			},
			ExtraConfig: []types.BaseOptionValue{
				&types.OptionValue{
					Key:   extraconfig.GuestInfoSecretKey,
					Value: secretKey.String(),
				},
			},
		}

		task, err := s.VMFolder.CreateVM(op, spec, s.Pool, nil)
		if err != nil {
			t.Fatal(err)
		}
		err = task.Wait(op)
		if err != nil {
			t.Fatal(err)
		}

		d := &Dispatcher{
			session:    s,
			op:         op,
			vmPathName: name,
		}

		// Attempt to extract the secret without setting the session's datastore
		secret, err := d.GuestInfoSecret(name, name, nil)
		assert.Nil(t, secret)
		assert.Equal(t, err, errNilDatastore)

		// Attempt to extract the secret with an empty .vmx file
		// TODO: simulator should write ExtraConfig to the .vmx file
		secret, err = d.GuestInfoSecret(name, name, ds)
		assert.Nil(t, secret)
		assert.Equal(t, err, errSecretKeyNotFound)

		// Write malformed key-value pairs
		dir := simulator.Map.Get(ds.Reference()).(*simulator.Datastore).Info.(*types.LocalDatastoreInfo).Path
		text := fmt.Sprintf("foo.bar = \"baz\"\n%s \"%s\"\n", extraconfig.GuestInfoSecretKey, secretKey.String())
		if err = ioutil.WriteFile(path.Join(dir, vmx), []byte(text), 0); err != nil {
			t.Fatal(err)
		}

		// Attempt to extract the secret from an incorrectly populated .vmx file
		secret, err = d.GuestInfoSecret(name, name, ds)
		assert.Nil(t, secret)
		assert.Error(t, err)

		// Write an invalid key that only prefix-matches the secret key
		text = fmt.Sprintf("%s = \"%s\"\n", extraconfig.GuestInfoSecretKey+"1", secretKey.String())
		if err = ioutil.WriteFile(path.Join(dir, vmx), []byte(text), 0); err != nil {
			t.Fatal(err)
		}

		// Attempt to extract the secret from an incorrectly populated .vmx file
		secret, err = d.GuestInfoSecret(name, name, ds)
		assert.Nil(t, secret)
		assert.Equal(t, err, errSecretKeyNotFound)

		// Write valid key-value pairs
		text = fmt.Sprintf("foo.bar = \"baz\"\n%s = \"%s\"\n", extraconfig.GuestInfoSecretKey, secretKey.String())
		if err = ioutil.WriteFile(path.Join(dir, vmx), []byte(text), 0); err != nil {
			t.Fatal(err)
		}

		// Extract the secret from a correctly populated .vmx file
		secret, err = d.GuestInfoSecret(name, name, ds)
		assert.NoError(t, err)
		assert.Equal(t, secret.String(), secretKey.String())
	}
}

func testUpdateResources(ctx context.Context, sess *session.Session, conf *config.VirtualContainerHostConfigSpec, vConf *data.InstallerData, hasErr bool, t *testing.T) {
	op := trace.NewOperation(ctx, "testUpdateResources")

	d := &Dispatcher{
		session: sess,
		op:      op,
		isVC:    sess.IsVC(),
		force:   false,
	}

	appliance, err := sess.Finder.VirtualMachine(op, conf.Name)
	if err != nil {
		t.Errorf("Didn't find appliance vm: %s", err)
	}
	d.appliance = vm.NewVirtualMachine(op, sess, appliance.Reference())

	settings := &data.InstallerData{}
	limit := int64(1024)
	settings.VCHSize.CPU.Limit = &limit
	settings.VCHSize.Memory.Limit = &limit
	settings.VCHSizeIsSet = true

	if err = d.updateResourceSettings(conf.Name, settings); err != nil {
		t.Errorf("Failed to update resources: %s", err)
	}
	newSettings, err := d.getPoolResourceSettings(d.vchPool)
	if err != nil {
		t.Errorf("Failed to get pool resources: %s", err)
	}

	assert.Equal(t, settings.VCHSize.CPU.Limit, newSettings.CPU.Limit, "Cpu limit is not updated")
	assert.Equal(t, 0, d.oldVCHResources.CPU.Limit, "Old Cpu limit is not as expected")

	d.oldVCHResources = nil
	if err = d.updateResourceSettings(conf.Name, settings); err != nil {
		t.Errorf("Failed to update resources: %s", err)
	}
	assert.Equal(t, d.oldVCHResources, nil, "should not update for same resource settings")

	settings2 := &data.InstallerData{}
	limit = int64(2048)
	settings2.VCHSize.CPU.Limit = &limit
	settings2.VCHSize.Memory.Limit = &limit
	settings2.VCHSizeIsSet = false
	if err = d.updateResourceSettings(conf.Name, settings); err != nil {
		t.Errorf("Failed to update resources: %s", err)
	}
	assert.Equal(t, d.oldVCHResources, nil, "should not update if VCH size is not set")

	settings2.VCHSizeIsSet = true
	if err = d.updateResourceSettings(conf.Name, settings); err != nil {
		t.Errorf("Failed to update resources: %s", err)
	}
	newSettings, err = d.getPoolResourceSettings(d.vchPool)
	if err != nil {
		t.Errorf("Failed to get pool resources: %s", err)
	}

	assert.Equal(t, settings2.VCHSize.CPU.Limit, newSettings.CPU.Limit, "Cpu limit is not updated")

	if err = d.rollbackResourceSettings(conf.Name, settings); err != nil {
		t.Errorf("Rollback failed: %s", err)
	}
	newSettings, err = d.getPoolResourceSettings(d.vchPool)
	if err != nil {
		t.Errorf("Failed to get pool resources: %s", err)
	}

	assert.Equal(t, settings.VCHSize.CPU.Limit, newSettings.CPU.Limit, "Cpu limit is not rollback")
}
