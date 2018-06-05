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

package v2

import (
	"net"
	"net/mail"
	"net/url"
	"time"

	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/vic/lib/config/executor"
)

// PatternToken is a set of tokens that can be placed into string constants
// for containerVMs that will be replaced with the specific values
type PatternToken string

const (
	// VM is the VM name - i.e. [ds] {vm}/{vm}.vmx
	VM PatternToken = "{vm}"
	// ID is the container ID for the VM
	ID = "{id}"
	// Name is the container name of the VM
	Name = "{name}"
)

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

	// configuration for vic-machine
	CreateBridgeNetwork bool `vic:"0.1" scope:"read-only" key:"create_bridge_network"`
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
	RegistryWhitelist []url.URL `vic:"0.1" scope:"read-only" recurse:"depth=0"`
	// Blacklist of registries
	RegistryBlacklist []url.URL `vic:"0.1" scope:"read-only" recurse:"depth=0"`
	// Insecure registries
	InsecureRegistries []url.URL `vic:"0.1" scope:"read-only" key:"insecure_registries"`
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

// Remove all methods from this file to reduce binary size
