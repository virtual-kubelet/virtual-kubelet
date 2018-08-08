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
	"path/filepath"
	"time"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/vic/lib/config"
	"github.com/vmware/vic/lib/install/data"
	"github.com/vmware/vic/lib/install/opsuser"
	"github.com/vmware/vic/lib/install/vchlog"
	"github.com/vmware/vic/pkg/errors"
	"github.com/vmware/vic/pkg/retry"
	"github.com/vmware/vic/pkg/trace"
)

const (
	uploadRetryLimit      = 5
	uploadMaxElapsedTime  = 30 * time.Minute
	uploadMaxInterval     = 1 * time.Minute
	uploadInitialInterval = 10 * time.Second
)

func (d *Dispatcher) CreateVCH(conf *config.VirtualContainerHostConfigSpec, settings *data.InstallerData, receiver vchlog.Receiver) error {
	defer trace.End(trace.Begin(conf.Name, d.op))

	var err error

	// Resource Pools are only available in DRS Enabled environments, so
	// the resource pool path will be determined on that setting.
	//
	// In a DRS disabled environment resource pools aren't available and all
	// VMs will reside in the cluster pool.  Attempting to create a pool would
	// result in an error and vic-machine failure.
	//
	// DRS Enabled:
	// append the appliance name to the path with the goal of having the
	// pool name match the appliance name.
	// DRS Disabled:
	// only use the compute path which will avoid a pool creation attempt.
	if d.session.DRSEnabled != nil && !*d.session.DRSEnabled {
		d.vchPoolPath = settings.ResourcePoolPath
	} else {
		d.vchPoolPath = path.Join(settings.ResourcePoolPath, conf.Name)
	}

	if err = d.checkExistence(conf, settings); err != nil {
		return err
	}

	if err = d.createPool(conf, settings); err != nil {
		return err
	}

	if err = d.createBridgeNetwork(conf); err != nil {
		d.cleanupAfterCreationFailed(conf, false)
		return err
	}

	if err = d.createAppliance(conf, settings); err != nil {
		d.cleanupAfterCreationFailed(conf, true)
		return errors.Errorf("Creating the appliance failed with %s. Exiting...", err)
	}

	// send the signal to VCH logger to indicate VCH datastore path is ready
	datastoreReadySignal := vchlog.DatastoreReadySignal{
		Datastore:  d.session.Datastore,
		Name:       "create",
		Operation:  d.op,
		VMPathName: d.vmPathName,
		Timestamp:  time.Now(),
	}
	receiver.Signal(datastoreReadySignal)

	if err = d.uploadISOs(settings.ImageFiles); err != nil {
		return errors.Errorf("Uploading vic isos failed with %s. Exiting...", err)
	}

	if conf.ShouldGrantPerms() {
		err = opsuser.GrantOpsUserPerms(d.op, d.session, conf)
		if err != nil {
			return errors.Errorf("Cannot init ops-user permissions, failure: %s. Exiting...", err)
		}
	}

	if err = d.createVMGroup(conf); err != nil {
		return err
	}

	return d.appliance.PowerOn(d.op)
}

func (d *Dispatcher) createPool(conf *config.VirtualContainerHostConfigSpec, settings *data.InstallerData) error {
	defer trace.End(trace.Begin("", d.op))

	var err error

	if d.vchPool, err = d.createResourcePool(conf, settings); err != nil {
		detail := fmt.Sprintf("Creating resource pool failed: %s", err)
		return errors.New(detail)
	}

	return nil
}

func (d *Dispatcher) uploadISOs(files map[string]string) error {
	defer trace.End(trace.Begin("", d.op))

	// upload the images
	d.op.Info("Uploading ISO images")

	// Build retry config
	backoffConf := retry.NewBackoffConfig()
	backoffConf.InitialInterval = uploadInitialInterval
	backoffConf.MaxInterval = uploadMaxInterval
	backoffConf.MaxElapsedTime = uploadMaxElapsedTime

	for key, image := range files {
		baseName := filepath.Base(image)
		// upload function that is passed to retry
		isoTargetPath := path.Join(d.vmPathName, key)

		operationForRetry := func() error {
			op, cancel := trace.WithCancel(&d.op, "uploadISOs")
			defer cancel()

			// attempt to delete the iso image first in case of failed upload
			dc := d.session.Datacenter
			fm := d.session.Datastore.NewFileManager(dc, false)
			ds := d.session.Datastore

			// check iso first
			op.Debugf("Checking if file already exists: %s", isoTargetPath)
			_, err := ds.Stat(op, isoTargetPath)
			if err != nil {
				switch err.(type) {
				case object.DatastoreNoSuchFileError:
					op.Debug("File not found. Nothing to do.")
				case object.DatastoreNoSuchDirectoryError:
					op.Debug("Directory not found. Nothing to do.")
				default:
					op.Debugf("ISO file already exists, deleting: %s", isoTargetPath)
					err := fm.Delete(d.op, isoTargetPath)
					if err != nil {
						op.Debugf("Failed to delete image (%s) with error (%s)", image, err.Error())
						return err
					}
				}
			}

			op.Infof("Uploading %s as %s", baseName, key)

			return d.session.Datastore.UploadFile(op, image, path.Join(d.vmPathName, key),
				nil)
		}

		// counter for retry decider
		retryCount := uploadRetryLimit

		// decider for our retry, will retry the upload uploadRetryLimit times
		uploadRetryDecider := func(err error) bool {
			if err == nil {
				return false
			}

			retryCount--
			if retryCount < 0 {
				d.op.Warnf("Attempted upload a total of %d times without success, Upload process failed.", uploadRetryLimit)
				return false
			}
			d.op.Warnf("Failed an attempt to upload isos with err (%s), %d retries remain", err.Error(), retryCount)
			return true
		}

		uploadErr := retry.DoWithConfig(operationForRetry, uploadRetryDecider, backoffConf)
		if uploadErr != nil {
			finalMessage := fmt.Sprintf("\t\tUpload failed for %q: %s\n", image, uploadErr)
			if d.force {
				finalMessage = fmt.Sprintf("%s\t\tContinuing despite failures (due to --force option)\n", finalMessage)
				finalMessage = fmt.Sprintf("%s\t\tNote: The VCH will not function without %q...", finalMessage, image)
			}
			d.op.Error(finalMessage)
			return errors.New("Failed to upload iso images.")
		}

	}
	return nil

}

// cleanupAfterCreationFailed cleans up the dangling resource pool for the failed VCH and any bridge network if there is any.
// The function will not abort and early terminate upon any error during cleanup process. Error details are logged.
func (d *Dispatcher) cleanupAfterCreationFailed(conf *config.VirtualContainerHostConfigSpec, cleanupNetwork bool) {
	defer trace.End(trace.Begin(conf.Name, d.op))
	var err error

	d.op.Debug("Cleaning up dangling VCH resources after VCH creation failure.")

	err = d.cleanupEmptyPool(conf)
	if err != nil {
		d.op.Errorf("Failed to clean up dangling VCH resource pool after VCH creation failure: %s", err)
	} else {
		d.op.Debug("Successfully cleaned up dangling resource pool.")
	}

	// only clean up bridge network created if told to
	if cleanupNetwork {
		err = d.cleanupBridgeNetwork(conf)
		if err != nil {
			d.op.Errorf("Failed to clean up dangling bridge network after VCH creation failure: %s", err)
		} else {
			d.op.Debug("Successfully cleaned up dangling bridge network.")
		}
	}

	// Delete the VCH Folder
	d.deleteFolder(d.session.VCHFolder)
}

// cleanupEmptyPool cleans up any dangling empty VCH resource pool when creating this VCH. no-op when VCH pool is nonempty.
func (d *Dispatcher) cleanupEmptyPool(conf *config.VirtualContainerHostConfigSpec) error {
	defer trace.End(trace.Begin(conf.Name, d.op))
	var err error

	d.parentResourcepool, err = d.getComputeResource(nil, conf)
	if err != nil {
		return err
	}

	defaultrp, err := d.session.Cluster.ResourcePool(d.op)
	if err != nil {
		return err
	}

	if d.parentResourcepool != nil && d.parentResourcepool.Reference() == defaultrp.Reference() {
		d.op.Info("VCH resource pool is cluster default pool - skipping cleanup")
		return nil
	}

	err = d.destroyResourcePoolIfEmpty(conf)
	if err != nil {
		return err
	}

	return nil
}

// cleanupBridgeNetwork cleans up any bridge networks created when creating this VCH. no-op for VCenter environment.
func (d *Dispatcher) cleanupBridgeNetwork(conf *config.VirtualContainerHostConfigSpec) error {
	defer trace.End(trace.Begin(conf.Name, d.op))

	err := d.removeNetwork(conf)
	if err != nil {
		return err
	}

	return nil
}
