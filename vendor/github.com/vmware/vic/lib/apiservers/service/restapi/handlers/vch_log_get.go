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
	"bytes"
	"net/http"
	"sort"

	"github.com/go-openapi/runtime/middleware"

	"github.com/vmware/vic/lib/apiservers/service/restapi/handlers/errors"
	"github.com/vmware/vic/lib/apiservers/service/restapi/handlers/target"
	"github.com/vmware/vic/lib/apiservers/service/restapi/operations"
	"github.com/vmware/vic/lib/install/management"
	"github.com/vmware/vic/lib/install/vchlog"
	"github.com/vmware/vic/pkg/trace"
)

// VCHLogGet is the handler for getting the log messages for a VCH without specifying a datacenter
type VCHLogGet struct {
	vchLogGet
}

// VCHDatacenterLogGet is the handler for getting the log messages for a VCH within a specified datacenter
type VCHDatacenterLogGet struct {
	vchLogGet
}

// vchLogGet allows for VCHLogGet and VCHDatacenterLogGet to share common code without polluting the package
type vchLogGet struct{}

// Handle is the handler implementation for getting the log messages for a VCH without specifying a datacenter
func (h *VCHLogGet) Handle(params operations.GetTargetTargetVchVchIDLogParams, principal interface{}) middleware.Responder {
	op := trace.FromContext(params.HTTPRequest.Context(), "VCHLogGet: %s", params.VchID)

	b := target.Params{
		Target:     params.Target,
		Thumbprint: params.Thumbprint,
		VCHID:      &params.VchID,
	}

	output, err := h.handle(op, b, principal)
	if err != nil {
		return operations.NewGetTargetTargetVchVchIDLogDefault(errors.StatusCode(err)).WithPayload(err.Error())
	}

	return operations.NewGetTargetTargetVchVchIDLogOK().WithPayload(output)
}

// Handle is the handler implementation for getting the log messages for a VCH within a specified datacenter
func (h *VCHDatacenterLogGet) Handle(params operations.GetTargetTargetDatacenterDatacenterVchVchIDLogParams, principal interface{}) middleware.Responder {
	op := trace.FromContext(params.HTTPRequest.Context(), "VCHDatacenterLogGet: %s", params.VchID)

	b := target.Params{
		Target:     params.Target,
		Thumbprint: params.Thumbprint,
		Datacenter: &params.Datacenter,
		VCHID:      &params.VchID,
	}

	output, err := h.handle(op, b, principal)
	if err != nil {
		return operations.NewGetTargetTargetDatacenterDatacenterVchVchIDLogDefault(errors.StatusCode(err)).WithPayload(err.Error())
	}

	return operations.NewGetTargetTargetDatacenterDatacenterVchVchIDLogOK().WithPayload(output)
}

// handleVCHLogGet downloads all log files in datastore and concatenates the content
func (h *vchLogGet) handle(op trace.Operation, params target.Params, principal interface{}) (string, error) {
	d, c, err := target.Validate(op, management.ActionInspectLogs, params, principal)
	if err != nil {
		return "", err
	}

	helper, err := c.GetDatastoreHelper(op, d)
	if err != nil {
		return "", err
	}

	res, err := helper.Ls(op, "", vchlog.LogFilePrefix+"*"+vchlog.LogFileSuffix)
	if err != nil {
		return "", errors.NewError(http.StatusInternalServerError, "unable to list vic-machine log files in datastore: %s", err)
	}

	if len(res.File) == 0 {
		return "", errors.NewError(http.StatusNotFound, "no log file available in datastore folder")
	}

	var paths []string
	for _, f := range res.File {
		paths = append(paths, f.GetFileInfo().Path)
	}

	// sort log files based on timestamp
	sort.Strings(paths)

	// concatenate the content
	var buffer bytes.Buffer
	for _, p := range paths {
		reader, err := helper.Download(op, p)
		if err != nil {
			return "", errors.NewError(http.StatusInternalServerError, "unable to download log file %s: %s", p, err)
		}

		if _, err := buffer.ReadFrom(reader); err != nil {
			return "", errors.NewError(http.StatusInternalServerError, "error reading from log file %s: %s", p, err)
		}
	}

	return string(buffer.Bytes()), nil
}
