// Copyright 2017-2018 VMware, Inc. All Rights Reserved.
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

package handlers

import (
	"github.com/go-openapi/runtime/middleware"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"

	"github.com/vmware/vic/lib/apiservers/service/models"
	"github.com/vmware/vic/lib/apiservers/service/restapi/handlers/client"
	"github.com/vmware/vic/lib/apiservers/service/restapi/handlers/encode"
	"github.com/vmware/vic/lib/apiservers/service/restapi/handlers/errors"
	"github.com/vmware/vic/lib/apiservers/service/restapi/handlers/target"
	"github.com/vmware/vic/lib/apiservers/service/restapi/operations"
	"github.com/vmware/vic/lib/install/interaction"
	"github.com/vmware/vic/lib/install/management"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/version"
	"github.com/vmware/vic/pkg/vsphere/vm"
)

// VCHListGet is the handler for listing VCHs without specifying a datacenter
type VCHListGet struct {
	vchListGet
}

// VCHDatacenterListGet is the handler for listing VCHs within a specified datacenter
type VCHDatacenterListGet struct {
	vchListGet
}

// vchListGet  allows for VCHListGet and VCHDatacenterListGet to share common code without polluting the package
type vchListGet struct{}

// Handle is the handler implementation for listing VCHs without specifying a datacenter
func (h *VCHListGet) Handle(params operations.GetTargetTargetVchParams, principal interface{}) middleware.Responder {
	op := trace.FromContext(params.HTTPRequest.Context(), "VCHListGet")

	b := target.Params{
		Target:          params.Target,
		Thumbprint:      params.Thumbprint,
		ComputeResource: params.ComputeResource,
	}

	vchs, err := h.handle(op, b, principal)
	if err != nil {
		return operations.NewGetTargetTargetVchDefault(errors.StatusCode(err)).WithPayload(&models.Error{Message: err.Error()})
	}

	return operations.NewGetTargetTargetVchOK().WithPayload(operations.GetTargetTargetVchOKBody{Vchs: vchs})
}

// Handle is the handler implementation for listing VCHs within a specified datacenter
func (h *VCHDatacenterListGet) Handle(params operations.GetTargetTargetDatacenterDatacenterVchParams, principal interface{}) middleware.Responder {
	op := trace.FromContext(params.HTTPRequest.Context(), "VCHDatacenterListGet")

	b := target.Params{
		Target:          params.Target,
		Thumbprint:      params.Thumbprint,
		Datacenter:      &params.Datacenter,
		ComputeResource: params.ComputeResource,
	}

	vchs, err := h.handle(op, b, principal)
	if err != nil {
		return operations.NewGetTargetTargetDatacenterDatacenterVchDefault(errors.StatusCode(err)).WithPayload(&models.Error{Message: err.Error()})
	}

	return operations.NewGetTargetTargetVchOK().WithPayload(operations.GetTargetTargetVchOKBody{Vchs: vchs})
}

func (h *vchListGet) handle(op trace.Operation, params target.Params, principal interface{}) ([]*models.VCHListItem, error) {
	_, c, err := target.Validate(op, management.ActionList, params, principal)
	if err != nil {
		return nil, err
	}

	vchs, err := c.GetVCHs(op)
	if err != nil {
		return nil, err
	}

	return h.vchsToModels(op, c, vchs), nil
}

func (h *vchListGet) vchsToModels(op trace.Operation, c *client.HandlerClient, vchs []*vm.VirtualMachine) []*models.VCHListItem {
	installerVer := version.GetBuild()
	payload := make([]*models.VCHListItem, 0)

	for _, vch := range vchs {
		name := vch.Name()
		id := vch.Reference().Value

		vchConfig, err := c.GetConfigForVCH(op, vch)
		// If we can't get the extra config from this VCH, the VCH at this point could already been deleted / partially created / corrupted
		// we ignore this partial VCH, log the error and skip to next one
		if err != nil {
			op.Warnf("Failed to get extra config from VCH %s, %s", id, err)
			continue
		}

		dockerHost, adminPortal, err := c.GetAddresses(vchConfig)
		if err != nil {
			op.Warnf("Failed to get docker host and admin portal address for VCH %s: %s", id, err)
		}

		powerState, err := vch.PowerState(op)
		if err != nil {
			op.Warnf("Failed to get power state of VCH %s: %s", id, err)
			powerState = "error"
		}

		model := &models.VCHListItem{
			ID:          id,
			Name:        name,
			AdminPortal: adminPortal,
			DockerHost:  dockerHost,
			PowerState:  string(powerState),
		}

		model.Parent = h.parent(op, vch)

		version := vchConfig.Version
		if version != nil {
			model.Version = version.ShortVersion()
			model.UpgradeStatus = interaction.GetUpgradeStatusShortMessage(op, vch, installerVer, version)
		}

		payload = append(payload, model)
	}

	return payload
}

func (h *vchListGet) parent(op trace.Operation, vch *vm.VirtualMachine) *models.ManagedObject {
	id := vch.Reference().Value

	// Once we know we will never encounter vApp-based VCHs, we can probably
	// replace much of the following with a call to `vch.ResourcePool(op)`.
	var mvm mo.VirtualMachine
	if err := vch.Properties(op, vch.Reference(), []string{"parentVApp", "resourcePool"}, &mvm); err != nil {
		op.Debugf("Failed to get parent of VCH %s: %s", id, err)
		return nil
	}
	if mvm.ParentVApp != nil {
		op.Debugf("VCH %s has vApp parent, omitting", id)
		return nil
	}

	if mvm.ResourcePool == nil {
		op.Debugf("VCH %s does not have a resource pool, omitting", id)
		return nil
	}

	rp := object.NewResourcePool(vch.Common.Client(), *mvm.ResourcePool)
	mo := encode.AsManagedObject(rp)
	return &mo
}
