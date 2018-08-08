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
	"github.com/go-openapi/strfmt"

	"github.com/vmware/govmomi/object"

	"github.com/vmware/vic/lib/apiservers/service/models"
	"github.com/vmware/vic/lib/apiservers/service/restapi/handlers/client"
	"github.com/vmware/vic/lib/apiservers/service/restapi/handlers/encode"
	"github.com/vmware/vic/lib/apiservers/service/restapi/handlers/errors"
	"github.com/vmware/vic/lib/apiservers/service/restapi/handlers/target"
	"github.com/vmware/vic/lib/apiservers/service/restapi/operations"
	"github.com/vmware/vic/lib/config"
	"github.com/vmware/vic/lib/install/data"
	"github.com/vmware/vic/lib/install/interaction"
	"github.com/vmware/vic/lib/install/management"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/version"
	"github.com/vmware/vic/pkg/vsphere/vm"
)

// VCHGet is the handler for inspecting a VCH without specifying a datacenter
type VCHGet struct {
	vchGet
}

// VCHDatacenterGet is the handler for inspecting a VCH within a specified datacenter
type VCHDatacenterGet struct {
	vchGet
}

// vchGet allows for VCHGet and VCHDatacenterGet to share common code without polluting the package
type vchGet struct{}

// Handle is the handler implementation for inspecting a VCH without specifying a datacenter
func (h *VCHGet) Handle(params operations.GetTargetTargetVchVchIDParams, principal interface{}) middleware.Responder {
	op := trace.FromContext(params.HTTPRequest.Context(), "VCHGet: %s", params.VchID)

	b := target.Params{
		Target:     params.Target,
		Thumbprint: params.Thumbprint,
		VCHID:      &params.VchID,
	}

	vch, err := h.handle(op, b, principal)
	if err != nil {
		return operations.NewGetTargetTargetVchVchIDDefault(errors.StatusCode(err)).WithPayload(&models.Error{Message: err.Error()})
	}

	return operations.NewGetTargetTargetVchVchIDOK().WithPayload(vch)
}

// Handle is the handler implementation for inspecting a VCH within a specified datacenter
func (h *VCHDatacenterGet) Handle(params operations.GetTargetTargetDatacenterDatacenterVchVchIDParams, principal interface{}) middleware.Responder {
	op := trace.FromContext(params.HTTPRequest.Context(), "VCHDatacenterGet: %s", params.VchID)

	b := target.Params{
		Target:     params.Target,
		Thumbprint: params.Thumbprint,
		Datacenter: &params.Datacenter,
		VCHID:      &params.VchID,
	}

	vch, err := h.handle(op, b, principal)
	if err != nil {
		return operations.NewGetTargetTargetDatacenterDatacenterVchVchIDDefault(errors.StatusCode(err)).WithPayload(&models.Error{Message: err.Error()})
	}

	return operations.NewGetTargetTargetDatacenterDatacenterVchVchIDOK().WithPayload(vch)
}

// handle inspects the VCH described by params using the credentials from principal. If the VCH cannot be found, a 404
// is returned. If another error occurs, a 500 is returned.
func (h *vchGet) handle(op trace.Operation, params target.Params, principal interface{}) (*models.VCH, error) {
	d, c, err := target.Validate(op, management.ActionInspect, params, principal)
	if err != nil {
		return nil, err
	}

	vch, err := c.GetVCH(op, d)
	if err != nil {
		return nil, err
	}

	vchConfig, err := c.GetConfigForVCH(op, vch)
	if err != nil {
		return nil, err
	}

	return h.vchToModel(op, d, vch, vchConfig, c), nil
}

func (h *vchGet) vchToModel(op trace.Operation, d *data.Data, vch *vm.VirtualMachine, vchConfig *config.VirtualContainerHostConfigSpec, c *client.HandlerClient) *models.VCH {
	model := &models.VCH{}
	model.Version = models.Version(vchConfig.Version.ShortVersion())
	model.Name = vchConfig.Name
	model.Debug = int64(vchConfig.Diagnostics.DebugLevel)

	// compute
	model.Compute = &models.VCHCompute{
		Affinity: &models.VCHComputeAffinity{
			UseVMGroup: vchConfig.UseVMGroup,
		},
		CPU: &models.VCHComputeCPU{
			Limit:       encode.AsMHz(d.ResourceLimits.VCHCPULimitsMHz),
			Reservation: encode.AsMHz(d.ResourceLimits.VCHCPUReservationsMHz),
			Shares:      encode.AsShares(d.ResourceLimits.VCHCPUShares),
		},
		Memory: &models.VCHComputeMemory{
			Limit:       encode.AsMiB(d.ResourceLimits.VCHMemoryLimitsMB),
			Reservation: encode.AsMiB(d.ResourceLimits.VCHMemoryReservationsMB),
			Shares:      encode.AsShares(d.ResourceLimits.VCHMemoryShares),
		},
		Resource: &models.ManagedObject{
			ID: encode.AsManagedObjectID(vchConfig.Container.ComputeResources[0].String()),
		},
	}

	// network
	model.Network = &models.VCHNetwork{
		Bridge: &models.VCHNetworkBridge{
			PortGroup: &models.ManagedObject{
				ID: encode.AsManagedObjectID(vchConfig.ExecutorConfig.Networks[vchConfig.Network.BridgeNetwork].Network.Common.ID),
			},
			IPRange: encode.AsCIDR(vchConfig.Network.BridgeIPRange),
		},
		Client:     encode.AsNetwork(vchConfig.ExecutorConfig.Networks["client"]),
		Management: encode.AsNetwork(vchConfig.ExecutorConfig.Networks["management"]),
		Public:     encode.AsNetwork(vchConfig.ExecutorConfig.Networks["public"]),
	}

	containerNetworks := make([]*models.ContainerNetwork, 0, len(vchConfig.Network.ContainerNetworks))
	for key, value := range vchConfig.Network.ContainerNetworks {
		if key != "bridge" {
			containerNetworks = append(containerNetworks, &models.ContainerNetwork{
				Alias: value.Name,
				PortGroup: &models.ManagedObject{
					ID: encode.AsManagedObjectID(value.Common.ID),
				},
				Nameservers: *encode.AsIPAddresses(&value.Nameservers),
				Gateway: &models.Gateway{
					Address:             encode.AsIPAddress(value.Gateway.IP),
					RoutingDestinations: []models.CIDR{encode.AsCIDR(&value.Gateway)},
				},
				IPRanges: *encode.AsIPRanges(&value.Pools),
				Firewall: value.TrustLevel.String(),
			})
		}
	}
	model.Network.Container = containerNetworks

	// storage
	scratchSize := int(vchConfig.Storage.ScratchSize)
	model.Storage = &models.VCHStorage{
		BaseImageSize: encode.AsKB(&scratchSize),
	}

	volumeLocations := make([]*models.VCHStorageVolumeStoresItems0, 0, len(vchConfig.Storage.VolumeLocations))
	for label, path := range vchConfig.Storage.VolumeLocations {
		parsedPath := object.DatastorePath{}
		parsed := parsedPath.FromString(path.Path)
		if parsed {
			path.Path = parsedPath.Path
		}

		volume := models.VCHStorageVolumeStoresItems0{Datastore: path.String(), Label: label}
		volumeLocations = append(volumeLocations, &volume)
	}
	model.Storage.VolumeStores = volumeLocations

	imageStores := make([]string, 0, len(vchConfig.Storage.ImageStores))
	for _, value := range vchConfig.Storage.ImageStores {
		imageStores = append(imageStores, value.String())
	}
	model.Storage.ImageStores = imageStores

	// auth
	model.Auth = &models.VCHAuth{
		Client: &models.VCHAuthClient{},
	}

	if vchConfig.Certificate.HostCertificate != nil {
		model.Auth.Server = &models.VCHAuthServer{
			Certificate: encode.AsPemCertificate(vchConfig.Certificate.HostCertificate.Cert),
		}
	}

	model.Auth.Client.CertificateAuthorities = encode.AsPemCertificates(vchConfig.Certificate.CertificateAuthorities)

	// endpoint
	model.Endpoint = &models.VCHEndpoint{
		Memory: encode.AsMiB(&d.MemoryMB),
		CPU: &models.VCHEndpointCPU{
			Sockets: int64(d.NumCPUs),
		},
		OperationsCredentials: &models.VCHEndpointOperationsCredentials{
			User: vchConfig.Connection.Username,
			// Password intentionally excluded from GET responses for security reasons!
		},
	}

	// registry
	model.Registry = &models.VCHRegistry{
		Insecure:               vchConfig.Registry.InsecureRegistries,
		Whitelist:              vchConfig.Registry.RegistryWhitelist,
		CertificateAuthorities: encode.AsPemCertificates(vchConfig.Certificate.RegistryCertificateAuthorities),
		ImageFetchProxy:        encode.AsImageFetchProxy(vchConfig.ExecutorConfig.Sessions[config.VicAdminService], config.VICAdminHTTPProxy, config.VICAdminHTTPSProxy),
	}

	// runtime
	model.Runtime = &models.VCHRuntime{}

	installerVer := version.GetBuild()
	upgradeStatus := interaction.GetUpgradeStatusShortMessage(op, vch, installerVer, vchConfig.Version)
	model.Runtime.UpgradeStatus = upgradeStatus

	powerState, err := vch.PowerState(op)
	if err != nil {
		powerState = "error"
	}
	model.Runtime.PowerState = string(powerState)

	model.Runtime.DockerHost, model.Runtime.AdminPortal, err = c.GetAddresses(vchConfig)
	if err != nil {
		op.Warn("Failed to get docker host and admin portal address: %s", err)
	}

	// syslog_addr: syslog server address
	if syslogConfig := vchConfig.Diagnostics.SysLogConfig; syslogConfig != nil {
		model.SyslogAddr = strfmt.URI(syslogConfig.Network + "://" + syslogConfig.RAddr)
	}

	model.Container = &models.VCHContainer{}
	if vchConfig.ContainerNameConvention != "" {
		model.Container.NameConvention = vchConfig.ContainerNameConvention
	}

	return model
}
