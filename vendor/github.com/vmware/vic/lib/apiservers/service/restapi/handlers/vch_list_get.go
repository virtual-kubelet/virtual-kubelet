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
	"fmt"
	"net/http"

	"github.com/go-openapi/runtime/middleware"

	"github.com/vmware/vic/lib/apiservers/service/models"
	"github.com/vmware/vic/lib/apiservers/service/restapi/handlers/util"
	"github.com/vmware/vic/lib/apiservers/service/restapi/operations"
	"github.com/vmware/vic/lib/install/data"
	"github.com/vmware/vic/lib/install/management"
	"github.com/vmware/vic/lib/install/validate"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/version"
	"github.com/vmware/vic/pkg/vsphere/vm"
)

// VCHListGet is the handler for listing VCHs
type VCHListGet struct {
}

// VCHDatacenterListGet is the handler for listing VCHs within a Datacenter
type VCHDatacenterListGet struct {
}

func (h *VCHListGet) Handle(params operations.GetTargetTargetVchParams, principal interface{}) middleware.Responder {
	op := trace.FromContext(params.HTTPRequest.Context(), "VCHListGet")

	b := buildDataParams{
		target:          params.Target,
		thumbprint:      params.Thumbprint,
		computeResource: params.ComputeResource,
	}

	d, validator, err := buildDataAndValidateTarget(op, b, principal)
	if err != nil {
		return operations.NewGetTargetTargetVchDefault(util.StatusCode(err)).WithPayload(&models.Error{Message: err.Error()})
	}

	vchs, err := listVCHs(op, d, validator)
	if err != nil {
		return operations.NewGetTargetTargetVchDefault(util.StatusCode(err)).WithPayload(&models.Error{Message: err.Error()})
	}

	return operations.NewGetTargetTargetVchOK().WithPayload(operations.GetTargetTargetVchOKBody{Vchs: vchs})
}

func (h *VCHDatacenterListGet) Handle(params operations.GetTargetTargetDatacenterDatacenterVchParams, principal interface{}) middleware.Responder {
	op := trace.FromContext(params.HTTPRequest.Context(), "VCHDatacenterListGet")

	b := buildDataParams{
		target:          params.Target,
		thumbprint:      params.Thumbprint,
		datacenter:      &params.Datacenter,
		computeResource: params.ComputeResource,
	}

	d, validator, err := buildDataAndValidateTarget(op, b, principal)
	if err != nil {
		return operations.NewGetTargetTargetDatacenterDatacenterVchDefault(util.StatusCode(err)).WithPayload(&models.Error{Message: err.Error()})
	}

	vchs, err := listVCHs(op, d, validator)
	if err != nil {
		return operations.NewGetTargetTargetDatacenterDatacenterVchDefault(util.StatusCode(err)).WithPayload(&models.Error{Message: err.Error()})
	}

	return operations.NewGetTargetTargetVchOK().WithPayload(operations.GetTargetTargetVchOKBody{Vchs: vchs})
}

func listVCHs(op trace.Operation, d *data.Data, validator *validate.Validator) ([]*models.VCHListItem, error) {

	executor := management.NewDispatcher(validator.Context, validator.Session, management.NoAction, false)
	vchs, err := executor.SearchVCHs(validator.ClusterPath)
	if err != nil {
		return nil, util.NewError(http.StatusInternalServerError, fmt.Sprintf("Failed to search VCHs in %s: %s", validator.ResourcePoolPath, err))
	}

	return vchsToModels(op, vchs, executor), nil
}

func vchsToModels(op trace.Operation, vchs []*vm.VirtualMachine, executor *management.Dispatcher) []*models.VCHListItem {
	installerVer := version.GetBuild()
	payload := make([]*models.VCHListItem, 0)

	for _, vch := range vchs {
		name := vch.Name()
		id := vch.Reference().Value

		vchConfig, err := executor.GetNoSecretVCHConfig(vch)
		// If we can't get the extra config from this VCH, the VCH at this point could already been deleted / partially created / corrupted
		// we ignore this partial VCH, log the error and skip to next one
		if err != nil {
			op.Warnf("Failed to get extra config from VCH %s: %s", id, err)
			continue
		}

		version := vchConfig.Version
		dockerHost, adminPortal, err := getAddresses(executor, vchConfig)
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

		if version != nil {
			model.Version = version.ShortVersion()
			model.UpgradeStatus = upgradeStatusMessage(op, vch, installerVer, version)
		}
		payload = append(payload, model)
	}

	return payload
}
