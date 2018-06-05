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
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/vic/lib/config"
	"github.com/vmware/vic/lib/install/data"
	"github.com/vmware/vic/pkg/errors"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/version"
)

func (d *Dispatcher) createVApp(conf *config.VirtualContainerHostConfigSpec, settings *data.InstallerData) (*object.VirtualApp, error) {
	defer trace.End(trace.Begin("", d.op))
	var err error
	d.op.Infof("Creating virtual app %q", conf.Name)

	resSpec := types.DefaultResourceConfigSpec()

	if settings.VCHSize.CPU.Limit != nil && *settings.VCHSize.CPU.Limit != 0 {
		resSpec.CpuAllocation.Limit = settings.VCHSize.CPU.Limit
	}

	if settings.VCHSize.CPU.Reservation != nil && *settings.VCHSize.CPU.Reservation != 0 {
		resSpec.CpuAllocation.Reservation = settings.VCHSize.CPU.Reservation
	}

	if settings.VCHSize.CPU.Shares != nil {
		resSpec.CpuAllocation.Shares = settings.VCHSize.CPU.Shares
	}

	if settings.VCHSize.Memory.Limit != nil && *settings.VCHSize.Memory.Limit != 0 {
		resSpec.MemoryAllocation.Limit = settings.VCHSize.Memory.Limit
	}

	if settings.VCHSize.Memory.Reservation != nil && *settings.VCHSize.Memory.Reservation != 0 {
		resSpec.MemoryAllocation.Reservation = settings.VCHSize.Memory.Reservation
	}

	if settings.VCHSize.Memory.Shares != nil {
		resSpec.MemoryAllocation.Shares = settings.VCHSize.Memory.Shares
	}

	prodSpec := types.VAppProductSpec{
		Info: &types.VAppProductInfo{
			Name:      "vSphere Integrated Containers",
			Vendor:    "VMware",
			VendorUrl: "http://www.vmware.com/",
			Version:   version.Version,
		},
		ArrayUpdateSpec: types.ArrayUpdateSpec{
			Operation: types.ArrayUpdateOperationAdd,
		},
	}

	configSpec := types.VAppConfigSpec{
		Annotation: "vSphere Integrated Containers",
		VmConfigSpec: types.VmConfigSpec{
			Product: []types.VAppProductSpec{prodSpec},
		},
	}

	app, err := d.session.Pool.CreateVApp(d.op, conf.Name, resSpec, configSpec, d.session.VMFolder)
	if err != nil {
		d.op.Debugf("Failed to create virtual app %q: %s", conf.Name, err)
		return nil, err
	}
	conf.ComputeResources = append(conf.ComputeResources, app.Reference())
	return app, nil
}

func (d *Dispatcher) findVirtualApp(path string) (*object.VirtualApp, error) {
	defer trace.End(trace.Begin(path, d.op))
	vapp, err := d.session.Finder.VirtualApp(d.op, path)
	if err != nil {
		_, ok := err.(*find.NotFoundError)
		if !ok {
			err = errors.Errorf("Failed to query virtual app %q: %s", path, err)
			return nil, err
		}
		return nil, nil
	}
	return vapp, nil
}
