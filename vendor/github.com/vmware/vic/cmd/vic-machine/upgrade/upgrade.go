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

package upgrade

import (
	"path"
	"time"

	"gopkg.in/urfave/cli.v1"

	"github.com/vmware/vic/cmd/vic-machine/common"
	"github.com/vmware/vic/lib/install/data"
	"github.com/vmware/vic/lib/install/management"
	"github.com/vmware/vic/lib/install/validate"
	"github.com/vmware/vic/pkg/errors"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/vsphere/vm"
)

// Upgrade has all input parameters for vic-machine upgrade command
type Upgrade struct {
	*data.Data

	executor *management.Dispatcher
}

func NewUpgrade() *Upgrade {
	upgrade := &Upgrade{}
	upgrade.Data = data.NewData()

	return upgrade
}

// Flags return all cli flags for upgrade
func (u *Upgrade) Flags() []cli.Flag {
	util := []cli.Flag{
		cli.BoolFlag{
			Name:        "force, f",
			Usage:       "Force the upgrade (ignores version checks)",
			Destination: &u.Force,
		},
		cli.DurationFlag{
			Name:        "timeout",
			Value:       3 * time.Minute,
			Usage:       "Time to wait for upgrade",
			Destination: &u.Timeout,
		},
		cli.BoolFlag{
			Name:        "rollback",
			Usage:       "Roll back VCH version to before the current upgrade",
			Destination: &u.Rollback,
		},
		cli.BoolFlag{
			Name:        "reset-progress",
			Usage:       "Reset the UpdateInProgress flag. Warning: Do not reset this flag if another upgrade/configure process is running",
			Destination: &u.ResetInProgressFlag,
		},
	}

	target := u.TargetFlags()
	id := u.IDFlags()
	compute := u.ComputeFlags()
	iso := u.ImageFlags(false)
	debug := u.DebugFlags(true)

	// flag arrays are declared, now combined
	var flags []cli.Flag
	for _, f := range [][]cli.Flag{target, id, compute, iso, util, debug} {
		flags = append(flags, f...)
	}

	return flags
}

func (u *Upgrade) processParams(op trace.Operation) error {
	defer trace.End(trace.Begin("", op))

	if err := u.HasCredentials(op); err != nil {
		return err
	}

	return nil
}

func (u *Upgrade) Run(clic *cli.Context) (err error) {
	op := common.NewOperation(clic, u.Debug.Debug)
	defer func() {
		// urfave/cli will print out exit in error handling, so no more information in main method can be printed out.
		err = common.LogErrorIfAny(op, clic, err)
	}()
	op, cancel := trace.WithCancel(&op, clic.App.Name)
	defer cancel()

	if err = u.processParams(op); err != nil {
		return err
	}

	if len(clic.Args()) > 0 {
		op.Errorf("Unknown argument: %s", clic.Args()[0])
		return errors.New("invalid CLI arguments")
	}

	var images map[string]string
	if images, err = u.CheckImagesFiles(op, u.Force); err != nil {
		return err
	}

	op.Infof("### Upgrading VCH ####")

	validator, err := validate.NewValidator(op, u.Data)
	if err != nil {
		op.Errorf("Upgrade cannot continue - failed to create validator: %s", err)
		return errors.New("upgrade failed")
	}
	defer validator.Session.Logout(op)

	_, err = validator.ValidateTarget(op, u.Data)
	if err != nil {
		op.Errorf("Upgrade cannot continue - target validation failed: %s", err)
		return errors.New("upgrade failed")
	}
	executor := management.NewDispatcher(validator.Context, validator.Session, nil, u.Force)

	var vch *vm.VirtualMachine
	if u.Data.ID != "" {
		vch, err = executor.NewVCHFromID(u.Data.ID)
	} else {
		vch, err = executor.NewVCHFromComputePath(u.Data.ComputeResourcePath, u.Data.DisplayName, validator)
	}
	if err != nil {
		op.Errorf("Failed to get Virtual Container Host %s", u.DisplayName)
		op.Error(err)
		return errors.New("upgrade failed")
	}

	op.Infof("")
	op.Infof("VCH ID: %s", vch.Reference().String())

	if u.ResetInProgressFlag {
		if err = vch.SetVCHUpdateStatus(op, false); err != nil {
			op.Error("Failed to reset UpdateInProgress flag")
			op.Error(err)
			return errors.New("upgrade failed")
		}
		op.Infof("Reset UpdateInProgress flag successfully")
		return nil
	}

	upgrading, err := vch.VCHUpdateStatus(op)
	if err != nil {
		op.Error("Unable to determine if upgrade/configure is in progress")
		op.Error(err)
		return errors.New("upgrade failed")
	}
	if upgrading {
		op.Error("Upgrade failed: another upgrade/configure operation is in progress")
		op.Error("If no other upgrade/configure process is running, use --reset-progress to reset the VCH upgrade/configure status")
		return errors.New("upgrade failed")
	}

	if err = vch.SetVCHUpdateStatus(op, true); err != nil {
		op.Error("Failed to set UpdateInProgress flag to true")
		op.Error(err)
		return errors.New("upgrade failed")
	}

	defer func() {
		if err = vch.SetVCHUpdateStatus(op, false); err != nil {
			op.Error("Failed to reset UpdateInProgress")
			op.Error(err)
		}
	}()

	vchConfig, err := executor.FetchAndMigrateVCHConfig(vch)
	if err != nil {
		op.Error("Failed to get Virtual Container Host configuration")
		op.Error(err)
		return errors.New("upgrade failed")
	}

	vConfig := validator.AddDeprecatedFields(op, vchConfig, u.Data)
	vConfig.ImageFiles = images
	vConfig.ApplianceISO = path.Base(u.ApplianceISO)
	vConfig.BootstrapISO = path.Base(u.BootstrapISO)
	vConfig.Timeout = u.Timeout

	// only care about versions if we're not doing a manual rollback
	if !u.Data.Rollback {
		if err := validator.AssertVersion(op, vchConfig); err != nil {
			op.Error(err)
			return errors.New("upgrade failed")
		}
	}

	if vchConfig, err = validator.ValidateMigratedConfig(op, vchConfig); err != nil {
		op.Errorf("Failed to migrate Virtual Container Host configuration %s", u.DisplayName)
		op.Error(err)
		return errors.New("upgrade failed")
	}

	if !u.Data.Rollback {
		err = executor.Configure(vch, vchConfig, vConfig, false)
	} else {
		err = executor.Rollback(vch, vchConfig, vConfig)
	}

	if err != nil {
		// upgrade failed
		executor.CollectDiagnosticLogs()
		return errors.New("upgrade failed")
	}

	op.Infof("Completed successfully")

	return nil
}
