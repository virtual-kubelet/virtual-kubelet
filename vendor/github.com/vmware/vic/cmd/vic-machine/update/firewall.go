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

package update

import (
	"context"
	"time"

	"gopkg.in/urfave/cli.v1"

	"github.com/vmware/vic/cmd/vic-machine/common"
	"github.com/vmware/vic/lib/install/data"
	"github.com/vmware/vic/lib/install/management"
	"github.com/vmware/vic/lib/install/validate"
	"github.com/vmware/vic/pkg/errors"
	"github.com/vmware/vic/pkg/trace"
)

// UpdateFw has all input parameters for vic-machine update firewall command
type UpdateFw struct {
	*data.Data

	executor *management.Dispatcher

	enableFw  bool
	disableFw bool
}

func NewUpdateFw() *UpdateFw {
	i := &UpdateFw{}
	i.Data = data.NewData()
	return i
}

// Flags return all cli flags for update firewall
func (i *UpdateFw) Flags() []cli.Flag {
	update := []cli.Flag{
		cli.DurationFlag{
			Name:        "timeout",
			Value:       3 * time.Minute,
			Usage:       "Time to wait for update firewall",
			Destination: &i.Timeout,
		},
		cli.BoolFlag{
			Name:        "allow",
			Usage:       "Enable a firewall rule on target host(s) to allow VIC communication",
			Destination: &i.enableFw,
		},
		cli.BoolFlag{
			Name:        "deny",
			Usage:       "Disable the firewall rule on target host(s) that allows VIC communication",
			Destination: &i.disableFw,
		},
	}

	target := i.TargetFlags()
	compute := i.ComputeFlagsNoName()
	debug := i.DebugFlags(true)

	// flag arrays are declared, now combined
	var flags []cli.Flag
	for _, f := range [][]cli.Flag{target, compute, update, debug} {
		flags = append(flags, f...)
	}

	return flags
}

func (i *UpdateFw) processParams(op trace.Operation) error {
	defer trace.End(trace.Begin("", op))

	if err := i.HasCredentials(op); err != nil {
		return err
	}

	if i.enableFw && i.disableFw {
		return errors.New("Only one of --allow and --deny can be set")
	}

	if !i.enableFw && !i.disableFw {
		return errors.New("No command selected")
	}

	return nil
}

func (i *UpdateFw) Run(clic *cli.Context) (err error) {
	op := common.NewOperation(clic, i.Debug.Debug)
	defer func() {
		// urfave/cli will print out exit in error handling, so no more information in main method can be printed out.
		err = common.LogErrorIfAny(op, clic, err)
	}()
	op, cancel := trace.WithTimeout(&op, i.Timeout, clic.App.Name)
	defer cancel()
	defer func() {
		if op.Err() != nil && op.Err() == context.DeadlineExceeded {
			//context deadline exceeded, replace returned error message
			err = errors.Errorf("Update timed out: use --timeout to add more time")
		}
	}()

	if err = i.processParams(op); err != nil {
		return err
	}

	if len(clic.Args()) > 0 {
		op.Errorf("Unknown argument: %s", clic.Args()[0])
		return errors.New("invalid CLI arguments")
	}

	op.Infof("### Updating Firewall ####")

	var validator *validate.Validator
	if validator, err = validate.NewValidator(op, i.Data); err != nil {
		op.Errorf("Update cannot continue - failed to create validator: %s", err)
		return errors.New("update firewall failed")
	}
	defer validator.Session().Logout(op)

	_, err = validator.ValidateTarget(op, i.Data, false)
	if err != nil {
		op.Errorf("Update cannot continue - target validation failed: %s", err)
		return errors.New("update firewall failed")
	}
	_, err = validator.ValidateCompute(op, i.Data, true)
	if err != nil {
		op.Errorf("Update cannot continue - compute resource validation failed: %s", err)
		return errors.New("update firewall failed")
	}

	executor := management.NewDispatcher(op, validator.Session(), management.ActionUpdate, false)

	if i.enableFw {
		op.Info("")
		op.Warn("### WARNING ###")
		op.Warn("\tThis command modifies the host firewall on the target machine or cluster")
		op.Warnf("\tThe ruleset %q will be enabled", management.RulesetID)
		op.Warn("\tThis allows all outbound TCP traffic from the target")
		op.Warn("\tTo undo this modification use --deny")
		op.Info("")

		err := executor.EnableFirewallRuleset()
		if err != nil {
			op.Errorf("Failed to enable VIC firewall rule: %s", err)
			return errors.New("failed to enable firewall rule")
		}
	}

	if i.disableFw {
		op.Info("")
		op.Warn("### WARNING ###")
		op.Warn("\tThis command modifies the host firewall on the target machine or cluster")
		op.Warnf("\tThe ruleset %q will be disabled", management.RulesetID)
		op.Warn("\tThis disables the ruleset that allows all outbound TCP traffic from the target")
		op.Warn("\tVIC Engine will not function unless 2377/tcp outbound is allowed")
		op.Warn("\tTo undo this modification use --allow")
		op.Info("")

		err := executor.DisableFirewallRuleset()
		if err != nil {
			op.Errorf("Failed to disable VIC firewall rule: %s", err)
			return errors.New("failed to disable firewall rule")
		}
	}
	op.Info("")

	if i.enableFw || i.disableFw {
		op.Infof("Firewall changes complete")
	}

	op.Infof("Command completed successfully")
	return nil
}
