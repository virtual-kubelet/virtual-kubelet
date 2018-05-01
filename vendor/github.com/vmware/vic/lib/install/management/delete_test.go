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
	"io/ioutil"
	"net/url"
	"os"
	"path"
	"testing"

	log "github.com/Sirupsen/logrus"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/vic/lib/config"
	"github.com/vmware/vic/lib/install/data"
	"github.com/vmware/vic/lib/install/validate"
	"github.com/vmware/vic/pkg/errors"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/vsphere/session"
)

func TestDelete(t *testing.T) {
	log.SetLevel(log.DebugLevel)
	trace.Logger.Level = log.DebugLevel
	ctx := context.Background()
	op := trace.NewOperation(ctx, "TestDelete")

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
		installSettings.ResourcePoolPath = path.Join(input.ComputeResourcePath, input.DisplayName)

		validator, err := validate.NewValidator(ctx, input)
		if err != nil {
			t.Errorf("Failed to validator: %s", err)
		}

		conf, err := validator.Validate(ctx, input)
		if err != nil {
			log.Errorf("Failed to validate conf: %s", err)
			validator.ListIssues(op)
		}

		testCreateNetwork(op, validator.Session, conf, t)
		createAppliance(ctx, validator.Session, conf, installSettings, false, t)

		testNewVCHFromCompute(input.ComputeResourcePath, input.DisplayName, validator, t)
		//		testUpgrade(input.ComputeResourcePath, input.DisplayName, validator, installSettings, t) TODO: does not implement: CreateSnapshot_Task
		testDeleteVCH(validator, conf, t)

		testDeleteDatastoreFiles(validator, t)
	}
}

func testUpgrade(computePath string, name string, v *validate.Validator, settings *data.InstallerData, t *testing.T) {
	// TODO: add tests for rollback after snapshot func is added in vcsim
	d := &Dispatcher{
		session: v.Session,
		op:      trace.FromContext(v.Context, "testUpgrade"),
		isVC:    v.Session.IsVC(),
		force:   false,
	}
	vch, err := d.NewVCHFromComputePath(computePath, name, v)
	if err != nil {
		t.Errorf("Failed to get VCH: %s", err)
		return
	}
	t.Logf("Got VCH %s, path %s", vch, path.Dir(vch.InventoryPath))
	conf, err := d.GetVCHConfig(vch)
	if err != nil {

		t.Errorf("Failed to get vch configuration: %s", err)
	}
	if err := d.Configure(vch, conf, settings, false); err != nil {
		t.Errorf("Failed to upgrade: %s", err)
	}
}

func createAppliance(ctx context.Context, sess *session.Session, conf *config.VirtualContainerHostConfigSpec, vConf *data.InstallerData, hasErr bool, t *testing.T) {
	var err error

	d := &Dispatcher{
		session: sess,
		op:      trace.FromContext(ctx, "createAppliance"),
		isVC:    sess.IsVC(),
		force:   false,
	}

	err = d.createPool(conf, vConf)
	if err != nil {
		t.Fatal(err)
	}

	err = d.createAppliance(conf, vConf)
	if err != nil {
		t.Fatal(err)
	}
}

func testNewVCHFromCompute(computePath string, name string, v *validate.Validator, t *testing.T) {
	d := &Dispatcher{
		session: v.Session,
		op:      trace.FromContext(v.Context, "testNewVCHFromCompute"),
		isVC:    v.Session.IsVC(),
		force:   false,
	}
	vch, err := d.NewVCHFromComputePath(computePath, name, v)
	if err != nil {
		t.Errorf("Failed to get VCH: %s", err)
		return
	}

	if d.session.Cluster == nil {
		t.Errorf("Failed to set cluster: %s", err)
		return
	}

	t.Logf("Got VCH %s, path %s", vch, path.Dir(vch.InventoryPath))
}

func testDeleteVCH(v *validate.Validator, conf *config.VirtualContainerHostConfigSpec, t *testing.T) {
	d := &Dispatcher{
		session: v.Session,
		op:      trace.FromContext(v.Context, "testDeleteVCH"),
		isVC:    v.Session.IsVC(),
		force:   false,
	}
	// failed to get vm FolderName, that will eventually cause panic in simulator to delete empty datastore file
	if err := d.DeleteVCH(conf, nil, nil); err != nil {
		t.Errorf("Failed to get VCH: %s", err)
		return
	}
	t.Logf("Successfully deleted VCH")
	// check images directory is removed
	dsPath := "[LocalDS_0] VIC"
	_, err := d.lsFolder(v.Session.Datastore, dsPath)
	if err != nil {
		if !types.IsFileNotFound(err) {
			t.Errorf("Failed to browse folder %s: %s", dsPath, errors.ErrorStack(err))
		}
		t.Logf("Images Folder is not found")
	}

	// check appliance vm is deleted
	vm, err := d.findApplianceByID(conf)
	if vm != nil {
		t.Errorf("Should not found vm %s", vm.Reference())
	}

	if err != nil {
		t.Errorf("Unexpected error to get appliance VM: %s", err)
	}
	// delete VM does not clean up resource pool after VM is removed, so resource pool could not be removed
}

func testDeleteDatastoreFiles(v *validate.Validator, t *testing.T) {
	d := &Dispatcher{
		session: v.Session,
		op:      trace.FromContext(v.Context, "testDeleteDatastoreFiles"),
		isVC:    v.Session.IsVC(),
		force:   false,
	}

	ds := v.Session.Datastore
	m := object.NewFileManager(ds.Client())
	err := m.MakeDirectory(v.Context, ds.Path("Test/folder/data"), v.Session.Datacenter, true)
	if err != nil {
		t.Errorf("Failed to create datastore dir: %s", err)
		return
	}
	err = m.MakeDirectory(v.Context, ds.Path("Test/folder/metadata"), v.Session.Datacenter, true)
	if err != nil {
		t.Errorf("Failed to create datastore dir: %s", err)
		return
	}
	err = m.MakeDirectory(v.Context, ds.Path("Test/folder/file"), v.Session.Datacenter, true)
	if err != nil {
		t.Errorf("Failed to create datastore dir: %s", err)
		return
	}

	isVSAN := d.isVSAN(ds)
	t.Logf("datastore is vsan: %t", isVSAN)

	if err = createDatastoreFiles(d, ds, t); err != nil {
		t.Errorf("Failed to upload file: %s", err)
		return
	}

	fm := ds.NewFileManager(d.session.Datacenter, true)
	if err = d.deleteFilesIteratively(fm, ds, ds.Path("Test")); err != nil {
		t.Errorf("Failed to delete recursively: %s", err)
	}

	err = m.MakeDirectory(v.Context, ds.Path("Test/folder/data"), v.Session.Datacenter, true)
	if err != nil {
		t.Errorf("Failed to create datastore dir: %s", err)
		return
	}

	if err = createDatastoreFiles(d, ds, t); err != nil {
		t.Errorf("Failed to upload file: %s", err)
		return
	}

	if _, err = d.deleteDatastoreFiles(ds, "Test", true); err != nil {
		t.Errorf("Failed to delete recursively: %s", err)
	}
}

func createDatastoreFiles(d *Dispatcher, ds *object.Datastore, t *testing.T) error {
	tmpfile, err := ioutil.TempFile("", "tempDatastoreFile.vmdk")
	if err != nil {
		t.Errorf("Failed to create file: %s", err)
		return err
	}

	defer os.Remove(tmpfile.Name()) // clean up

	if err = ds.UploadFile(d.op, tmpfile.Name(), "Test/folder/data/temp.vmdk", nil); err != nil {
		t.Errorf("Failed to upload file %q: %s", "Test/folder/data/temp.vmdk", err)
		return err
	}
	if err = ds.UploadFile(d.op, tmpfile.Name(), "Test/folder/tempMetadata", nil); err != nil {
		t.Errorf("Failed to upload file %q: %s", "Test/folder/tempMetadata", err)
		return err
	}
	return nil
}
