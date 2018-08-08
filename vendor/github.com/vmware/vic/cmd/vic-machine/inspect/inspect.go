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

package inspect

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"gopkg.in/urfave/cli.v1"

	"github.com/vmware/vic/cmd/vic-machine/common"
	"github.com/vmware/vic/cmd/vic-machine/converter"
	"github.com/vmware/vic/cmd/vic-machine/create"
	"github.com/vmware/vic/lib/config"
	"github.com/vmware/vic/lib/install/data"
	"github.com/vmware/vic/lib/install/interaction"
	"github.com/vmware/vic/lib/install/management"
	"github.com/vmware/vic/lib/install/validate"
	"github.com/vmware/vic/pkg/errors"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/version"
	"github.com/vmware/vic/pkg/vsphere/vm"
)

// Inspect has all input parameters for vic-machine inspect command
type Inspect struct {
	*data.Data

	CertPath string

	executor *management.Dispatcher

	Format string
}

type state struct {
	i         *Inspect
	op        trace.Operation
	validator *validate.Validator
	vchConfig *config.VirtualContainerHostConfigSpec
	vch       *vm.VirtualMachine
	executor  *management.Dispatcher
}

type command func(state) error

func NewInspect() *Inspect {
	d := &Inspect{}
	d.Data = data.NewData()
	return d
}

// Flags returns all cli flags for inspect
func (i *Inspect) Flags() []cli.Flag {
	util := []cli.Flag{
		cli.DurationFlag{
			Name:        "timeout",
			Value:       3 * time.Minute,
			Usage:       "Time to wait for inspect",
			Destination: &i.Timeout,
		},
		cli.StringFlag{
			Name:        "tls-cert-path",
			Value:       "",
			Usage:       "The path to check for existing certificates. Defaults to './<vch name>/'",
			Destination: &i.CertPath,
		},
	}

	target := i.TargetFlags()
	id := i.IDFlags()
	compute := i.ComputeFlags()
	debug := i.DebugFlags(true)

	// flag arrays are declared, now combined
	var flags []cli.Flag
	for _, f := range [][]cli.Flag{target, id, compute, util, debug} {
		flags = append(flags, f...)
	}

	return flags
}

func (i *Inspect) ConfigFlags() []cli.Flag {
	config := cli.StringFlag{
		Name:        "format",
		Value:       "verbose",
		Usage:       "Determine the format of configuration output. Supported formats: raw, verbose",
		Destination: &i.Format,
	}
	flags := []cli.Flag{config}
	flags = append(flags, i.Flags()...)
	return flags
}

func (i *Inspect) processParams(op trace.Operation) error {
	defer trace.End(trace.Begin("", op))

	if err := i.HasCredentials(op); err != nil {
		return err
	}

	return nil
}

func (i *Inspect) run(clic *cli.Context, op trace.Operation, cmd command) (err error) {
	defer func() {
		// urfave/cli will print out exit in error handling, so no more information in main method can be printed out.
		err = common.LogErrorIfAny(op, clic, err)
	}()
	op, cancel := trace.WithTimeout(&op, i.Timeout, clic.App.Name)
	defer cancel()
	defer func() {
		if op.Err() != nil && op.Err() == context.DeadlineExceeded {
			//context deadline exceeded, replace returned error message
			err = errors.Errorf("Inspect timed out: use --timeout to add more time")
		}
	}()

	if err = i.processParams(op); err != nil {
		return err
	}

	if len(clic.Args()) > 0 {
		op.Errorf("Unknown argument: %s", clic.Args()[0])
		return errors.New("invalid CLI arguments")
	}

	op.Infof("### Inspecting VCH ####")

	validator, err := validate.NewValidator(op, i.Data)

	if err != nil {
		op.Errorf("Inspect cannot continue - failed to create validator: %s", err)
		return errors.New("inspect failed")
	}
	defer validator.Session().Logout(op)

	_, err = validator.ValidateTarget(op, i.Data, false)
	if err != nil {
		op.Errorf("Inspect cannot continue - target validation failed: %s", err)
		return errors.New("inspect failed")
	}

	executor := management.NewDispatcher(op, validator.Session(), management.ActionInspect, i.Force)

	var vch *vm.VirtualMachine
	if i.Data.ID != "" {
		vch, err = executor.NewVCHFromID(i.Data.ID)
	} else {
		vch, err = executor.NewVCHFromComputePath(i.Data.ComputeResourcePath, i.Data.DisplayName, validator)
	}
	if err != nil {
		op.Errorf("Failed to get Virtual Container Host %s", i.DisplayName)
		op.Error(err)
		return errors.New("inspect failed")
	}

	vchConfig, err := executor.GetNoSecretVCHConfig(vch)
	if err != nil {
		op.Error("Failed to get Virtual Container Host configuration")
		op.Error(err)
		return errors.New("inspect failed")
	}

	return cmd(state{i, op, validator, vchConfig, vch, executor})
}

func (i *Inspect) RunConfig(clic *cli.Context) (err error) {
	op := common.NewOperation(clic, i.Debug.Debug)

	if i.Format == "raw" {
		op.Logger.Level = logrus.ErrorLevel
		op.Logger.Out = os.Stderr
	} else if i.Format != "verbose" {
		op.Warnf("Invalid configuration output format '%s'. Valid options are raw, verbose.", i.Format)
		op.Warn("Using verbose configuration format")
		i.Format = "verbose"
	}

	return i.run(clic, op, func(s state) error {
		err = i.showConfiguration(s.op, s.validator, s.vchConfig, s.vch)
		if err != nil {
			op.Error("Failed to print Virtual Container Host configuration")
			op.Error(err)
			return errors.New("inspect failed")
		}
		return nil
	})
}

func (i *Inspect) Run(clic *cli.Context) (err error) {
	op := common.NewOperation(clic, i.Debug.Debug)

	return i.run(clic, op, func(s state) error {
		installerVer := version.GetBuild()

		op.Info("")
		op.Infof("Installer version: %s", installerVer.ShortVersion())
		op.Infof("VCH version: %s", s.vchConfig.Version.ShortVersion())
		op.Info("")
		op.Info("VCH upgrade status:")
		i.upgradeStatusMessage(s.op, s.vch, installerVer, s.vchConfig.Version)

		if err = s.executor.InspectVCH(s.vch, s.vchConfig, i.CertPath); err != nil {
			s.executor.CollectDiagnosticLogs()
			op.Errorf("%s", err)
			return errors.New("inspect failed")
		}

		op.Infof("Completed successfully")

		return nil
	})
}

func retrieveMapOptions(op trace.Operation, validator *validate.Validator,
	conf *config.VirtualContainerHostConfigSpec, vm *vm.VirtualMachine) (map[string][]string, error) {
	data, err := validate.NewDataFromConfig(op, validator.Session().Finder, conf)
	if err != nil {
		return nil, err
	}
	if err = validator.SetDataFromVM(op, vm, data); err != nil {
		return nil, err
	}
	return converter.DataToOption(data)
}

func (i Inspect) showConfiguration(op trace.Operation, validator *validate.Validator, conf *config.VirtualContainerHostConfigSpec, vm *vm.VirtualMachine) error {
	mapOptions, err := retrieveMapOptions(op, validator, conf, vm)
	if err != nil {
		return err
	}
	options := i.sortedOutput(mapOptions)
	if i.Format == "raw" {
		strOptions := strings.Join(options, " ")
		fmt.Println(strOptions)
	} else if i.Format == "verbose" {
		strOptions := strings.Join(options, "\n\t")
		op.Info("")
		op.Infof("The target VCH is configured with the following options: \n\n\t%s\n", strOptions)
	}

	return nil
}

func (i *Inspect) sortedOutput(mapOptions map[string][]string) (output []string) {
	create := create.NewCreate()
	cFlags := create.Flags()
	for _, f := range cFlags {
		key := f.GetName()
		// change multiple option name to long name: e.g. from target,t => target
		s := strings.Split(key, ",")
		if len(s) > 1 {
			key = s[0]
		}

		values, ok := mapOptions[key]
		if !ok {
			continue
		}

		defaultValue := ""
		switch t := f.(type) {
		case cli.StringFlag:
			defaultValue = t.Value
		case cli.IntFlag:
			defaultValue = strconv.Itoa(t.Value)
		}
		for _, val := range values {
			if val == defaultValue {
				// do not print command option if it's same to default
				continue
			}
			output = append(output, fmt.Sprintf("--%s=%s", key, val))
		}
	}
	return
}

// upgradeStatusMessage generates a user facing status string about upgrade progress and status
func (i *Inspect) upgradeStatusMessage(op trace.Operation, vch *vm.VirtualMachine, installerVer *version.Build, vchVer *version.Build) {
	interaction.LogUpgradeStatusLongMessage(op, vch, installerVer, vchVer)
	return
}
