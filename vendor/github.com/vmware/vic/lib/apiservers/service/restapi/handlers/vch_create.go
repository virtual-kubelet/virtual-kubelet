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
	"context"
	"fmt"
	"math"
	"net"
	"net/http"
	"path"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/docker/go-units"
	"github.com/go-openapi/runtime/middleware"
	"github.com/go-openapi/strfmt"
	"gopkg.in/urfave/cli.v1"

	"github.com/vmware/govmomi/list"
	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/vic/cmd/vic-machine/common"
	"github.com/vmware/vic/cmd/vic-machine/create"
	"github.com/vmware/vic/lib/apiservers/service/models"
	"github.com/vmware/vic/lib/apiservers/service/restapi/handlers/util"
	"github.com/vmware/vic/lib/apiservers/service/restapi/operations"
	"github.com/vmware/vic/lib/config/executor"
	"github.com/vmware/vic/lib/constants"
	"github.com/vmware/vic/lib/install/data"
	"github.com/vmware/vic/lib/install/management"
	"github.com/vmware/vic/lib/install/validate"
	"github.com/vmware/vic/lib/install/vchlog"
	"github.com/vmware/vic/pkg/ip"
	viclog "github.com/vmware/vic/pkg/log"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/version"
)

const (
	logFile = "vic-machine.log" // name of local log file
)

// This interface is declared so that we can enable mocking finder in tests
// as the govmomi types do not use interfaces themselves.
type finder interface {
	Element(context.Context, types.ManagedObjectReference) (*list.Element, error)
}

// VCHCreate is the handler for creating a VCH
type VCHCreate struct {
}

// VCHDatacenterCreate is the handler for creating a VCH within a Datacenter
type VCHDatacenterCreate struct {
}

// Handle is the handler implementation for VCH creation without a datacenter
func (h *VCHCreate) Handle(params operations.PostTargetTargetVchParams, principal interface{}) middleware.Responder {
	op := trace.FromContext(params.HTTPRequest.Context(), "VCHCreate")

	datastoreLogger := setUpLogger(&op)
	defer datastoreLogger.Close()

	b := buildDataParams{
		target:     params.Target,
		thumbprint: params.Thumbprint,
	}

	d, validator, err := buildDataAndValidateTarget(op, b, principal)
	if err != nil {
		return operations.NewPostTargetTargetVchDefault(util.StatusCode(err)).WithPayload(&models.Error{Message: err.Error()})
	}

	c, err := buildCreate(op, d, finder(validator.Session.Finder), params.Vch)
	if err != nil {
		return operations.NewPostTargetTargetVchDefault(util.StatusCode(err)).WithPayload(&models.Error{Message: err.Error()})
	}

	task, err := handleCreate(op, c, validator, datastoreLogger)
	if err != nil {
		return operations.NewPostTargetTargetVchDefault(util.StatusCode(err)).WithPayload(&models.Error{Message: err.Error()})
	}

	return operations.NewPostTargetTargetVchCreated().WithPayload(operations.PostTargetTargetVchCreatedBody{Task: task})
}

// Handle is the handler implementation for VCH creation with a datacenter
func (h *VCHDatacenterCreate) Handle(params operations.PostTargetTargetDatacenterDatacenterVchParams, principal interface{}) middleware.Responder {
	op := trace.FromContext(params.HTTPRequest.Context(), "VCHDatacenterCreate")

	datastoreLogger := setUpLogger(&op)
	defer datastoreLogger.Close()

	b := buildDataParams{
		target:     params.Target,
		thumbprint: params.Thumbprint,
		datacenter: &params.Datacenter,
	}

	d, validator, err := buildDataAndValidateTarget(op, b, principal)
	if err != nil {
		return operations.NewPostTargetTargetDatacenterDatacenterVchDefault(util.StatusCode(err)).WithPayload(&models.Error{Message: err.Error()})
	}

	c, err := buildCreate(op, d, validator.Session.Finder, params.Vch)
	if err != nil {
		return operations.NewPostTargetTargetDatacenterDatacenterVchDefault(util.StatusCode(err)).WithPayload(&models.Error{Message: err.Error()})
	}

	task, err := handleCreate(op, c, validator, datastoreLogger)
	if err != nil {
		return operations.NewPostTargetTargetDatacenterDatacenterVchDefault(util.StatusCode(err)).WithPayload(&models.Error{Message: err.Error()})
	}

	return operations.NewPostTargetTargetDatacenterDatacenterVchCreated().WithPayload(operations.PostTargetTargetDatacenterDatacenterVchCreatedBody{Task: task})
}

func setUpLogger(op *trace.Operation) *vchlog.VCHLogger {
	log := vchlog.New()

	op.Logger = logrus.New()
	op.Logger.Out = log.GetPipe()
	op.Logger.Level = logrus.DebugLevel
	op.Logger.Formatter = viclog.NewTextFormatter()

	op.Logger.Infof("Starting API-based VCH Creation. Version: %q", version.GetBuild().ShortVersion())

	go log.Run()

	return log
}

func buildCreate(op trace.Operation, d *data.Data, finder finder, vch *models.VCH) (*create.Create, error) {
	c := &create.Create{Data: d}

	// TODO (#6032): deduplicate with create.processParams

	if vch != nil {
		if vch.Version != "" && version.String() != string(vch.Version) {
			return nil, util.NewError(http.StatusBadRequest, fmt.Sprintf("Invalid version: %s", vch.Version))
		}

		c.DisplayName = vch.Name

		// TODO (#6710): move validation to swagger
		if err := common.CheckUnsupportedChars(c.DisplayName); err != nil {
			return nil, util.NewError(http.StatusBadRequest, fmt.Sprintf("Invalid display name: %s", err))
		}
		if len(c.DisplayName) > create.MaxDisplayNameLen {
			return nil, util.NewError(http.StatusBadRequest, fmt.Sprintf("Invalid display name: length exceeds %d characters", create.MaxDisplayNameLen))
		}

		debug := int(vch.Debug)
		c.Debug.Debug = &debug

		if vch.Compute != nil {
			if vch.Compute.CPU != nil {
				c.ResourceLimits.VCHCPULimitsMHz = mhzFromValueHertz(vch.Compute.CPU.Limit)
				c.ResourceLimits.VCHCPUReservationsMHz = mhzFromValueHertz(vch.Compute.CPU.Reservation)
				c.ResourceLimits.VCHCPUShares = fromShares(vch.Compute.CPU.Shares)
			}

			if vch.Compute.Memory != nil {
				c.ResourceLimits.VCHMemoryLimitsMB = mbFromValueBytes(vch.Compute.Memory.Limit)
				c.ResourceLimits.VCHMemoryReservationsMB = mbFromValueBytes(vch.Compute.Memory.Reservation)
				c.ResourceLimits.VCHMemoryShares = fromShares(vch.Compute.Memory.Shares)
			}

			resourcePath, err := fromManagedObject(op, finder, "ResourcePool", vch.Compute.Resource) // TODO (#6711): Do we need to handle clusters differently?
			if err != nil {
				return nil, util.NewError(http.StatusBadRequest, fmt.Sprintf("Error finding resource pool: %s", err))
			}
			if resourcePath == "" {
				return nil, util.NewError(http.StatusBadRequest, "Resource pool must be specified (by name or id)")
			}
			c.ComputeResourcePath = resourcePath
		}

		if vch.Network != nil {
			if vch.Network.Bridge != nil {
				path, err := fromManagedObject(op, finder, "Network", vch.Network.Bridge.PortGroup)
				if err != nil {
					return nil, util.NewError(http.StatusBadRequest, fmt.Sprintf("Error finding bridge network: %s", err))
				}
				if path == "" {
					return nil, util.NewError(http.StatusBadRequest, "Bridge network portgroup must be specified (by name or id)")
				}
				c.BridgeNetworkName = path
				c.BridgeIPRange = fromCIDR(&vch.Network.Bridge.IPRange)

				if err := c.ProcessBridgeNetwork(); err != nil {
					return nil, util.WrapError(http.StatusBadRequest, err)
				}
			}

			if vch.Network.Client != nil {
				path, err := fromManagedObject(op, finder, "Network", vch.Network.Client.PortGroup)
				if err != nil {
					return nil, util.NewError(http.StatusBadRequest, fmt.Sprintf("Error finding client network portgroup: %s", err))
				}
				if path == "" {
					return nil, util.NewError(http.StatusBadRequest, "Client network portgroup must be specified (by name or id)")
				}
				c.ClientNetworkName = path
				c.ClientNetworkGateway = fromGateway(vch.Network.Client.Gateway)
				c.ClientNetworkIP = fromCIDR(&vch.Network.Client.Static)

				if err := c.ProcessNetwork(op, &c.Data.ClientNetwork, "client", c.ClientNetworkName, c.ClientNetworkIP, c.ClientNetworkGateway); err != nil {
					return nil, util.WrapError(http.StatusBadRequest, err)
				}
			}

			if vch.Network.Management != nil {
				path, err := fromManagedObject(op, finder, "Network", vch.Network.Management.PortGroup)
				if err != nil {
					return nil, util.NewError(http.StatusBadRequest, fmt.Sprintf("Error finding management network portgroup: %s", err))
				}
				if path == "" {
					return nil, util.NewError(http.StatusBadRequest, "Management network portgroup must be specified (by name or id)")
				}
				c.ManagementNetworkName = path
				c.ManagementNetworkGateway = fromGateway(vch.Network.Management.Gateway)
				c.ManagementNetworkIP = fromCIDR(&vch.Network.Management.Static)

				if err := c.ProcessNetwork(op, &c.Data.ManagementNetwork, "management", c.ManagementNetworkName, c.ManagementNetworkIP, c.ManagementNetworkGateway); err != nil {
					return nil, util.WrapError(http.StatusBadRequest, err)
				}
			}

			if vch.Network.Public != nil {
				path, err := fromManagedObject(op, finder, "Network", vch.Network.Public.PortGroup)
				if err != nil {
					return nil, util.NewError(http.StatusBadRequest, fmt.Sprintf("Error finding public network portgroup: %s", err))
				}
				if path == "" {
					return nil, util.NewError(http.StatusBadRequest, "Public network portgroup must be specified (by name or id)")
				}
				c.PublicNetworkName = path
				c.PublicNetworkGateway = fromGateway(vch.Network.Public.Gateway)
				c.PublicNetworkIP = fromCIDR(&vch.Network.Public.Static)

				if err := c.ProcessNetwork(op, &c.Data.PublicNetwork, "public", c.PublicNetworkName, c.PublicNetworkIP, c.PublicNetworkGateway); err != nil {
					return nil, util.WrapError(http.StatusBadRequest, err)
				}

				c.Nameservers = common.DNS{
					DNS: fromIPAddresses(vch.Network.Public.Nameservers),
				}
				c.DNS, err = c.Nameservers.ProcessDNSServers(op)
				if err != nil {
					return nil, util.WrapError(http.StatusBadRequest, err)
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

					path, err := fromManagedObject(op, finder, "Network", cnetwork.PortGroup)
					if err != nil {
						return nil, util.NewError(http.StatusBadRequest, fmt.Sprintf("Error finding portgroup for container network %s: %s", alias, err))
					}
					if path == "" {
						return nil, util.NewError(http.StatusBadRequest, fmt.Sprintf("Container network %s portgroup must be specified (by name or id)", alias))
					}
					containerNetworks.MappedNetworks[alias] = path

					if cnetwork.Gateway != nil {
						address := net.ParseIP(string(cnetwork.Gateway.Address))
						if address == nil {
							return nil, util.NewError(http.StatusBadRequest, fmt.Sprintf("Error parsing gateway IP %s for container network %s", cnetwork.Gateway.Address, alias))
						}
						if cnetwork.Gateway.RoutingDestinations == nil || len(cnetwork.Gateway.RoutingDestinations) != 1 {
							return nil, util.NewError(http.StatusBadRequest, fmt.Sprintf("Error parsing network mask for container network %s: exactly one subnet must be specified", alias))
						}
						_, mask, err := net.ParseCIDR(string(cnetwork.Gateway.RoutingDestinations[0]))
						if err != nil {
							return nil, util.NewError(http.StatusBadRequest, fmt.Sprintf("Error parsing network mask for container network %s: %s", alias, err))
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
							return nil, util.NewError(http.StatusBadRequest, fmt.Sprintf("Error parsing trust level for container network %s: %s", alias, err))
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
				return nil, util.WrapError(http.StatusBadRequest, err)
			}

			if vch.Storage.VolumeStores != nil {
				volumes := make([]string, 0, len(vch.Storage.VolumeStores))
				for _, v := range vch.Storage.VolumeStores {
					volumes = append(volumes, fmt.Sprintf("%s:%s", v.Datastore, v.Label))
				}

				vs := common.VolumeStores{VolumeStores: cli.StringSlice(volumes)}
				volumeLocations, err := vs.ProcessVolumeStores()
				if err != nil {
					return nil, util.NewError(http.StatusBadRequest, fmt.Sprintf("Error processing volume stores: %s", err))
				}
				c.VolumeLocations = volumeLocations
			}

			c.ScratchSize = constants.DefaultBaseImageScratchSize
			if vch.Storage.BaseImageSize != nil {
				c.ScratchSize = fromValueBytesMetric(vch.Storage.BaseImageSize)
			}
		}

		if vch.Auth != nil {
			c.Certs.NoTLS = vch.Auth.NoTLS

			if vch.Auth.Client != nil {
				c.Certs.NoTLSverify = vch.Auth.Client.NoTLSVerify
				c.Certs.ClientCAs = fromPemCertificates(vch.Auth.Client.CertificateAuthorities)
				c.ClientCAs = c.Certs.ClientCAs
			}

			if vch.Auth.Server != nil {

				if vch.Auth.Server.Generate != nil {
					c.Certs.Cname = vch.Auth.Server.Generate.Cname
					c.Certs.Org = vch.Auth.Server.Generate.Organization
					c.Certs.KeySize = fromValueBits(vch.Auth.Server.Generate.Size)

					c.Certs.NoSaveToDisk = true
					c.Certs.Networks = c.Networks
					if err := c.Certs.ProcessCertificates(op, c.DisplayName, c.Force, 0); err != nil {
						return nil, util.NewError(http.StatusBadRequest, fmt.Sprintf("Error generating certificates: %s", err))
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
				c.MemoryMB = *mbFromValueBytes(vch.Endpoint.Memory)
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
			return nil, util.WrapError(http.StatusBadRequest, err)
		}

		if vch.Registry != nil {
			c.InsecureRegistries = vch.Registry.Insecure
			c.WhitelistRegistries = vch.Registry.Whitelist

			c.RegistryCAs = fromPemCertificates(vch.Registry.CertificateAuthorities)

			if vch.Registry.ImageFetchProxy != nil {
				c.Proxies = fromImageFetchProxy(vch.Registry.ImageFetchProxy)

				hproxy, sproxy, err := c.Proxies.ProcessProxies()
				if err != nil {
					return nil, util.NewError(http.StatusBadRequest, fmt.Sprintf("Error processing proxies: %s", err))
				}
				c.HTTPProxy = hproxy
				c.HTTPSProxy = sproxy
			}
		}

		if vch.SyslogAddr != "" {
			c.SyslogAddr = vch.SyslogAddr.String()
			if err := c.ProcessSyslog(); err != nil {
				return nil, util.NewError(http.StatusBadRequest, fmt.Sprintf("Error processing syslog server address: %s", err))
			}
		}

		if vch.Container != nil && vch.Container.NameConvention != "" {
			c.ContainerNameConvention = vch.Container.NameConvention
		}
	}

	return c, nil
}

func handleCreate(op trace.Operation, c *create.Create, validator *validate.Validator, receiver vchlog.Receiver) (*strfmt.URI, error) {
	vchConfig, err := validator.Validate(op, c.Data)
	if err != nil {
		issues := validator.GetIssues()
		messages := make([]string, 0, len(issues))
		for _, issue := range issues {
			messages = append(messages, issue.Error())
		}

		return nil, util.NewError(http.StatusBadRequest, fmt.Sprintf("Failed to validate VCH: %s", strings.Join(messages, ", ")))
	}

	vConfig := validator.AddDeprecatedFields(op, vchConfig, c.Data)

	// TODO (#6714): make this configurable
	images := common.Images{}
	vConfig.ImageFiles, err = images.CheckImagesFiles(op, true)
	vConfig.ApplianceISO = path.Base(images.ApplianceISO)
	vConfig.BootstrapISO = path.Base(images.BootstrapISO)

	vConfig.HTTPProxy = c.HTTPProxy
	vConfig.HTTPSProxy = c.HTTPSProxy

	executor := management.NewDispatcher(op, validator.Session, nil, false)
	err = executor.CreateVCH(vchConfig, vConfig, receiver)
	if err != nil {
		return nil, util.NewError(http.StatusInternalServerError, fmt.Sprintf("Failed to create VCH: %s", err))
	}

	return nil, nil
}

func fromManagedObject(op trace.Operation, finder finder, t string, m *models.ManagedObject) (string, error) {
	if m == nil {
		return "", nil
	}

	if m.ID != "" {
		managedObjectReference := types.ManagedObjectReference{Type: t, Value: m.ID}
		element, err := finder.Element(op, managedObjectReference)

		if err != nil {
			return "", err
		}

		return element.Path, nil
	}

	return m.Name, nil
}

func fromCIDR(m *models.CIDR) string {
	if m == nil {
		return ""
	}

	return string(*m)
}

func fromCIDRs(m *[]models.CIDR) *[]string {
	s := make([]string, 0, len(*m))
	for _, d := range *m {
		s = append(s, fromCIDR(&d))
	}

	return &s
}

func fromIPAddress(m *models.IPAddress) string {
	if m == nil {
		return ""
	}

	return string(*m)
}

func fromIPAddresses(m []models.IPAddress) []string {
	s := make([]string, 0, len(m))
	for _, ip := range m {
		s = append(s, fromIPAddress(&ip))
	}

	return s
}

func fromGateway(m *models.Gateway) string {
	if m == nil {
		return ""
	}

	if m.RoutingDestinations == nil {
		return fmt.Sprintf("%s",
			m.Address,
		)
	}

	return fmt.Sprintf("%s:%s",
		strings.Join(*fromCIDRs(&m.RoutingDestinations), ","),
		m.Address,
	)
}

func fromValueBytesMetric(m *models.ValueBytesMetric) string {
	v := float64(m.Value.Value)

	var bytes float64
	switch m.Value.Units {
	case models.ValueBytesMetricUnitsB:
		bytes = v
	case models.ValueBytesMetricUnitsKB:
		bytes = v * float64(units.KB)
	case models.ValueBytesMetricUnitsMB:
		bytes = v * float64(units.MB)
	case models.ValueBytesMetricUnitsGB:
		bytes = v * float64(units.GB)
	case models.ValueBytesMetricUnitsTB:
		bytes = v * float64(units.TB)
	case models.ValueBytesMetricUnitsPB:
		bytes = v * float64(units.PB)
	}

	return fmt.Sprintf("%d B", int64(bytes))
}

func mbFromValueBytes(m *models.ValueBytes) *int {
	if m == nil {
		return nil
	}

	v := float64(m.Value.Value)

	var mbs float64
	switch m.Value.Units {
	case models.ValueBytesUnitsB:
		mbs = v / float64(units.MiB)
	case models.ValueBytesUnitsKiB:
		mbs = v / (float64(units.MiB) / float64(units.KiB))
	case models.ValueBytesUnitsMiB:
		mbs = v
	case models.ValueBytesUnitsGiB:
		mbs = v * (float64(units.GiB) / float64(units.MiB))
	case models.ValueBytesUnitsTiB:
		mbs = v * (float64(units.TiB) / float64(units.MiB))
	case models.ValueBytesUnitsPiB:
		mbs = v * (float64(units.PiB) / float64(units.MiB))
	}

	i := int(math.Ceil(mbs))
	return &i
}

func mhzFromValueHertz(m *models.ValueHertz) *int {
	if m == nil {
		return nil
	}

	v := float64(m.Value.Value)

	var mhzs float64
	switch m.Value.Units {
	case models.ValueHertzUnitsHz:
		mhzs = v / float64(units.MB)
	case models.ValueHertzUnitsKHz:
		mhzs = v / (float64(units.MB) / float64(units.KB))
	case models.ValueHertzUnitsMHz:
		mhzs = v
	case models.ValueHertzUnitsGHz:
		mhzs = v * (float64(units.GB) / float64(units.MB))
	}

	i := int(math.Ceil(mhzs))
	return &i
}

func fromShares(m *models.Shares) *types.SharesInfo {
	if m == nil {
		return nil
	}

	var level types.SharesLevel
	switch types.SharesLevel(m.Level) {
	case types.SharesLevelLow:
		level = types.SharesLevelLow
	case types.SharesLevelNormal:
		level = types.SharesLevelNormal
	case types.SharesLevelHigh:
		level = types.SharesLevelHigh
	default:
		level = types.SharesLevelCustom
	}

	return &types.SharesInfo{
		Level:  level,
		Shares: int32(m.Number),
	}
}

func fromValueBits(m *models.ValueBits) int {
	return int(m.Value.Value)
}

func fromPemCertificates(m []*models.X509Data) []byte {
	var b []byte

	for _, ca := range m {
		c := []byte(ca.Pem)
		b = append(b, c...)
	}

	return b
}

func fromImageFetchProxy(p *models.VCHRegistryImageFetchProxy) common.Proxies {
	http := string(p.HTTP)
	https := string(p.HTTPS)

	return common.Proxies{
		HTTPProxy:  &http,
		HTTPSProxy: &https,
	}
}
