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

// Package target encapsulates the logic for translating the raw input about
// the intended target of an API operation (vSphere endpoint, thumbprint,
// datacenter, compute resource, VCH ID) into structs handlers can use.
package target

import (
	"net/http"
	"net/url"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/vmware/vic/cmd/vic-machine/common"
	"github.com/vmware/vic/lib/apiservers/service/restapi/handlers/client"
	"github.com/vmware/vic/lib/apiservers/service/restapi/handlers/errors"
	"github.com/vmware/vic/lib/install/data"
	"github.com/vmware/vic/lib/install/management"
	"github.com/vmware/vic/lib/install/validate"
	"github.com/vmware/vic/pkg/trace"
)

// Params captures the raw target information, much of which is optional.
type Params struct {
	Target          string
	Thumbprint      *string
	Datacenter      *string
	ComputeResource *string
	VCHID           *string
}

// Validate params and build data and config structs from them.
func Validate(op trace.Operation, action management.Action, params Params, principal interface{}) (*data.Data, *client.HandlerClient, error) {
	data := &data.Data{
		Target: &common.Target{
			URL: &url.URL{Host: params.Target},
		},
	}

	if c, ok := principal.(Credentials); ok {
		data.Target.User = c.user
		data.Target.Password = &c.pass
	} else {
		data.Target.CloneTicket = principal.(Session).ticket
	}

	if err := data.HasCredentials(op); err != nil {
		return data, nil, errors.NewError(http.StatusUnauthorized, "invalid credentials: %s", err)
	}

	if params.Thumbprint != nil {
		data.Thumbprint = *params.Thumbprint
	}

	if params.ComputeResource != nil {
		data.ComputeResourcePath = *params.ComputeResource
	}

	if params.VCHID != nil {
		data.ID = *params.VCHID
	}

	var validator *validate.Validator
	var allowEmptyDC bool
	if params.Datacenter != nil {
		s, err := validate.NewSession(op, data)
		if err != nil {
			return data, nil, errors.NewError(http.StatusBadRequest, "session error: %s", err)
		}

		datacenterManagedObjectReference := types.ManagedObjectReference{Type: "Datacenter", Value: *params.Datacenter}

		datacenterObject, err := s.Finder.ObjectReference(op, datacenterManagedObjectReference)
		if err != nil {
			return nil, nil, errors.WrapError(http.StatusNotFound, err)
		}

		dc, ok := datacenterObject.(*object.Datacenter)
		if !ok {
			return data, nil, errors.NewError(http.StatusBadRequest, "validation error: datacenter parameter is not a datacenter moref")
		}

		err = s.SetDatacenter(op, dc)
		if err != nil {
			return data, nil, errors.NewError(http.StatusBadRequest, "validation error: error finding datacenter folders: %s", err)
		}

		v, err := validate.CreateFromSession(op, s)
		if err != nil {
			return data, nil, errors.NewError(http.StatusBadRequest, "validation error: %s", err)
		}

		validator = v
	} else {
		v, err := validate.NewValidator(op, data)
		if err != nil {
			return data, nil, errors.NewError(http.StatusBadRequest, "validation error: %s", err)
		}

		// If dc is not set, and multiple datacenters are available, operate on all datacenters.
		allowEmptyDC = true

		validator = v
	}

	if _, err := validator.ValidateTarget(op, data, allowEmptyDC); err != nil {
		return data, nil, errors.NewError(http.StatusBadRequest, "target validation failed: %s", err)
	}

	if _, err := validator.ValidateCompute(op, data, false); err != nil {
		return data, nil, errors.NewError(http.StatusBadRequest, "compute resource validation failed: %s", err)
	}

	c, err := client.NewHandlerClient(op, action, validator.Session().Finder, validator.Session(), validator)

	return data, c, err
}
