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
	"net"
	"net/http"
	"path"
	"strings"

	"github.com/go-openapi/runtime/middleware"
	"github.com/go-openapi/strfmt"
	"gopkg.in/urfave/cli.v1"

	"github.com/vmware/vic/cmd/vic-machine/common"
	"github.com/vmware/vic/cmd/vic-machine/create"
	"github.com/vmware/vic/lib/apiservers/service/models"
	"github.com/vmware/vic/lib/apiservers/service/restapi/handlers/client"
	"github.com/vmware/vic/lib/apiservers/service/restapi/handlers/decode"
	"github.com/vmware/vic/lib/apiservers/service/restapi/handlers/errors"
	"github.com/vmware/vic/lib/apiservers/service/restapi/handlers/logging"
	"github.com/vmware/vic/lib/apiservers/service/restapi/handlers/target"
	"github.com/vmware/vic/lib/apiservers/service/restapi/operations"
	"github.com/vmware/vic/lib/config/executor"
	"github.com/vmware/vic/lib/constants"
	"github.com/vmware/vic/lib/install/data"
	"github.com/vmware/vic/lib/install/management"
	"github.com/vmware/vic/lib/install/vchlog"
	"github.com/vmware/vic/pkg/ip"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/version"
)

const (
	logFile = "vic-machine.log" // name of local log file
)

// VCHCreate is the handler for creating a VCH without specifying a datacenter
type VCHCreate struct {
	vchCreate
}

// VCHDatacenterCreate is the handler for creating a VCH within a specified datacenter
type VCHDatacenterCreate struct {
	vchCreate
}

// vchCreate allows for VCHCreate and VCHDatacenterCreate to share common code without polluting the package
type vchCreate struct{}

// Handle is the handler implementation for creating a VCH without specifying a datacenter
func (h *VCHCreate) Handle(params operations.PostTargetTargetVchParams, principal interface{}) middleware.Responder {
	op := trace.FromContext(params.HTTPRequest.Context(), "VCHCreate")

	b := target.Params{
		Target:     params.Target,
		Thumbprint: params.Thumbprint,
	}

	task, err := h.handle(op, b, principal, params.Vch)
	if err != nil {
		return operations.NewPostTargetTargetVchDefault(errors.StatusCode(err)).WithPayload(&models.Error{Message: err.Error()})
	}

	return operations.NewPostTargetTargetVchCreated().WithPayload(operations.PostTargetTargetVchCreatedBody{Task: task})
}

// Handle is the handler implementation for creating a VCH within a specified datacenter
func (h *VCHDatacenterCreate) Handle(params operations.PostTargetTargetDatacenterDatacenterVchParams, principal interface{}) middleware.Responder {
	op := trace.FromContext(params.HTTPRequest.Context(), "VCHDatacenterCreate")

	b := target.Params{
		Target:     params.Target,
		Thumbprint: params.Thumbprint,
		Datacenter: &params.Datacenter,
	}

	task, err := h.handle(op, b, principal, params.Vch)
	if err != nil {
		return operations.NewPostTargetTargetDatacenterDatacenterVchDefault(errors.StatusCode(err)).WithPayload(&models.Error{Message: err.Error()})
	}

	return operations.NewPostTargetTargetDatacenterDatacenterVchCreated().WithPayload(operations.PostTargetTargetDatacenterDatacenterVchCreatedBody{Task: task})
}

// handle creates a VCH with the settings from vch at the target described by params, using the credentials from
// principal. If an error occurs validating the requested settings, a 400 is returned. If an error occurs during
// creation, a 500 is returned. Currently, no task is ever returned.
func (h *vchCreate) handle(op trace.Operation, params target.Params, principal interface{}, vch *models.VCH) (*strfmt.URI, error) {
	datastoreLogger := logging.SetUpLogger(&op)
	defer datastoreLogger.Close()

	d, hc, err := target.Validate(op, management.ActionCreate, params, principal)
	if err != nil {
		return nil, err
	}

	c, err := h.buildCreate(op, d, hc.Finder(), vch)
	if err != nil {
		return nil, err
	}

	return h.handleCreate(op, c, hc, datastoreLogger)
}

func (h *vchCreate) buildCreate(op trace.Operation, d *data.Data, finder client.Finder, vch *models.VCH) (*create.Create, error) {
	c := &create.Create{Data: d}

	// TODO (#6032): deduplicate with create.processParams

	if vch != nil {
		if vch.Version != "" && version.String() != string(vch.Version) {
			return nil, errors.NewError(http.StatusBadRequest, "invalid version: %s", vch.Version)
		}

		c.DisplayName = vch.Name

		// TODO (#6710): move validation to swagger
		if err := common.CheckUnsupportedChars(c.DisplayName); err != nil {
			return nil, errors.NewError(http.StatusBadRequest, "invalid display name: %s", err)
		}
		if len(c.DisplayName) > create.MaxDisplayNameLen {
			return nil, errors.NewError(http.StatusBadRequest, "invalid display name: length exceeds %d characters", create.MaxDisplayNameLen)
		}

		debug := int(vch.Debug)
		c.Debug.Debug = &debug

		if vch.Compute != nil {
			if vch.Compute.CPU != nil {
				c.ResourceLimits.VCHCPULimitsMHz = decode.MHzFromValueHertz(vch.Compute.CPU.Limit)
				c.ResourceLimits.VCHCPUReservationsMHz = decode.MHzFromValueHertz(vch.Compute.CPU.Reservation)
				c.ResourceLimits.VCHCPUShares = decode.FromShares(vch.Compute.CPU.Shares)
			}

			if vch.Compute.Memory != nil {
				c.ResourceLimits.VCHMemoryLimitsMB = decode.MBFromValueBytes(vch.Compute.Memory.Limit)
				c.ResourceLimits.VCHMemoryReservationsMB = decode.MBFromValueBytes(vch.Compute.Memory.Reservation)
				c.ResourceLimits.VCHMemoryShares = decode.FromShares(vch.Compute.Memory.Shares)
			}

			resourcePath, err := decode.FromManagedObject(op, finder, "ResourcePool", vch.Compute.Resource) // TODO (#6711): Do we need to handle clusters differently?
			if err != nil {
				return nil, errors.NewError(http.StatusBadRequest, "error finding resource pool: %s", err)
			}
			if resourcePath == "" {
				return nil, errors.NewError(http.StatusBadRequest, "resource pool must be specified (by name or id)")
			}
			c.ComputeResourcePath = resourcePath

			if vch.Compute.Affinity != nil {
				c.UseVMGroup = vch.Compute.Affinity.UseVMGroup
			}
		}

		if vch.Network != nil {
			if vch.Network.Bridge != nil {
				path, err := decode.FromManagedObject(op, finder, "Network", vch.Network.Bridge.PortGroup)
				if err != nil {
					return nil, errors.NewError(http.StatusBadRequest, "error finding bridge network: %s", err)
				}
				if path == "" {
					return nil, errors.NewError(http.StatusBadRequest, "bridge network portgroup must be specified (by name or id)")
				}
				c.BridgeNetworkName = path
				c.BridgeIPRange = decode.FromCIDR(&vch.Network.Bridge.IPRange)

				if err := c.ProcessBridgeNetwork(); err != nil {
					return nil, errors.WrapError(http.StatusBadRequest, err)
				}
			}

			if vch.Network.Client != nil {
				path, err := decode.FromManagedObject(op, finder, "Network", vch.Network.Client.PortGroup)
				if err != nil {
					return nil, errors.NewError(http.StatusBadRequest, "error finding client network portgroup: %s", err)
				}
				if path == "" {
					return nil, errors.NewError(http.StatusBadRequest, "client network portgroup must be specified (by name or id)")
				}
				c.ClientNetworkName = path
				c.ClientNetworkGateway = decode.FromGateway(vch.Network.Client.Gateway)
				c.ClientNetworkIP = decode.FromCIDR(&vch.Network.Client.Static)

				if err := c.ProcessNetwork(op, &c.Data.ClientNetwork, "client", c.ClientNetworkName, c.ClientNetworkIP, c.ClientNetworkGateway); err != nil {
					return nil, errors.WrapError(http.StatusBadRequest, err)
				}
			}

			if vch.Network.Management != nil {
				path, err := decode.FromManagedObject(op, finder, "Network", vch.Network.Management.PortGroup)
				if err != nil {
					return nil, errors.NewError(http.StatusBadRequest, "error finding management network portgroup: %s", err)
				}
				if path == "" {
					return nil, errors.NewError(http.StatusBadRequest, "management network portgroup must be specified (by name or id)")
				}
				c.ManagementNetworkName = path
				c.ManagementNetworkGateway = decode.FromGateway(vch.Network.Management.Gateway)
				c.ManagementNetworkIP = decode.FromCIDR(&vch.Network.Management.Static)

				if err := c.ProcessNetwork(op, &c.Data.ManagementNetwork, "management", c.ManagementNetworkName, c.ManagementNetworkIP, c.ManagementNetworkGateway); err != nil {
					return nil, errors.WrapError(http.StatusBadRequest, err)
				}
			}

			if vch.Network.Public != nil {
				path, err := decode.FromManagedObject(op, finder, "Network", vch.Network.Public.PortGroup)
				if err != nil {
					return nil, errors.NewError(http.StatusBadRequest, "error finding public network portgroup: %s", err)
				}
				if path == "" {
					return nil, errors.NewError(http.StatusBadRequest, "public network portgroup must be specified (by name or id)")
				}
				c.PublicNetworkName = path
				c.PublicNetworkGateway = decode.FromGateway(vch.Network.Public.Gateway)
				c.PublicNetworkIP = decode.FromCIDR(&vch.Network.Public.Static)

				if err := c.ProcessNetwork(op, &c.Data.PublicNetwork, "public", c.PublicNetworkName, c.PublicNetworkIP, c.PublicNetworkGateway); err != nil {
					return nil, errors.WrapError(http.StatusBadRequest, err)
				}

				c.Nameservers = common.DNS{
					DNS: decode.FromIPAddresses(vch.Network.Public.Nameservers),
				}
				c.DNS, err = c.Nameservers.ProcessDNSServers(op)
				if err != nil {
					return nil, errors.WrapError(http.StatusBadRequest, err)
				}
			}

			if vch.Network.Container != nil {
				containerNetworks := common.ContainerNetworks{
					MappedNetworks:          make(map[string]string),
					MappedNetworksGateways:  make(map[string]net.IPNet),
					MappedNetworksIPRanges:  make(map[string][]ip.Range),
					MappedNetworksDNS:       make(map[string][]net.IP),
					MappedNetworksFirewalls: make(map[string]executor.TrustLevel),
				}

				for _, cnetwork := range vch.Network.Container {
					alias := cnetwork.Alias

					path, err := decode.FromManagedObject(op, finder, "Network", cnetwork.PortGroup)
					if err != nil {
						return nil, errors.NewError(http.StatusBadRequest, "error finding portgroup for container network %s: %s", alias, err)
					}
					if path == "" {
						return nil, errors.NewError(http.StatusBadRequest, "container network %s portgroup must be specified (by name or id)", alias)
					}
					containerNetworks.MappedNetworks[alias] = path

					if cnetwork.Gateway != nil {
						address := net.ParseIP(string(cnetwork.Gateway.Address))
						if address == nil {
							return nil, errors.NewError(http.StatusBadRequest, "error parsing gateway IP %s for container network %s", cnetwork.Gateway.Address, alias)
						}
						if cnetwork.Gateway.RoutingDestinations == nil || len(cnetwork.Gateway.RoutingDestinations) != 1 {
							return nil, errors.NewError(http.StatusBadRequest, "error parsing network mask for container network %s: exactly one subnet must be specified", alias)
						}
						_, mask, err := net.ParseCIDR(string(cnetwork.Gateway.RoutingDestinations[0]))
						if err != nil {
							return nil, errors.NewError(http.StatusBadRequest, "error parsing network mask for container network %s: %s", alias, err)
						}
						containerNetworks.MappedNetworksGateways[alias] = net.IPNet{
							IP:   address,
							Mask: mask.Mask,
						}
					}

					ipranges := make([]ip.Range, 0, len(cnetwork.IPRanges))
					for _, ipRange := range cnetwork.IPRanges {
						r := ip.ParseRange(string(ipRange))

						ipranges = append(ipranges, *r)
					}
					containerNetworks.MappedNetworksIPRanges[alias] = ipranges

					nameservers := make([]net.IP, 0, len(cnetwork.Nameservers))
					for _, nameserver := range cnetwork.Nameservers {
						n := net.ParseIP(string(nameserver))
						nameservers = append(nameservers, n)
					}
					containerNetworks.MappedNetworksDNS[alias] = nameservers

					if cnetwork.Firewall != "" {
						trustLevel, err := executor.ParseTrustLevel(cnetwork.Firewall)
						if err != nil {
							return nil, errors.NewError(http.StatusBadRequest, "error parsing trust level for container network %s: %s", alias, err)
						}

						containerNetworks.MappedNetworksFirewalls[alias] = trustLevel
					}
				}

				c.ContainerNetworks = containerNetworks
			}
		}

		if vch.Storage != nil {
			if vch.Storage.ImageStores != nil && len(vch.Storage.ImageStores) > 0 {
				c.ImageDatastorePath = vch.Storage.ImageStores[0] // TODO (#6712): many vs. one mismatch
			}

			if err := common.CheckUnsupportedCharsDatastore(c.ImageDatastorePath); err != nil {
				return nil, errors.WrapError(http.StatusBadRequest, err)
			}

			if vch.Storage.VolumeStores != nil {
				volumes := make([]string, 0, len(vch.Storage.VolumeStores))
				for _, v := range vch.Storage.VolumeStores {
					volumes = append(volumes, fmt.Sprintf("%s:%s", v.Datastore, v.Label))
				}

				vs := common.VolumeStores{VolumeStores: cli.StringSlice(volumes)}
				volumeLocations, err := vs.ProcessVolumeStores()
				if err != nil {
					return nil, errors.NewError(http.StatusBadRequest, "error processing volume stores: %s", err)
				}
				c.VolumeLocations = volumeLocations
			}

			c.ScratchSize = constants.DefaultBaseImageScratchSize
			if vch.Storage.BaseImageSize != nil {
				c.ScratchSize = decode.FromValueBytesMetric(vch.Storage.BaseImageSize)
			}
		}

		if vch.Auth != nil {
			c.Certs.NoTLS = vch.Auth.NoTLS

			if vch.Auth.Client != nil {
				c.Certs.NoTLSverify = vch.Auth.Client.NoTLSVerify
				c.Certs.ClientCAs = decode.FromPemCertificates(vch.Auth.Client.CertificateAuthorities)
				c.ClientCAs = c.Certs.ClientCAs
			}

			if vch.Auth.Server != nil {

				if vch.Auth.Server.Generate != nil {
					c.Certs.Cname = vch.Auth.Server.Generate.Cname
					c.Certs.Org = vch.Auth.Server.Generate.Organization
					c.Certs.KeySize = decode.FromValueBits(vch.Auth.Server.Generate.Size)

					c.Certs.NoSaveToDisk = true
					c.Certs.Networks = c.Networks
					if err := c.Certs.ProcessCertificates(op, c.DisplayName, c.Force, 0); err != nil {
						return nil, errors.NewError(http.StatusBadRequest, "error generating certificates: %s", err)
					}
				} else {
					c.Certs.CertPEM = []byte(vch.Auth.Server.Certificate.Pem)
					c.Certs.KeyPEM = []byte(vch.Auth.Server.PrivateKey.Pem)
				}

				c.KeyPEM = c.Certs.KeyPEM
				c.CertPEM = c.Certs.CertPEM
				c.ClientCAs = c.Certs.ClientCAs
			}
		}

		c.MemoryMB = constants.DefaultEndpointMemoryMB
		if vch.Endpoint != nil {
			if vch.Endpoint.Memory != nil {
				c.MemoryMB = *decode.MBFromValueBytes(vch.Endpoint.Memory)
			}
			if vch.Endpoint.CPU != nil {
				c.NumCPUs = int(vch.Endpoint.CPU.Sockets)
			}

			if vch.Endpoint.OperationsCredentials != nil {
				opsPassword := string(vch.Endpoint.OperationsCredentials.Password)
				c.OpsCredentials = common.OpsCredentials{
					OpsUser:     &vch.Endpoint.OperationsCredentials.User,
					OpsPassword: &opsPassword,
					GrantPerms:  &vch.Endpoint.OperationsCredentials.GrantPermissions,
				}
			}
		}
		if err := c.OpsCredentials.ProcessOpsCredentials(op, true, c.Target.User, c.Target.Password); err != nil {
			return nil, errors.WrapError(http.StatusBadRequest, err)
		}

		if vch.Registry != nil {
			c.InsecureRegistries = vch.Registry.Insecure
			c.WhitelistRegistries = vch.Registry.Whitelist

			c.RegistryCAs = decode.FromPemCertificates(vch.Registry.CertificateAuthorities)

			if vch.Registry.ImageFetchProxy != nil {
				c.Proxies = decode.FromImageFetchProxy(vch.Registry.ImageFetchProxy)

				hproxy, sproxy, err := c.Proxies.ProcessProxies()
				if err != nil {
					return nil, errors.NewError(http.StatusBadRequest, "error processing proxies: %s", err)
				}
				c.HTTPProxy = hproxy
				c.HTTPSProxy = sproxy
			}
		}

		if vch.SyslogAddr != "" {
			c.SyslogAddr = vch.SyslogAddr.String()
			if err := c.ProcessSyslog(); err != nil {
				return nil, errors.NewError(http.StatusBadRequest, "error processing syslog server address: %s", err)
			}
		}

		if vch.Container != nil && vch.Container.NameConvention != "" {
			c.ContainerNameConvention = vch.Container.NameConvention
		}
	}

	return c, nil
}

func (h *vchCreate) handleCreate(op trace.Operation, c *create.Create, hc *client.HandlerClient, receiver vchlog.Receiver) (*strfmt.URI, error) {
	validator := hc.Validator() // TODO (#6032): Move some of the logic that uses this into methods on hc

	vchConfig, err := validator.Validate(op, c.Data, false)
	if err != nil {
		issues := validator.GetIssues()
		messages := make([]string, 0, len(issues))
		for _, issue := range issues {
			messages = append(messages, issue.Error())
		}

		return nil, errors.NewError(http.StatusBadRequest, "failed to validate VCH: %s", strings.Join(messages, ", "))
	}

	vConfig := validator.AddDeprecatedFields(op, vchConfig, c.Data)

	// TODO (#6714): make this configurable
	images := common.Images{}
	vConfig.ImageFiles, err = images.CheckImagesFiles(op, true)
	vConfig.ApplianceISO = path.Base(images.ApplianceISO)
	vConfig.BootstrapISO = path.Base(images.BootstrapISO)

	vConfig.HTTPProxy = c.HTTPProxy
	vConfig.HTTPSProxy = c.HTTPSProxy

	err = hc.Executor().CreateVCH(vchConfig, vConfig, receiver)
	if err != nil {
		return nil, errors.NewError(http.StatusInternalServerError, "failed to create VCH: %s", err)
	}

	return nil, nil
}
