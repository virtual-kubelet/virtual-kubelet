// Copyright 2016-2017 VMware, Inc. All Rights Reserved.
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

package config

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"net/mail"
	"net/url"
	"time"

	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/vic/lib/config/executor"
	"github.com/vmware/vic/pkg/certificate"
)

// PatternToken is a set of tokens that can be placed into string constants
// for containerVMs that will be replaced with the specific values
type PatternToken string

const (
	// VM is the VM name - i.e. [ds] {vm}/{vm}.vmx
	VMToken PatternToken = "{vm}"
	// ID is the container ID for the VM
	IDToken PatternToken = "{id}"
	// Name is the container name of the VM
	NameToken PatternToken = "{name}"

	// The default naming pattern that gets applied if no convention is supplied
	DefaultNamePattern = "{name}-{id}"

	// ID represents the VCH in creating status, which helps to identify VCH VM which still does not have a valid VM moref set
	CreatingVCH = "CreatingVCH"

	PublicNetworkName     = "public"
	ClientNetworkName     = "client"
	ManagementNetworkName = "management"

	PersonaService        = "docker-personality"
	PortLayerService      = "port-layer"
	VicAdminService       = "vicadmin"
	KubeletStarterService = "kubelet-starter"

	GeneralHTTPProxy   = "HTTP_PROXY"
	GeneralHTTPSProxy  = "HTTPS_PROXY"
	VICAdminHTTPProxy  = "VICADMIN_HTTP_PROXY"
	VICAdminHTTPSProxy = "VICADMIN_HTTPS_PROXY"

	AddPerms = "ADD"
)

func (p PatternToken) String() string {
	return string(p)
}

// Can we just treat the VCH appliance as a containerVM booting off a specific bootstrap image
// It has many of the same requirements (around networks being attached, version recorded,
// volumes mounted, et al). Each of the components can easily be captured as a Session given they
// are simply processes.
// This would require that the bootstrap read session record for the VM and relaunch them - that
// actually aligns very well with containerVMs restarting their processes if restarted directly
// (this is obviously a behaviour we'd want to toggles for in regular containers).

// VirtualContainerHostConfigSpec holds the metadata for a Virtual Container Host that should be visible inside the appliance VM.
type VirtualContainerHostConfigSpec struct {
	// The base config for the appliance. This includes the networks that are to be attached
	// and disks to be mounted.
	// Networks are keyed by interface name
	executor.ExecutorConfig `vic:"0.1" scope:"read-only" key:"init"`

	// vSphere connection configuration
	Connection `vic:"0.1" scope:"read-only" key:"connect"`

	// basic contact information
	Contacts `vic:"0.1" scope:"read-only" key:"contact"`

	// certificate configuration, for both inbound and outbound access
	Certificate `vic:"0.1" scope:"read-only" key:"cert"`

	// Port Layer - storage
	Storage `vic:"0.1" scope:"read-only" key:"storage"`

	// Port Layer - network
	Network `vic:"0.1" scope:"read-only" key:"network"`

	// Port Layer - exec
	Container `vic:"0.1" scope:"read-only" key:"container"`

	// Registry configuration for Imagec
	Registry `vic:"0.1" scope:"read-only" key:"registry"`

	// virtual kubelet specific options
	Kubelet `vic:"0.1" scope:"read-only" key:"virtual_kubelet"`

	// configuration for vic-machine
	CreateBridgeNetwork bool `vic:"0.1" scope:"read-only" key:"create_bridge_network"`

	// grant ops-user permissions, string instead of bool for future enhancements
	GrantPermsLevel string `vic:"0.1" scope:"read-only" key:"grant_permissions"`

	// vic-machine create options used to create or reconfigure the VCH
	VicMachineCreateOptions []string `vic:"0.1" scope:"read-only" key:"vic_machine_create_options"`
}

// ContainerConfig holds the container configuration for a virtual container host
type Container struct {
	// Default containerVM capacity
	ContainerVMSize Resources `vic:"0.1" scope:"read-only" recurse:"depth=0"`
	// Resource pools under which all containers will be created
	ComputeResources []types.ManagedObjectReference `vic:"0.1" scope:"read-only"`
	// Path of the ISO to use for bootstrapping containers
	BootstrapImagePath string `vic:"0.1" scope:"read-only" key:"bootstrap_image_path"`
	// Allow custom naming convention for containerVMs
	ContainerNameConvention string
	// Permitted datastore URLs for container storage for this virtual container host
	ContainerStores []url.URL `vic:"0.1" scope:"read-only" recurse:"depth=0"`
}

// RegistryConfig defines the registries virtual container host can talk to
type Registry struct {
	// Whitelist of registries
	RegistryWhitelist []string `vic:"0.1" scope:"read-only" key:"whitelist_registries"`
	// Blacklist of registries
	RegistryBlacklist []string `vic:"0.1" scope:"read-only" recurse:"depth=0"`
	// Insecure registries
	InsecureRegistries []string `vic:"0.1" scope:"read-only" key:"insecure_registries"`
}

// Virtual Kubelet
type Kubelet struct {
	KubernetesServerAddress string
	KubeletConfigFile       string
	KubeletConfigContent    string
}

// NetworkConfig defines the network configuration of virtual container host
type Network struct {
	// The network to use by default to provide access to the world
	BridgeNetwork string `vic:"0.1" scope:"read-only" key:"bridge_network"`
	// Published networks available for containers to join, keyed by consumption name
	ContainerNetworks map[string]*executor.ContainerNetwork `vic:"0.1" scope:"read-only" key:"container_networks"`
	// The IP range for the bridge networks
	BridgeIPRange *net.IPNet `vic:"0.1" scope:"read-only" key:"bridge-ip-range"`
	// The width of each new bridge network
	BridgeNetworkWidth *net.IPMask `vic:"0.1" scope:"read-only" key:"bridge-net-width"`
}

// StorageConfig defines the storage configuration including images and volumes
type Storage struct {
	// Datastore URLs for image stores - the top layer is [0], the bottom layer is [len-1]
	ImageStores []url.URL `vic:"0.1" scope:"read-only" key:"image_stores"`
	// Permitted datastore URL roots for volumes
	// Keyed by the volume store name (which is used by the docker user to
	// refer to the datstore + path), valued by the datastores and the path.
	VolumeLocations map[string]*url.URL `vic:"0.1" scope:"read-only"`
	// default size for root image
	ScratchSize int64 `vic:"0.1" scope:"read-only" key:"scratch_size"`
}

type Certificate struct {
	// Certificates for user authentication - this needs to be expanded to allow for directory server auth
	UserCertificates []*RawCertificate
	// Certificates for general outgoing network access, keyed by CIDR (IPNet.String())
	NetworkCertificates map[string]*RawCertificate
	// The certificate used to validate the appliance to clients
	HostCertificate *RawCertificate `vic:"0.1" scope:"read-only"`
	// The CAs to validate client connections
	CertificateAuthorities []byte `vic:"0.1" scope:"read-only"`
	// The CAs to validate docker registry connections
	RegistryCertificateAuthorities []byte `vic:"0.1" scope:"read-only"`
	// Certificates for specific system access, keyed by FQDN
	HostCertificates map[string]*RawCertificate
}

// Connection holds the vSphere connection configuration
type Connection struct {
	// The sdk URL
	Target string `vic:"0.1" scope:"read-only" key:"target"`
	// Username for target login
	Username string `vic:"0.1" scope:"read-only" key:"username"`
	// Token is an SSO token or password
	Token string `vic:"0.1" scope:"secret" key:"token"`
	// TargetThumbprint is the SHA-1 digest of the Target's public certificate
	TargetThumbprint string `vic:"0.1" scope:"read-only" key:"target_thumbprint"`
	// The session timeout
	Keepalive time.Duration `vic:"0.1" scope:"read-only" key:"keepalive"`
}

type Contacts struct {
	// Administrative contact for the Virtual Container Host
	Admin []mail.Address
	// Administrative contact for hosting infrastructure
	InfrastructureAdmin []mail.Address
}

// RawCertificate is present until we add extraconfig support for [][]byte slices that are present
// in tls.Certificate
type RawCertificate struct {
	Key  []byte `vic:"0.1" scope:"secret"`
	Cert []byte
}

// CustomerExperienceImprovementProgram provides configuration for the phone home mechanism
// This is broken out so that we can have more granular configuration in here in the future
// and so that it is insulated from changes to Virtual Container Host structure
type CustomerExperienceImprovementProgram struct {
	// The server target is as follows, where the uuid is the raw number, no dashes
	// "https://vcsa.vmware.com/ph-stg/api/hyper/send?_v=1.0&_c=vic.1_0&_i="+vc.uuid
	// If this is non-nil then it's enabled
	CEIPGateway url.URL
}

// Resources is used instead of the ResourceAllocation structs in govmomi as
// those don't currently hold IO or storage related data.
type Resources struct {
	CPU     types.ResourceAllocationInfo
	Memory  types.ResourceAllocationInfo
	IO      types.ResourceAllocationInfo
	Storage types.ResourceAllocationInfo
}

// SetHostCertificate sets the certificate for authenticting with the appliance itself
func (t *VirtualContainerHostConfigSpec) SetHostCertificate(key *[]byte) {
	t.ExecutorConfig.Key = *key
}

// SetName sets the name of the VCH - this will be used as the hostname for the appliance
func (t *VirtualContainerHostConfigSpec) SetName(name string) {
	t.ExecutorConfig.Name = name
}

// SetDebug configures the debug logging level for the VCH
func (t *VirtualContainerHostConfigSpec) SetDebug(level int) {
	t.ExecutorConfig.Diagnostics.DebugLevel = level
}

// SetMoref sets the moref of the VCH - this allows components to acquire a handle to
// the appliance VM.
func (t *VirtualContainerHostConfigSpec) SetMoref(moref *types.ManagedObjectReference) {
	if moref != nil {
		t.ExecutorConfig.ID = moref.String()
	}
}

// SetIsCreating sets the ID of the VCH to a constant if creating is true, to identify the creating VCH VM before the VM moref can be set into this property
// Reset the property back to empty string if creating is false
func (t *VirtualContainerHostConfigSpec) SetIsCreating(creating bool) {
	if creating {
		t.ExecutorConfig.ID = CreatingVCH
	} else {
		t.ExecutorConfig.ID = ""
	}
}

// IsCreating is checking if this configuration is for one creating VCH VM
func (t *VirtualContainerHostConfigSpec) IsCreating() bool {
	return t.ExecutorConfig.ID == CreatingVCH
}

func (t *VirtualContainerHostConfigSpec) SetGrantPerms() {
	t.GrantPermsLevel = AddPerms
}

func (t *VirtualContainerHostConfigSpec) ClearGrantPerms() {
	t.GrantPermsLevel = ""
}

func (t *VirtualContainerHostConfigSpec) ShouldGrantPerms() bool {
	return t.GrantPermsLevel == AddPerms
}

// AddNetwork adds a network that will be configured on the appliance VM
func (t *VirtualContainerHostConfigSpec) AddNetwork(net *executor.NetworkEndpoint) {
	if net != nil {
		if t.ExecutorConfig.Networks == nil {
			t.ExecutorConfig.Networks = make(map[string]*executor.NetworkEndpoint)
		}

		t.ExecutorConfig.Networks[net.Network.Name] = net
	}
}

// AddContainerNetwork adds a network that will be configured on the appliance VM
func (t *VirtualContainerHostConfigSpec) AddContainerNetwork(net *executor.ContainerNetwork) {
	if net != nil {
		if t.ContainerNetworks == nil {
			t.ContainerNetworks = make(map[string]*executor.ContainerNetwork)
		}

		t.ContainerNetworks[net.Name] = net
	}
}

func (t *VirtualContainerHostConfigSpec) AddComponent(name string, component *executor.SessionConfig) {
	if component != nil {
		if t.ExecutorConfig.Sessions == nil {
			t.ExecutorConfig.Sessions = make(map[string]*executor.SessionConfig)
		}

		if component.Name == "" {
			component.Name = name
		}
		if component.ID == "" {
			component.ID = name
		}
		t.ExecutorConfig.Sessions[name] = component
	}
}

func (t *VirtualContainerHostConfigSpec) AddImageStore(url *url.URL) {
	if url != nil {
		t.ImageStores = append(t.ImageStores, *url)
	}
}

func (t *VirtualContainerHostConfigSpec) AddVolumeLocation(name string, u *url.URL) {

	if u != nil {
		if t.VolumeLocations == nil {
			t.VolumeLocations = make(map[string]*url.URL)
		}

		t.VolumeLocations[name] = u
	}
}

// AddComputeResource adds a moref to the set of permitted root pools. It takes a ResourcePool rather than
// an inventory path to encourage validation.
func (t *VirtualContainerHostConfigSpec) AddComputeResource(pool *types.ManagedObjectReference) {
	if pool != nil {
		t.ComputeResources = append(t.ComputeResources, *pool)
	}
}

func CreateSession(cmd string, args ...string) *executor.SessionConfig {
	cfg := &executor.SessionConfig{
		Cmd: executor.Cmd{
			Path: cmd,
			Args: []string{
				cmd,
			},
		},
	}

	cfg.Cmd.Args = append(cfg.Cmd.Args, args...)

	return cfg
}

func (t *RawCertificate) Certificate() (*tls.Certificate, error) {
	if t.IsNil() {
		return nil, errors.New("nil certificate")
	}
	cert, err := tls.X509KeyPair(t.Cert, t.Key)
	return &cert, err
}

func (t *RawCertificate) X509Certificate() (*x509.Certificate, error) {
	if t.IsNil() {
		return nil, errors.New("nil certificate")
	}
	cert, _, err := certificate.ParseCertificate(t.Cert, t.Key)
	return cert, err
}

func (t *RawCertificate) IsNil() bool {
	if t == nil {
		return true
	}

	return len(t.Cert) == 0 && len(t.Key) == 0
}
