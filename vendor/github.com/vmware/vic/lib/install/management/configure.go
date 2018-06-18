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
	"bufio"
	"context"
	"fmt"
	"io"
	"path"
	"reflect"
	"strings"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/task"
	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/vic/lib/config"
	"github.com/vmware/vic/lib/install/data"
	"github.com/vmware/vic/lib/install/opsuser"
	"github.com/vmware/vic/pkg/errors"
	"github.com/vmware/vic/pkg/retry"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/vsphere/datastore"
	"github.com/vmware/vic/pkg/vsphere/extraconfig"
	"github.com/vmware/vic/pkg/vsphere/extraconfig/vmomi"
	"github.com/vmware/vic/pkg/vsphere/tasks"
	"github.com/vmware/vic/pkg/vsphere/vm"
)

const (
	// deprecated snapshot name prefix
	UpgradePrefix = "upgrade for"
	// new snapshot name for upgrade and configure are using same process
	ConfigurePrefix = "reconfigure for"
)

var (
	errSecretKeyNotFound = fmt.Errorf("unable to find guestinfo secret")
	errNilDatastore      = fmt.Errorf("session's datastore is not set")
)

// Configure will try to reconfigure vch appliance. If failed will try to roll back to original status.
func (d *Dispatcher) Configure(vch *vm.VirtualMachine, conf *config.VirtualContainerHostConfigSpec, settings *data.InstallerData, isConfigureOp bool) (err error) {
	defer trace.End(trace.Begin(conf.Name, d.op))

	d.appliance = vch

	// update the displayname to the actual folder name used
	if d.vmPathName, err = d.appliance.FolderName(d.op); err != nil {
		d.op.Errorf("Failed to get canonical name for appliance: %s", err)
		return err
	}

	ds, err := d.session.Finder.Datastore(d.op, conf.ImageStores[0].Host)
	if err != nil {
		err = errors.Errorf("Failed to find image datastore %q", conf.ImageStores[0].Host)
		return err
	}
	d.session.Datastore = ds
	d.setDockerPort(conf, settings)

	if len(settings.ImageFiles) > 0 {
		// Need to update iso files
		if err = d.uploadImages(settings.ImageFiles); err != nil {
			return errors.Errorf("Uploading images failed with %s. Exiting...", err)
		}
		conf.BootstrapImagePath = fmt.Sprintf("[%s] %s/%s", conf.ImageStores[0].Host, d.vmPathName, settings.BootstrapISO)
	}

	if err = d.updateResourceSettings(conf.Name, settings); err != nil {
		err = errors.Errorf("Failed to reconfigure resources: %s", err)
		return err
	}

	defer func() {
		if err != nil {
			d.rollbackResourceSettings(conf.Name, settings)
		}
	}()

	// ensure that we wait for components to come up
	for _, s := range conf.ExecutorConfig.Sessions {
		s.Started = ""
	}

	snapshotName := fmt.Sprintf("%s %s", ConfigurePrefix, conf.Version.BuildNumber)
	snapshotName = strings.TrimSpace(snapshotName)

	// check for old snapshot
	// #nosec: Errors unhandled.
	oldSnapshot, _ := d.appliance.GetCurrentSnapshotTree(d.op)

	newSnapshotRef, err := d.tryCreateSnapshot(snapshotName, "configure snapshot")
	if err != nil {
		d.deleteUpgradeImages(ds, settings)
		return err
	}

	err = d.update(conf, settings, isConfigureOp)

	// If successful try to grant permissions to the ops-user
	if err == nil && conf.ShouldGrantPerms() {
		err = opsuser.GrantOpsUserPerms(d.op, d.session.Vim25(), conf)
		if err != nil {
			// Update error message and fall through to roll back
			err = errors.Errorf("Failed to grant permissions to ops-user, failure: %s", err)
		}
	}

	if err != nil {
		// Roll back
		d.op.Errorf("Failed to upgrade: %s", err)
		d.op.Infof("Rolling back upgrade")

		if rerr := d.rollback(conf, snapshotName, settings); rerr != nil {
			d.op.Errorf("Failed to revert appliance to snapshot: %s", rerr)
			return err
		}
		d.op.Infof("Appliance is rolled back to old version")

		d.deleteUpgradeImages(ds, settings)
		d.retryDeleteSnapshotByRef(newSnapshotRef, snapshotName, conf.Name)

		// return the error message for upgrade
		return err
	}

	// compatible with old version's upgrade snapshot name
	if oldSnapshot != nil && (vm.IsConfigureSnapshot(oldSnapshot, ConfigurePrefix) || vm.IsConfigureSnapshot(oldSnapshot, UpgradePrefix)) {
		d.retryDeleteSnapshotByRef(&oldSnapshot.Snapshot, oldSnapshot.Name, conf.Name)
	}
	return nil
}

func (d *Dispatcher) rollbackResourceSettings(poolName string, settings *data.InstallerData) error {
	if !settings.VCHSizeIsSet || d.oldVCHResources == nil {
		d.op.Debug("VCH resource settings are not changed")
		return nil
	}
	return updateResourcePoolConfig(d.op, d.vchPool, poolName, d.oldVCHResources)
}

func (d *Dispatcher) updateResourceSettings(poolName string, settings *data.InstallerData) error {
	if !settings.VCHSizeIsSet {
		d.op.Debug("VCH resource settings are not changed")
		return nil
	}
	var err error
	// compute resource
	d.vchPool, err = d.appliance.ResourcePool(d.op)
	if err != nil {
		err = errors.Errorf("Failed to get parent resource pool %q: %s", poolName, err)
		return err
	}
	oldSettings, err := d.getPoolResourceSettings(d.vchPool)
	if err != nil {
		err = errors.Errorf("Failed to get parent resource settings %q: %s", poolName, err)
		return err
	}
	if reflect.DeepEqual(oldSettings, &settings.VCHSize) {
		d.op.Debug("VCH resource settings are same as old value")
		return nil
	}
	d.oldVCHResources = oldSettings
	return updateResourcePoolConfig(d.op, d.vchPool, poolName, &settings.VCHSize)
}

func (d *Dispatcher) Rollback(vch *vm.VirtualMachine, conf *config.VirtualContainerHostConfigSpec, settings *data.InstallerData) error {

	// some setup that is only necessary because we didn't just create a VCH in this case
	d.appliance = vch
	d.setDockerPort(conf, settings)

	// ensure that we wait for components to come up
	// TODO this stanza appears in Update too so we need to abstract it into a helper function
	for _, s := range conf.ExecutorConfig.Sessions {
		s.Started = ""
	}

	notfound := "A VCH version available from before the last upgrade could not be found."
	snapshot, err := d.appliance.GetCurrentSnapshotTree(d.op)
	if err != nil {
		return errors.Errorf("%s An error was reported while trying to discover it: %s", notfound, err)
	}

	if snapshot == nil {
		return errors.Errorf("%s No error was reported, so it's possible that this VCH has never been upgraded or the saved previous version was removed out-of-band.", notfound)
	}

	err = d.rollback(conf, snapshot.Name, settings)
	if err != nil {
		return errors.Errorf("could not complete manual rollback: %s", err)
	}

	return d.retryDeleteSnapshotByRef(&snapshot.Snapshot, snapshot.Name, conf.Name)
}

// retryDeleteSnapshotByRef will retry to delete snapshot by its reference if there is GenericVmConfigFault returned. This is a workaround for vSAN delete snapshot
func (d *Dispatcher) retryDeleteSnapshotByRef(snapshot *types.ManagedObjectReference, snapshotName, applianceName string) error {
	// delete snapshot immediately after snapshot rollback usually fail in vSAN, so have to retry several times
	operation := func() error {
		return d.deleteSnapshotByRef(snapshot, snapshotName, applianceName)
	}
	var err error
	if err = retry.Do(operation, isSystemError); err != nil {
		d.op.Errorf("Failed to clean up appliance upgrade snapshot %q: %s.", snapshotName, err)
		d.op.Errorf("Snapshot %q of appliance virtual machine %q MUST be removed manually before upgrade again", snapshotName, applianceName)
	}
	return err
}

func isSystemError(err error) bool {
	if soap.IsSoapFault(err) {
		if _, ok := soap.ToSoapFault(err).VimFault().(*types.SystemError); ok {
			return true
		}
	}

	if soap.IsVimFault(err) {
		if _, ok := soap.ToVimFault(err).(*types.SystemError); ok {
			return true
		}
	}

	if terr, ok := err.(task.Error); ok {
		if _, ok := terr.Fault().(*types.SystemError); ok {
			return true
		}
	}

	return false
}

func (d *Dispatcher) deleteSnapshotByRef(snapshot *types.ManagedObjectReference, snapshotName, applianceName string) error {
	defer trace.End(trace.Begin(snapshotName, d.op))
	d.op.Infof("Deleting upgrade snapshot %q", snapshotName)
	// do clean up aggressively, even the previous operation failed with context deadline exceeded.
	op := trace.NewOperationWithLoggerFrom(context.Background(), d.op, "deleteSnapshotByRef cleanup")
	if _, err := d.appliance.WaitForResult(op, func(ctx context.Context) (tasks.Task, error) {
		consolidate := true
		return d.appliance.RemoveSnapshotByRef(ctx, snapshot, false, &consolidate)
	}); err != nil {
		return err
	}
	return nil
}

// tryCreateSnapshot try to create upgrade snapshot. It will check if upgrade snapshot already exists. If exists, return error.
// if succeed, return snapshot refID
func (d *Dispatcher) tryCreateSnapshot(name, desc string) (*types.ManagedObjectReference, error) {
	defer trace.End(trace.Begin(name, d.op))

	// TODO detect whether another upgrade is in progress & bail if it is.
	// Use solution from https://github.com/vmware/vic/issues/4069 to do this either as part of 4069 or once it's closed

	info, err := d.appliance.WaitForResult(d.op, func(ctx context.Context) (tasks.Task, error) {
		return d.appliance.CreateSnapshot(ctx, name, desc, true, false)
	})
	if err != nil {
		return nil, errors.Errorf("Failed to create upgrade snapshot %q: %s.", name, err)
	}
	return info.Entity, nil
}

func (d *Dispatcher) deleteUpgradeImages(ds *object.Datastore, settings *data.InstallerData) {
	defer trace.End(trace.Begin("", d.op))

	d.op.Info("Deleting upgrade images")

	// do clean up aggressively, even the previous operation failed with context deadline exceeded.
	d.op = trace.NewOperation(context.Background(), "deleteUpgradeImages cleanup")

	m := ds.NewFileManager(d.session.Datacenter, true)

	file := ds.Path(path.Join(d.vmPathName, settings.ApplianceISO))
	if err := d.deleteVMFSFiles(m, ds, file); err != nil {
		d.op.Warnf("Image file %q is not removed for %s. Use the vSphere UI to delete content", file, err)
	}

	file = ds.Path(path.Join(d.vmPathName, settings.BootstrapISO))
	if err := d.deleteVMFSFiles(m, ds, file); err != nil {
		d.op.Warnf("Image file %q is not removed for %s. Use the vSphere UI to delete content", file, err)
	}
}

func (d *Dispatcher) update(conf *config.VirtualContainerHostConfigSpec, settings *data.InstallerData, isConfigureOp bool) error {
	defer trace.End(trace.Begin(conf.Name, d.op))

	power, err := d.appliance.PowerState(d.op)
	if err != nil {
		d.op.Errorf("Failed to get vm power status %q: %s", d.appliance.Reference(), err)
		return err
	}
	if power != types.VirtualMachinePowerStatePoweredOff {
		if _, err = d.appliance.WaitForResult(d.op, func(ctx context.Context) (tasks.Task, error) {
			return d.appliance.PowerOff(ctx)
		}); err != nil {
			d.op.Errorf("Failed to power off appliance: %s", err)
			return err
		}
	}

	isoFile := ""
	if settings.ApplianceISO != "" {
		isoFile = fmt.Sprintf("[%s] %s/%s", conf.ImageStores[0].Host, d.vmPathName, settings.ApplianceISO)
	}

	// Create volume stores only for a configure operation, where conf has its storage fields validated.
	if isConfigureOp {
		if err := d.createVolumeStores(conf); err != nil {
			return err
		}
	}

	if err = d.reconfigVCH(conf, isoFile); err != nil {
		return err
	}

	if err = d.startAppliance(conf); err != nil {
		return err
	}

	op, cancel := trace.WithTimeout(&d.op, settings.Timeout, "CheckServiceReady during update")
	defer cancel()
	if err = d.CheckServiceReady(op, conf, nil); err != nil {
		if op.Err() == context.DeadlineExceeded {
			//context deadline exceeded, replace returned error message
			err = errors.Errorf("Upgrading VCH exceeded time limit of %s. Please increase the timeout using --timeout to accommodate for a busy vSphere target", settings.Timeout)
		}

		d.op.Info("\tAPI may be slow to start - please retry with increased timeout using --timeout")
		return err
	}
	return nil
}

func (d *Dispatcher) rollback(conf *config.VirtualContainerHostConfigSpec, snapshot string, settings *data.InstallerData) error {
	defer trace.End(trace.Begin(fmt.Sprintf("old appliance iso: %q, snapshot: %q", d.oldApplianceISO, snapshot), d.op))

	// do not power on appliance in this snapshot revert
	d.op.Infof("Reverting to snapshot %s", snapshot)
	if _, err := d.appliance.WaitForResult(d.op, func(ctx context.Context) (tasks.Task, error) {
		return d.appliance.RevertToSnapshot(d.op, snapshot, true)
	}); err != nil {
		return errors.Errorf("Failed to roll back upgrade: %s.", err)
	}
	return d.ensureRollbackReady(conf, settings)
}

func (d *Dispatcher) ensureRollbackReady(conf *config.VirtualContainerHostConfigSpec, settings *data.InstallerData) error {
	defer trace.End(trace.Begin(conf.Name, d.op))

	power, err := d.appliance.PowerState(d.op)
	if err != nil {
		d.op.Errorf("Failed to get vm power status %q after rollback: %s", d.appliance.Reference(), err)
		return err
	}
	if power == types.VirtualMachinePowerStatePoweredOff {
		d.op.Info("Roll back finished - Appliance is kept in powered off status")
		return nil
	}
	if err = d.startAppliance(conf); err != nil {
		return err
	}

	op, cancel := trace.WithTimeout(&d.op, settings.Timeout, "CheckServiceReady during rollback")
	defer cancel()
	if err = d.CheckServiceReady(op, conf, nil); err != nil {
		// do not return error in this case, to make sure clean up continues
		d.op.Info("\tAPI may be slow to start - try to connect to API after a few minutes")
	}
	return nil
}

func (d *Dispatcher) reconfigVCH(conf *config.VirtualContainerHostConfigSpec, isoFile string) error {
	defer trace.End(trace.Begin(isoFile, d.op))

	spec := &types.VirtualMachineConfigSpec{}

	if isoFile != "" {
		deviceChange, err := d.switchISO(isoFile)
		if err != nil {
			return err
		}

		spec.DeviceChange = deviceChange
	}

	if conf != nil {
		// reset service started attribute
		for _, sess := range conf.ExecutorConfig.Sessions {
			sess.Started = ""
			sess.Active = true
		}
		if err := d.addExtraConfig(spec, conf); err != nil {
			return err
		}
	}

	if spec.DeviceChange == nil && spec.ExtraConfig == nil {
		// nothing need to do
		d.op.Debug("Nothing changed, no need to reconfigure appliance")
		return nil
	}

	// reconfig
	d.op.Info("Setting VM configuration")
	info, err := d.appliance.WaitForResult(d.op, func(ctx context.Context) (tasks.Task, error) {
		return d.appliance.Reconfigure(ctx, *spec)
	})

	if err != nil {
		d.op.Errorf("Error while reconfiguring appliance: %s", err)
		return err
	}
	if info.State != types.TaskInfoStateSuccess {
		d.op.Errorf("Reconfiguring appliance reported: %s", info.Error.LocalizedMessage)
		return err
	}
	return nil
}

func (d *Dispatcher) addExtraConfig(spec *types.VirtualMachineConfigSpec, conf *config.VirtualContainerHostConfigSpec) error {
	if conf == nil {
		return nil
	}
	cfg, err := d.encodeConfig(conf)
	if err != nil {
		return err
	}
	spec.ExtraConfig = append(spec.ExtraConfig, vmomi.OptionValueFromMap(cfg, true)...)

	// get back old configuration, to remove keys not existed in new guestinfo. We don't care about value atm
	oldConfig, err := d.GetNoSecretVCHConfig(d.appliance)
	if err != nil {
		return err
	}
	old := make(map[string]string)
	extraconfig.Encode(extraconfig.MapSink(old), oldConfig)
	for k := range old {
		if _, ok := cfg[k]; !ok {
			// set old key value to empty string, will remove that key from guestinfo
			spec.ExtraConfig = append(spec.ExtraConfig, &types.OptionValue{Key: k, Value: ""})
		}
	}

	return nil
}

func (d *Dispatcher) switchISO(filePath string) ([]types.BaseVirtualDeviceConfigSpec, error) {
	defer trace.End(trace.Begin(filePath, d.op))

	var devices object.VirtualDeviceList
	var err error

	d.op.Infof("Switching appliance iso to %s", filePath)
	devices, err = d.appliance.Device(d.op)
	if err != nil {
		d.op.Errorf("Failed to get vm devices for appliance: %s", err)
		return nil, err
	}
	// find the single cdrom
	cd, err := devices.FindCdrom("")
	if err != nil {
		d.op.Errorf("Failed to get CD rom device from appliance: %s", err)
		return nil, err
	}

	oldApplianceISO := cd.Backing.(*types.VirtualCdromIsoBackingInfo).FileName
	if oldApplianceISO == filePath {
		d.op.Debug("Target file name %q is same to old one, no need to change.")
		return nil, nil
	}
	cd = devices.InsertIso(cd, filePath)
	changedDevices := object.VirtualDeviceList([]types.BaseVirtualDevice{cd})

	deviceChange, err := changedDevices.ConfigSpec(types.VirtualDeviceConfigSpecOperationEdit)
	if err != nil {
		d.op.Errorf("Failed to create config spec for appliance: %s", err)
		return nil, err
	}

	d.oldApplianceISO = oldApplianceISO
	return deviceChange, nil
}

// extractSecretFromFile reads and extracts the GuestInfoSecretKey value from the input.
func extractSecretFromFile(rc io.ReadCloser) (string, error) {

	scanner := bufio.NewScanner(rc)
	for scanner.Scan() {
		line := scanner.Text()

		// The line is of the format: key = "value"
		if strings.HasPrefix(line, extraconfig.GuestInfoSecretKey) {

			tokens := strings.SplitN(line, "=", 2)
			if len(tokens) < 2 {
				return "", fmt.Errorf("parse error: unexpected token count in line")
			}

			// Ensure that the key fully matches the secret key
			if strings.Trim(tokens[0], ` `) != extraconfig.GuestInfoSecretKey {
				continue
			}

			// Trim double quotes and spaces
			return strings.Trim(tokens[1], `" `), nil
		}
	}

	return "", errSecretKeyNotFound
}

// GuestInfoSecret downloads the VCH's .vmx file and returns the GuestInfoSecretKey value.
func (d *Dispatcher) GuestInfoSecret(vchName, vmPath string, ds *object.Datastore) (*extraconfig.SecretKey, error) {
	defer trace.End(trace.Begin("", d.op))

	if ds == nil {
		return nil, errNilDatastore
	}

	helper, err := datastore.NewHelper(d.op, d.session, ds, vmPath)
	if err != nil {
		return nil, err
	}

	// Download the VCH's .vmx file
	path := fmt.Sprintf("%s.vmx", vchName)
	rc, err := helper.Download(d.op, path)
	if err != nil {
		return nil, err
	}

	secret, err := extractSecretFromFile(rc)
	if err != nil {
		return nil, err
	}

	secretKey := &extraconfig.SecretKey{}
	if err = secretKey.FromString(secret); err != nil {
		return nil, err
	}

	return secretKey, nil
}
