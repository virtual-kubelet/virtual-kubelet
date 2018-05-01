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
	"fmt"
	"net/http"
	"net/url"

	"github.com/docker/docker/opts"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/vic/cmd/vic-machine/common"
	"github.com/vmware/vic/lib/apiservers/service/restapi/handlers/util"
	"github.com/vmware/vic/lib/config"
	"github.com/vmware/vic/lib/constants"
	"github.com/vmware/vic/lib/install/data"
	"github.com/vmware/vic/lib/install/management"
	"github.com/vmware/vic/lib/install/validate"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/version"
	"github.com/vmware/vic/pkg/vsphere/vm"
)

type buildDataParams struct {
	target          string
	thumbprint      *string
	datacenter      *string
	computeResource *string
	vchID           *string
}

func buildDataAndValidateTarget(op trace.Operation, params buildDataParams, principal interface{}) (*data.Data, *validate.Validator, error) {
	data := &data.Data{
		Target: &common.Target{
			URL: &url.URL{Host: params.target},
		},
	}

	if c, ok := principal.(Credentials); ok {
		data.Target.User = c.user
		data.Target.Password = &c.pass
	} else {
		data.Target.CloneTicket = principal.(Session).ticket
	}

	if err := data.HasCredentials(op); err != nil {
		return data, nil, util.NewError(http.StatusUnauthorized, "Invalid Credentials: %s", err)
	}

	if params.thumbprint != nil {
		data.Thumbprint = *params.thumbprint
	}

	if params.computeResource != nil {
		data.ComputeResourcePath = *params.computeResource
	}

	if params.vchID != nil {
		data.ID = *params.vchID
	}

	// TODO (#6032): clean this up
	var validator *validate.Validator
	if params.datacenter != nil {
		v, err := validate.NewValidator(op, data)
		if err != nil {
			return data, nil, util.NewError(http.StatusBadRequest, "Validation Error: %s", err)
		}

		datacenterManagedObjectReference := types.ManagedObjectReference{Type: "Datacenter", Value: *params.datacenter}

		datacenterObject, err := v.Session.Finder.ObjectReference(op, datacenterManagedObjectReference)
		if err != nil {
			return nil, nil, util.WrapError(http.StatusNotFound, err)
		}

		dc, ok := datacenterObject.(*object.Datacenter)
		if !ok {
			return data, nil, util.NewError(http.StatusBadRequest, "Validation Error: datacenter parameter is not a datacenter moref")
		}

		// Set validator datacenter path and correspondent validator session config
		v.DatacenterPath = dc.Name()
		v.Session.DatacenterPath = v.DatacenterPath
		v.Session.Datacenter = dc
		v.Session.Finder.SetDatacenter(dc)

		// Do what validator.session.Populate would have done if datacenterPath is set
		if v.Session.Datacenter != nil {
			folders, err := v.Session.Datacenter.Folders(op)
			if err != nil {
				return data, nil, util.NewError(http.StatusBadRequest, "Validation Error: error finding datacenter folders: %s", err)
			}
			v.Session.VMFolder = folders.VmFolder
		}

		validator = v
	} else {
		v, err := validate.NewValidator(op, data)
		if err != nil {
			return data, nil, util.NewError(http.StatusBadRequest, "Validation Error: %s", err)
		}

		// If dc is not set, and multiple datacenters are available, operate on all datacenters.
		v.AllowEmptyDC()

		validator = v
	}

	if _, err := validator.ValidateTarget(op, data); err != nil {
		return data, nil, util.NewError(http.StatusBadRequest, "Target validation failed: %s", err)
	}

	if _, err := validator.ValidateCompute(op, data, false); err != nil {
		return data, nil, util.NewError(http.StatusBadRequest, "Compute resource validation failed: %s", err)
	}

	return data, validator, nil
}

// Copied from list.go, and appears to be present other places. TODO (#6032): deduplicate
func upgradeStatusMessage(op trace.Operation, vch *vm.VirtualMachine, installerVer *version.Build, vchVer *version.Build) string {
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

func getVCHConfig(op trace.Operation, d *data.Data, validator *validate.Validator) (*config.VirtualContainerHostConfigSpec, error) {
	executor := management.NewDispatcher(validator.Context, validator.Session, nil, false)
	vch, err := executor.NewVCHFromID(d.ID)
	if err != nil {
		return nil, util.NewError(http.StatusNotFound, fmt.Sprintf("Unable to find VCH %s: %s", d.ID, err))
	}

	err = validate.SetDataFromVM(validator.Context, validator.Session.Finder, vch, d)
	if err != nil {
		return nil, util.NewError(http.StatusInternalServerError, fmt.Sprintf("Failed to load VCH data: %s", err))
	}

	vchConfig, err := executor.GetNoSecretVCHConfig(vch)
	if err != nil {
		return nil, fmt.Errorf("Unable to retrieve VCH information: %s", err)
	}

	return vchConfig, nil
}

func getAddresses(vchConfig *config.VirtualContainerHostConfigSpec) (dockerHost, adminPortal string) {
	if client := vchConfig.ExecutorConfig.Networks["client"]; client != nil {
		if publicIP := client.Assigned.IP; publicIP != nil {
			var dockerPort = opts.DefaultTLSHTTPPort
			if vchConfig.HostCertificate.IsNil() {
				dockerPort = opts.DefaultHTTPPort
			}

			dockerHost = fmt.Sprintf("%s:%d", publicIP, dockerPort)
			adminPortal = fmt.Sprintf("https://%s:%d", publicIP, constants.VchAdminPortalPort)
		}
	}

	return
}
