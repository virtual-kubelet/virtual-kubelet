// Copyright 2016 VMware, Inc. All Rights Reserved.
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

	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/vic/lib/config"
	"github.com/vmware/vic/pkg/errors"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/vsphere/vm"
)

func (d *Dispatcher) DebugVCH(vch *vm.VirtualMachine, conf *config.VirtualContainerHostConfigSpec, password, authorizedKey string) error {
	defer trace.End(trace.Begin(conf.Name, d.op))

	op := trace.FromContext(d.op, "enable appliance debug")

	err := d.enableSSH(op, vch, password, authorizedKey)
	if err != nil {
		op.Errorf("Unable to enable ssh on the VCH appliance VM: %s", err)
		return err
	}

	d.sshEnabled = true

	return nil
}

func (d *Dispatcher) enableSSH(ctx context.Context, vch *vm.VirtualMachine, password, authorizedKey string) error {
	op := trace.FromContext(ctx, "enable ssh in appliance")

	state, err := vch.PowerState(op)
	if err != nil {
		op.Error("Failed to get appliance power state, service might not be available at this moment.")
	}
	if state != types.VirtualMachinePowerStatePoweredOn {
		err = errors.Errorf("VCH appliance is not powered on, state %s", state)
		op.Errorf("%s", err)
		return err
	}

	running, err := vch.IsToolsRunning(op)
	if err != nil || !running {
		err = errors.New("Tools is not running in the appliance, unable to continue")
		op.Errorf("%s", err)
		return err
	}

	pm, err := d.opManager(vch)
	if err != nil {
		err = errors.Errorf("Unable to manage processes in appliance VM: %s", err)
		op.Errorf("%s", err)
		return err
	}

	auth := types.NamePasswordAuthentication{}

	spec := types.GuestProgramSpec{
		ProgramPath: "enable-ssh",
		Arguments:   string(authorizedKey),
	}

	pid, err := pm.StartProgram(op, &auth, &spec)
	if err != nil {
		err = errors.Errorf("Unable to enable SSH in appliance VM: %s", err)
		op.Errorf("%s", err)
		return err
	}

	_, err = d.opManagerWait(op, pm, &auth, pid)
	if err != nil {
		err = errors.Errorf("Unable to check enable SSH status: %s", err)
		op.Errorf("%s", err)
		return err
	}

	if password == "" {
		return nil
	}

	// set the password as well
	spec = types.GuestProgramSpec{
		ProgramPath: "passwd",
		Arguments:   password,
	}

	pid, err = pm.StartProgram(op, &auth, &spec)
	if err != nil {
		err = errors.Errorf("Unable to enable passwd in appliance VM: %s", err)
		op.Errorf("%s", err)
		return err
	}

	_, err = d.opManagerWait(op, pm, &auth, pid)
	if err != nil {
		err = errors.Errorf("Unable to check enable passwd status: %s", err)
		op.Errorf("%s", err)
		return err
	}

	return nil
}
