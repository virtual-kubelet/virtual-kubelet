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
	"net/url"
	"path"
	"testing"

	log "github.com/Sirupsen/logrus"

	"github.com/stretchr/testify/require"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/vic/lib/config"
	"github.com/vmware/vic/lib/install/data"
	"github.com/vmware/vic/lib/install/validate"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/vsphere/session"
)

func TestMain(t *testing.T) {
	log.SetLevel(log.DebugLevel)
	op := trace.NewOperation(context.Background(), "TestMain")

	for i, model := range []*simulator.Model{simulator.ESX(), simulator.VPX()} {
		t.Logf("%d", i)
		defer model.Remove()
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
		installSettings := &data.InstallerData{}
		cpu := int64(1)
		memory := int64(1024)
		installSettings.ApplianceSize.CPU.Limit = &cpu
		installSettings.ApplianceSize.Memory.Limit = &memory

		validator, err := validate.NewValidator(op, input)
		if err != nil {
			t.Fatalf("Failed to validator: %s", err)
		}

		conf, err := validator.Validate(op, input)
		if err != nil {
			log.Errorf("Failed to validate conf: %s", err)
			validator.ListIssues(op)
		}

		testCreateNetwork(op, validator.Session, conf, t)

		testCreateVolumeStores(op, validator.Session, conf, false, t)
		testDeleteVolumeStores(op, validator.Session, conf, 1, t)
		errConf := &config.VirtualContainerHostConfigSpec{}
		*errConf = *conf
		errConf.VolumeLocations = make(map[string]*url.URL)
		errConf.VolumeLocations["volume-store"], _ = url.Parse("ds://store_not_exist/volumes/test")
		testCreateVolumeStores(op, validator.Session, errConf, true, t)

		// FIXME: (pull vic/7088) have to make another VCH config from validator for negative test case and cleanup test
		// If we re-use the previous validator like we did in the earlier test (*errConf = *conf), it's not a deep copy of conf
		// This conf will get modified by appliance creation and cleanup test, and we can't test create appliance in positive case
		// The other way around, if we test positive case first, the VCH data and session data are modified, so we are not able to test the negative case
		conf2, err := validator.Validate(op, input)
		conf2.ImageStores[0].Host = "http://non-exist"
		testCreateAppliance(op, validator.Session, conf2, installSettings, true, t)
		testCleanup(op, validator.Session, conf2, t)

		testCreateAppliance(op, validator.Session, conf, installSettings, false, t)

		// cannot run test for func not implemented in vcsim: ResourcePool:resourcepool-24 does not implement: UpdateConfig
		// testUpdateResources(ctx, validator.Session, conf, installSettings, false, t)
	}
}

func getESXData(esxURL *url.URL) *data.Data {
	result := data.NewData()
	result.URL = esxURL
	result.DisplayName = "test001"
	result.ComputeResourcePath = "/ha-datacenter/host/localhost.localdomain/Resources"
	result.ImageDatastorePath = "LocalDS_0"
	result.BridgeNetworkName = "bridge"
	result.ManagementNetwork.Name = "VM Network"
	result.PublicNetwork.Name = "VM Network"
	result.VolumeLocations = make(map[string]*url.URL)
	testURL := &url.URL{
		Host: "LocalDS_0",
		Path: "volumes/test",
	}
	result.VolumeLocations["volume-store"] = testURL

	return result
}

func getVPXData(vcURL *url.URL) *data.Data {
	result := data.NewData()
	result.URL = vcURL
	result.DisplayName = "test001"
	result.ComputeResourcePath = "/DC0/host/DC0_C0/Resources"
	result.ImageDatastorePath = "LocalDS_0"
	result.PublicNetwork.Name = "VM Network"
	result.BridgeNetworkName = "DC0_DVPG0"
	result.VolumeLocations = make(map[string]*url.URL)
	testURL := &url.URL{
		Host: "LocalDS_0",
		Path: "volumes/test",
	}
	result.VolumeLocations["volume-store"] = testURL

	return result
}

func testCreateNetwork(op trace.Operation, sess *session.Session, conf *config.VirtualContainerHostConfigSpec, t *testing.T) {
	d := &Dispatcher{
		session: sess,
		op:      op,
		isVC:    sess.IsVC(),
		force:   false,
	}

	err := d.createBridgeNetwork(conf)
	if err != nil {
		t.Error(err)
	}

	if d.isVC {
		bnet := conf.ExecutorConfig.Networks[conf.BridgeNetwork]
		delete(conf.ExecutorConfig.Networks, conf.BridgeNetwork)

		err = d.createBridgeNetwork(conf)
		if err == nil {
			t.Error("expected error")
		}

		conf.ExecutorConfig.Networks[conf.BridgeNetwork] = bnet
	}
}

func testCreateVolumeStores(op trace.Operation, sess *session.Session, conf *config.VirtualContainerHostConfigSpec, hasErr bool, t *testing.T) {
	d := &Dispatcher{
		session: sess,
		op:      op,
		isVC:    sess.IsVC(),
		force:   false,
	}

	err := d.createVolumeStores(conf)
	if hasErr && err != nil {
		t.Logf("Got exepcted err: %s", err)
		return
	}
	if hasErr {
		t.Errorf("Should have error, but got success")
		return
	}
	if err != nil {
		t.Errorf("Unexpected error: %s", err)
	}
}

func testDeleteVolumeStores(op trace.Operation, sess *session.Session, conf *config.VirtualContainerHostConfigSpec, numVols int, t *testing.T) {
	d := &Dispatcher{
		session: sess,
		op:      op,
		isVC:    sess.IsVC(),
		force:   true,
	}

	if removed := d.deleteVolumeStoreIfForced(conf, nil); removed != numVols {
		t.Errorf("Did not successfully remove all specified volumes")
	}

}

func testCleanup(op trace.Operation, sess *session.Session, conf *config.VirtualContainerHostConfigSpec, t *testing.T) {
	d := &Dispatcher{
		session: sess,
		op:      op,
		isVC:    sess.IsVC(),
		force:   true,
	}

	err := d.cleanupEmptyPool(conf)
	if err != nil {
		t.Fatal(err)
		return
	}

	err = d.cleanupBridgeNetwork(conf)
	if err != nil {
		t.Fatal(err)
		return
	}

	d.deleteFolder(d.session.VCHFolder)

	// in this case we should expect the folder to be gone.
	folder, err := d.session.Finder.Folder(d.op, path.Join(d.session.VMFolder.InventoryPath, conf.Name))
	require.Error(t, err)
	require.Nil(t, folder)
}

func testCreateAppliance(op trace.Operation, sess *session.Session, conf *config.VirtualContainerHostConfigSpec, vConf *data.InstallerData, hasErr bool, t *testing.T) {
	d := &Dispatcher{
		session: sess,
		op:      op,
		isVC:    sess.IsVC(),
		force:   false,
	}

	err := d.createPool(conf, vConf)
	if err != nil {
		if hasErr {
			t.Logf("Got expected err: %s", err)
		} else {
			t.Fatal(err)
		}
		return
	}

	err = d.createAppliance(conf, vConf)
	if err != nil {
		if hasErr {
			t.Logf("Got expected err: %s", err)
		} else {
			t.Fatal(err)
		}
		return
	}

	if hasErr {
		t.Errorf("No error when error is expected.")
	}

	// check the folder structure and vch location here
	if d.isVC {
		vmFolderPath := d.session.VMFolder.InventoryPath
		folder, err := d.session.Finder.Folder(d.op, vmFolderPath)
		require.NoError(t, err)
		t.Logf("Found VMFolderPath: %s", vmFolderPath)

		children, err := folder.Children(d.op)
		require.NoError(t, err)
		for _, child := range children {
			obj, err := d.session.Finder.ObjectReference(d.op, child.Reference())
			require.NoError(t, err)

			if folder, ok := obj.(*object.Folder); ok {
				t.Log("\n")
				t.Logf("FOUND folder info: %s", folder)
				t.Logf("INVENTORY PATH: %s", folder.InventoryPath)
				t.Log("\n")
			}

		}

		folder, err = d.session.Finder.Folder(d.op, path.Join(vmFolderPath, conf.Name))
		require.NoError(t, err)
		require.NotNil(t, folder)
		require.Equal(t, folder.Name(), conf.Name)

		// also check for the correct location of the VCH
		vchVM, err := d.session.Finder.VirtualMachine(d.op, path.Join(vmFolderPath, conf.Name, conf.Name))
		require.NoError(t, err)
		require.NotNil(t, vchVM)

	}
}
