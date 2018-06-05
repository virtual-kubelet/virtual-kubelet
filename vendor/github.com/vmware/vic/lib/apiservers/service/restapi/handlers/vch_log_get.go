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

package handlers

import (
	"bytes"
	"fmt"
	"net/http"
	"sort"

	"github.com/go-openapi/runtime/middleware"

	"github.com/vmware/vic/lib/apiservers/service/restapi/handlers/util"
	"github.com/vmware/vic/lib/apiservers/service/restapi/operations"
	"github.com/vmware/vic/lib/install/data"
	"github.com/vmware/vic/lib/install/management"
	"github.com/vmware/vic/lib/install/validate"
	"github.com/vmware/vic/lib/install/vchlog"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/vsphere/datastore"
)

// VCHLogGet is the handler for getting the log messages for a VCH
type VCHLogGet struct {
}

// VCHDatacenterLogGet is the handler for getting the log messages for a VCH within a Datacenter
type VCHDatacenterLogGet struct {
}

func (h *VCHLogGet) Handle(params operations.GetTargetTargetVchVchIDLogParams, principal interface{}) middleware.Responder {
	op := trace.FromContext(params.HTTPRequest.Context(), "VCHLogGet: %s", params.VchID)

	b := buildDataParams{
		target:     params.Target,
		thumbprint: params.Thumbprint,
		vchID:      &params.VchID,
	}

	d, validator, err := buildDataAndValidateTarget(op, b, principal)
	if err != nil {
		return operations.NewGetTargetTargetVchVchIDLogDefault(util.StatusCode(err)).WithPayload(err.Error())
	}

	helper, err := getDatastoreHelper(op, d, validator)
	if err != nil {
		return operations.NewGetTargetTargetVchVchIDLogDefault(util.StatusCode(err)).WithPayload(err.Error())
	}

	output, err := getAllLogs(op, helper)
	if err != nil {
		return operations.NewGetTargetTargetVchVchIDLogDefault(util.StatusCode(err)).WithPayload(err.Error())
	}

	return operations.NewGetTargetTargetVchVchIDLogOK().WithPayload(output)
}

func (h *VCHDatacenterLogGet) Handle(params operations.GetTargetTargetDatacenterDatacenterVchVchIDLogParams, principal interface{}) middleware.Responder {
	op := trace.FromContext(params.HTTPRequest.Context(), "VCHDatacenterLogGet: %s", params.VchID)

	b := buildDataParams{
		target:     params.Target,
		thumbprint: params.Thumbprint,
		datacenter: &params.Datacenter,
		vchID:      &params.VchID,
	}

	d, validator, err := buildDataAndValidateTarget(op, b, principal)
	if err != nil {
		return operations.NewGetTargetTargetDatacenterDatacenterVchVchIDLogDefault(util.StatusCode(err)).WithPayload(err.Error())
	}

	helper, err := getDatastoreHelper(op, d, validator)
	if err != nil {
		return operations.NewGetTargetTargetDatacenterDatacenterVchVchIDLogDefault(util.StatusCode(err)).WithPayload(err.Error())
	}

	output, err := getAllLogs(op, helper)
	if err != nil {
		return operations.NewGetTargetTargetDatacenterDatacenterVchVchIDLogDefault(util.StatusCode(err)).WithPayload(err.Error())
	}

	return operations.NewGetTargetTargetDatacenterDatacenterVchVchIDLogOK().WithPayload(output)
}

// getDatastoreHelper validates the VCH and returns the datastore helper for the VCH. It errors when validation fails or when datastore is not ready
func getDatastoreHelper(op trace.Operation, d *data.Data, validator *validate.Validator) (*datastore.Helper, error) {
	executor := management.NewDispatcher(validator.Context, validator.Session, management.NoAction, false)
	vch, err := executor.NewVCHFromID(d.ID)
	if err != nil {
		return nil, util.NewError(http.StatusNotFound, fmt.Sprintf("Unable to find VCH %s: %s", d.ID, err))
	}

	if err := validate.SetDataFromVM(validator.Context, validator.Session.Finder, vch, d); err != nil {
		return nil, util.NewError(http.StatusInternalServerError, fmt.Sprintf("Failed to load VCH data: %s", err))
	}

	// Relative path of datastore folder
	vmPath, err := vch.VMPathNameAsURL(op)
	if err != nil {
		return nil, util.NewError(http.StatusNotFound, fmt.Sprintf("Unable to retrieve VCH datastore information: %s", err))
	}

	// Get VCH datastore object
	ds, err := validator.Session.Finder.Datastore(validator.Context, vmPath.Host)
	if err != nil {
		return nil, util.NewError(http.StatusNotFound, fmt.Sprintf("Datastore folder not found for VCH %s: %s", d.ID, err))
	}

	// Create a new datastore helper for file finding
	helper, err := datastore.NewHelper(op, validator.Session, ds, vmPath.Path)
	if err != nil {
		return nil, fmt.Errorf("Unable to get datastore helper: %s", err)
	}

	return helper, nil
}

// getAllLogs downloads all log files in datastore and concatenates the content
func getAllLogs(op trace.Operation, helper *datastore.Helper) (string, error) {
	res, err := helper.Ls(op, "", vchlog.LogFilePrefix+"*"+vchlog.LogFileSuffix)
	if err != nil {
		return "", fmt.Errorf("Unable to list vic-machine log files in datastore: %s", err)
	}

	if len(res.File) == 0 {
		return "", util.NewError(http.StatusNotFound, "No log file available in datastore folder")
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
			return "", fmt.Errorf("Unable to download log file %s: %s", p, err)
		}

		if _, err := buffer.ReadFrom(reader); err != nil {
			return "", fmt.Errorf("Error reading from log file %s: %s", p, err)
		}
	}

	return string(buffer.Bytes()), nil
}
