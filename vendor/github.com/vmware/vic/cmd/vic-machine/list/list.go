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

package list

import (
	"context"
	"fmt"
	"path"
	"text/tabwriter"
	"text/template"
	"time"

	"gopkg.in/urfave/cli.v1"

	"github.com/vmware/vic/cmd/vic-machine/common"
	"github.com/vmware/vic/lib/install/data"
	"github.com/vmware/vic/lib/install/management"
	"github.com/vmware/vic/lib/install/validate"
	"github.com/vmware/vic/pkg/errors"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/version"
	"github.com/vmware/vic/pkg/vsphere/vm"
)

type items struct {
	ID            string
	Path          string
	Name          string
	Version       string
	UpgradeStatus string
}

// templ is parsed by text/template package
const templ = `{{range .}}
{{.ID}}	{{.Path}}	{{.Name}}	{{.Version}}	{{.UpgradeStatus}}{{end}}
`

// List has all input parameters for vic-machine ls command
type List struct {
	*data.Data

	executor *management.Dispatcher
}

func NewList() *List {
	d := &List{}
	d.Data = data.NewData()
	return d
}

// Flags return all cli flags for ls
func (l *List) Flags() []cli.Flag {
	util := []cli.Flag{
		cli.DurationFlag{
			Name:        "timeout",
			Value:       3 * time.Minute,
			Usage:       "Time to wait for list",
			Destination: &l.Timeout,
		},
	}

	target := l.TargetFlags()
	// TODO: why not allow name as a filter, like most list operations
	compute := l.ComputeFlagsNoName()
	debug := l.DebugFlags(true)

	// flag arrays are declared, now combined
	var flags []cli.Flag
	for _, f := range [][]cli.Flag{target, compute, util, debug} {
		flags = append(flags, f...)
	}

	return flags
}

func (l *List) processParams(op trace.Operation) error {
	defer trace.End(trace.Begin("", op))

	if err := l.HasCredentials(op); err != nil {
		return err
	}

	return nil
}

func (l *List) prettyPrint(op trace.Operation, cli *cli.Context, vchs []*vm.VirtualMachine, executor *management.Dispatcher) {
	data := []items{
		{"ID", "PATH", "NAME", "VERSION", "UPGRADE STATUS"},
	}
	installerVer := version.GetBuild()
	for _, vch := range vchs {

		vchConfig, err := executor.GetNoSecretVCHConfig(vch)
		var version string
		var upgradeStatus string
		if err != nil {
			op.Warnf("Failed to get Virtual Container Host configuration for VCH %q: %s", vch.Reference().Value, err)
			op.Warnf("Skip listing VCH %q", vch.Reference().Value)
			version = "unknown"
			upgradeStatus = "unknown"
		} else {
			version = vchConfig.Version.ShortVersion()
			upgradeStatus = l.upgradeStatusMessage(op, vch, installerVer, vchConfig.Version)
		}
		// When the VCH was found the inventory path was overwritten with the resource pool path, so
		// to print the path to the pool we need to call Dir twice.
		data = append(data,
			items{vch.Reference().Value, path.Dir(path.Dir(vch.InventoryPath)), vch.Name(), version, upgradeStatus})
	}
	t := template.New("vic-machine ls")
	// #nosec: Errors unhandled.
	t, _ = t.Parse(templ)
	w := tabwriter.NewWriter(cli.App.Writer, 8, 8, 8, ' ', 0)
	if err := t.Execute(w, data); err != nil {
		op.Fatal(err)
	}
	// #nosec: Errors unhandled.
	w.Flush()
}

func (l *List) Run(clic *cli.Context) (err error) {
	op := common.NewOperation(clic, l.Debug.Debug)
	defer func() {
		// urfave/cli will print out exit in error handling, so no more information in main method can be printed out.
		err = common.LogErrorIfAny(op, clic, err)
	}()
	op, cancel := trace.WithTimeout(&op, l.Timeout, clic.App.Name)
	defer cancel()
	defer func() {
		if op.Err() != nil && op.Err() == context.DeadlineExceeded {
			//context deadline exceeded, replace returned error message
			err = errors.Errorf("List timed out: use --timeout to add more time")
		}
	}()

	if err = l.processParams(op); err != nil {
		return err
	}

	if len(clic.Args()) > 0 {
		op.Errorf("Unknown argument: %s", clic.Args()[0])
		return errors.New("invalid CLI arguments")
	}

	op.Infof("### Listing VCHs ####")

	var validator *validate.Validator
	if validator, err = validate.NewValidator(op, l.Data); err != nil {
		op.Errorf("List cannot continue - failed to create validator: %s", err)
		return errors.New("list failed")
	}
	defer validator.Session.Logout(op)

	// If dc is not set, and multiple datacenter is available, vic-machine ls will list VCHs under all datacenters.
	validator.AllowEmptyDC()

	_, err = validator.ValidateTarget(op, l.Data)
	if err != nil {
		op.Errorf("List cannot continue - target validation failed: %s", err)
		return errors.New("list failed")
	}
	_, err = validator.ValidateCompute(op, l.Data, false)
	if err != nil {
		op.Errorf("List cannot continue - compute resource validation failed: %s", err)
		return errors.New("list failed")
	}

	executor := management.NewDispatcher(validator.Context, validator.Session, management.ListAction, false)
	vchs, err := executor.SearchVCHs(validator.ClusterPath)
	if err != nil {
		op.Errorf("List cannot continue - failed to search VCHs in %s: %s", validator.ResourcePoolPath, err)
	}
	l.prettyPrint(op, clic, vchs, executor)
	return nil
}

// upgradeStatusMessage generates a user facing status string about upgrade progress and status
func (l *List) upgradeStatusMessage(op trace.Operation, vch *vm.VirtualMachine, installerVer *version.Build, vchVer *version.Build) string {
	if sameVer := installerVer.Equal(vchVer); sameVer {
		return "Up to date"
	}

	upgrading, err := vch.VCHUpdateStatus(op)
	if err != nil {
		return fmt.Sprintf("Unknown: %s", err)
	}
	if upgrading {
		return "Upgrade in progress"
	}

	canUpgrade, err := installerVer.IsNewer(vchVer)
	if err != nil {
		return fmt.Sprintf("Unknown: %s", err)
	}
	if canUpgrade {
		return fmt.Sprintf("Upgradeable to %s", installerVer.ShortVersion())
	}

	oldInstaller, err := installerVer.IsOlder(vchVer)
	if err != nil {
		return fmt.Sprintf("Unknown: %s", err)
	}
	if oldInstaller {
		return fmt.Sprintf("VCH has newer version")
	}

	// can't get here
	return "Invalid upgrade status"
}
