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
	"net/http"

	"github.com/go-openapi/runtime/middleware"

	"github.com/vmware/vic/lib/apiservers/service/models"
	"github.com/vmware/vic/lib/apiservers/service/restapi/handlers/decode"
	"github.com/vmware/vic/lib/apiservers/service/restapi/handlers/errors"
	"github.com/vmware/vic/lib/apiservers/service/restapi/handlers/target"
	"github.com/vmware/vic/lib/apiservers/service/restapi/operations"
	"github.com/vmware/vic/lib/install/management"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/version"
)

// VCHDelete is the handler for deleting a VCH without specifying a datacenter
type VCHDelete struct {
	vchDelete
}

// VCHDatacenterDelete is the handler for deleting a VCH within a specified datacenter
type VCHDatacenterDelete struct {
	vchDelete
}

// vchDelete allows for VCHDelete and VCHDatacenterDelete to share common code without polluting the package
type vchDelete struct{}

// Handle is the handler implementation for deleting a VCH without specifying a datacenter
func (h *VCHDelete) Handle(params operations.DeleteTargetTargetVchVchIDParams, principal interface{}) middleware.Responder {
	op := trace.FromContext(params.HTTPRequest.Context(), "VCHDelete: %s", params.VchID)

	b := target.Params{
		Target:     params.Target,
		Thumbprint: params.Thumbprint,
		VCHID:      &params.VchID,
	}

	err := h.handle(op, b, principal, params.DeletionSpecification)
	if err != nil {
		return operations.NewDeleteTargetTargetVchVchIDDefault(errors.StatusCode(err)).WithPayload(&models.Error{Message: err.Error()})
	}

	return operations.NewDeleteTargetTargetVchVchIDAccepted()
}

// Handle is the handler implementation for deleting a VCH within a specified datacenter
func (h *VCHDatacenterDelete) Handle(params operations.DeleteTargetTargetDatacenterDatacenterVchVchIDParams, principal interface{}) middleware.Responder {
	op := trace.FromContext(params.HTTPRequest.Context(), "VCHDatacenterDelete: %s", params.VchID)

	b := target.Params{
		Target:     params.Target,
		Thumbprint: params.Thumbprint,
		Datacenter: &params.Datacenter,
		VCHID:      &params.VchID,
	}

	err := h.handle(op, b, principal, params.DeletionSpecification)
	if err != nil {
		return operations.NewDeleteTargetTargetDatacenterDatacenterVchVchIDDefault(errors.StatusCode(err)).WithPayload(&models.Error{Message: err.Error()})
	}

	return operations.NewDeleteTargetTargetDatacenterDatacenterVchVchIDAccepted()
}

// handle deletes the VCH described by params based on the preferences expressed by specification, using the credentials
// from principal. If the VCH cannot be found, a 404 is returned. If an error occurs during deletion, a 500 is returned.
func (h *vchDelete) handle(op trace.Operation, params target.Params, principal interface{}, specification *models.DeletionSpecification) error {
	d, c, err := target.Validate(op, management.ActionDelete, params, principal)
	if err != nil {
		return err
	}

	vchConfig, err := c.GetVCHConfig(op, d)
	if err != nil {
		return err
	}

	// compare vch version and vic-machine version
	installerBuild := version.GetBuild()
	if vchConfig.Version == nil || !installerBuild.Equal(vchConfig.Version) {
		op.Debugf("VCH version %q is different than API version %s", vchConfig.Version.ShortVersion(), installerBuild.ShortVersion())
	}

	deleteContainers, deleteVolumeStores := decode.FromDeletionSpecification(specification)
	err = c.Executor().DeleteVCH(vchConfig, deleteContainers, deleteVolumeStores)
	if err != nil {
		return errors.NewError(http.StatusInternalServerError, "failed to delete VCH: %s", err)
	}

	return nil
}
