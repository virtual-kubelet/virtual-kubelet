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

package data

import (
	"fmt"
	"net"
	"net/url"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"

	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/vic/cmd/vic-machine/common"
	"github.com/vmware/vic/lib/config"
	"github.com/vmware/vic/lib/config/executor"
	"github.com/vmware/vic/pkg/ip"
)

// Data wraps all parameters required by value validation
type Data struct {
	*common.Target
	common.Debug
	common.Compute
	common.VCHID
	common.ContainerConfig

	OpsCredentials common.OpsCredentials
	Kubelet        common.Kubelet

	CertPEM     []byte
	KeyPEM      []byte
	ClientCAs   []byte
	RegistryCAs []byte
	common.Images

	ImageDatastorePath string              `cmd:"image-store"`
	VolumeLocations    map[string]*url.URL `cmd:"volume-store" label:"value-key"`

	BridgeNetworkName string        `cmd:"bridge-network"`
	ClientNetwork     NetworkConfig `cmd:"client-network"`
	PublicNetwork     NetworkConfig `cmd:"public-network"`
	ManagementNetwork NetworkConfig `cmd:"management-network"`
	DNS               []net.IP      `cmd:"dns-server"`

	common.ContainerNetworks `cmd:"container-network"`
	common.ResourceLimits

	BridgeIPRange *net.IPNet `cmd:"bridge-network-range"`

	InsecureRegistries  []string `cmd:"insecure-registry"`
	WhitelistRegistries []string `cmd:"whitelist-registry"`

	HTTPSProxy *url.URL `cmd:"https-proxy"`
	HTTPProxy  *url.URL `cmd:"http-proxy"`
	ProxyIsSet bool

	NumCPUs  int `cmd:"endpoint-cpu"`
	MemoryMB int `cmd:"endpoint-memory"`

	Timeout time.Duration

	Force               bool
	ResetInProgressFlag bool

	AsymmetricRouting bool `cmd:"asymmetric-routes"`

	ScratchSize string `cmd:"base-image-size"`
	Rollback    bool

	SyslogConfig SyslogConfig `cmd:"syslog"`
}

type SyslogConfig struct {
	Addr *url.URL `cmd:"address"`
	Tag  string
}

// NetworkConfig is used to set IP addr for each network
type NetworkConfig struct {
	Name         string `cmd:"parent"`
	Destinations []net.IPNet
	Gateway      net.IPNet
	IP           net.IPNet `cmd:"ip"`
}

// Empty determines if ip and gateway are unset
func (n *NetworkConfig) Empty() bool {
	return ip.Empty(n.Gateway) && ip.Empty(n.IP)
}

func (n *NetworkConfig) IsSet() bool {
	return n.Name != "" || !n.Empty()
}

// InstallerData is used to hold the transient installation configuration that shouldn't be serialized
type InstallerData struct {
	// Virtual Container Host capacity
	VCHSize      config.Resources
	VCHSizeIsSet bool

	// Appliance capacity
	ApplianceSize config.Resources

	KeyPEM  string
	CertPEM string

	//FIXME: remove following attributes after port-layer-server read config from guestinfo
	DatacenterName         string
	ClusterPath            string
	ResourcePoolPath       string
	ApplianceInventoryPath string

	Datacenter types.ManagedObjectReference
	Cluster    types.ManagedObjectReference

	ImageFiles map[string]string

	ApplianceISO      string
	BootstrapISO      string
	ISOVersion        string
	PreUpgradeVersion string
	Timeout           time.Duration

	HTTPSProxy *url.URL
	HTTPProxy  *url.URL

	// Used only for configure, so that the current state can be validated relative to the requested changes
	CreateVMGroup bool
	DeleteVMGroup bool
}

func NewData() *Data {
	d := &Data{
		Target: common.NewTarget(),
		ContainerNetworks: common.ContainerNetworks{
			MappedNetworks:          make(map[string]string),
			MappedNetworksGateways:  make(map[string]net.IPNet),
			MappedNetworksIPRanges:  make(map[string][]ip.Range),
			MappedNetworksDNS:       make(map[string][]net.IP),
			MappedNetworksFirewalls: make(map[string]executor.TrustLevel),
		},
		Timeout: 3 * time.Minute,
	}
	return d
}

// equalIPRanges checks if two ip.Range slices are equal by using
// reflect.DeepEqual, with an extra check that returns true when the
// lengths of both slices are zero.
func equalIPRanges(a, b []ip.Range) bool {
	if !reflect.DeepEqual(a, b) {
		return len(a) == 0 && len(b) == 0
	}
	return true
}

// equalIPSlices checks if two net.IP slices are equal by using
// reflect.DeepEqual, with an extra check that returns true when the
// lengths of both slices are zero.
func equalIPSlices(a, b []net.IP) bool {
	if !reflect.DeepEqual(a, b) {
		return len(a) == 0 && len(b) == 0
	}
	return true
}

// const copyInfoMsg = `Please use vic-machine inspect config to find existing
// container network settings and supply them along with new container networks`
const copyInfoMsg = `Please use vic-machine inspect config to find existing
%s settings and supply them along with new %s`

// copyContainerNetworks checks that existing container networks (in d) are present
// in the specified networks (src) and then copies new networks into d. It does not
// overwrite data for existing networks.
func (d *Data) copyContainerNetworks(src *Data) error {
	// Any existing container networks and their related options must be specified
	// while performing a configure operation.
	errMsg := "Existing container-network %s:%s not specified in configure command"
	var netsMissing, netsChanged bool
	for vicNet, vmNet := range d.ContainerNetworks.MappedNetworks {
		if _, ok := src.ContainerNetworks.MappedNetworks[vicNet]; !ok {
			netsMissing = true
			log.Errorf(fmt.Sprintf(errMsg, vmNet, vicNet))
			continue
		}

		// If an existing container network is specified, ensure that all existing settings match the specified settings.
		if !reflect.DeepEqual(d.ContainerNetworks.MappedNetworks[vicNet], src.ContainerNetworks.MappedNetworks[vicNet]) ||
			!reflect.DeepEqual(d.ContainerNetworks.MappedNetworksGateways[vicNet], src.ContainerNetworks.MappedNetworksGateways[vicNet]) ||
			!equalIPRanges(d.ContainerNetworks.MappedNetworksIPRanges[vicNet], src.ContainerNetworks.MappedNetworksIPRanges[vicNet]) ||
			!equalIPSlices(d.ContainerNetworks.MappedNetworksDNS[vicNet], src.ContainerNetworks.MappedNetworksDNS[vicNet]) ||
			!reflect.DeepEqual(d.ContainerNetworks.MappedNetworksFirewalls[vicNet], src.ContainerNetworks.MappedNetworksFirewalls[vicNet]) {

			netsChanged = true
			log.Errorf("Found changes to existing container network %s", vicNet)
		}
	}

	if netsMissing {
		log.Info(fmt.Sprintf(copyInfoMsg, "container network", "container networks"))
		return fmt.Errorf("all existing container networks must also be specified")
	}
	if netsChanged {
		log.Info(fmt.Sprintf(copyInfoMsg, "container network", "container networks"))
		return fmt.Errorf("changes to existing container networks are not supported")
	}

	// Copy data only for new container networks.
	for vicNet, vmNet := range src.ContainerNetworks.MappedNetworks {
		if _, ok := d.ContainerNetworks.MappedNetworks[vicNet]; !ok {
			d.ContainerNetworks.MappedNetworks[vicNet] = vmNet
			d.ContainerNetworks.MappedNetworksGateways[vicNet] = src.ContainerNetworks.MappedNetworksGateways[vicNet]
			d.ContainerNetworks.MappedNetworksIPRanges[vicNet] = src.ContainerNetworks.MappedNetworksIPRanges[vicNet]
			d.ContainerNetworks.MappedNetworksDNS[vicNet] = src.ContainerNetworks.MappedNetworksDNS[vicNet]
			d.ContainerNetworks.MappedNetworksFirewalls[vicNet] = src.ContainerNetworks.MappedNetworksFirewalls[vicNet]
		}
	}

	return nil
}

// copyVolumeStores checks that existing volume stores (in d) are present in the
// specified volume stores (src) and then copies new volume stores into d. It
// does not overwrite data for existing volume stores.
func (d *Data) copyVolumeStores(src *Data) error {
	// Any existing volume stores must be specified while performing a configure operation.
	errMsg := "Existing volume store %s not specified in configure command"
	var storesMissing, storesChanged bool
	for label, url := range d.VolumeLocations {
		srcURL, ok := src.VolumeLocations[label]
		if !ok {
			storesMissing = true
			log.Errorf(fmt.Sprintf(errMsg, label))
			continue
		}

		dURLString := fmt.Sprintf("%s://%s", url.Scheme, filepath.Join(strings.Trim(url.Host, "/"), strings.Trim(url.Path, "/")))
		srcURLString := fmt.Sprintf("%s://%s", srcURL.Scheme, filepath.Join(strings.Trim(srcURL.Host, "/"), strings.Trim(srcURL.Path, "/")))
		if dURLString != srcURLString {
			storesChanged = true
			log.Errorf("Found changes to existing volume store %s", label)
		}
	}

	if storesMissing {
		log.Info(fmt.Sprintf(copyInfoMsg, "volume store", "volume stores"))
		return fmt.Errorf("all existing volume stores must also be specified")
	}
	if storesChanged {
		log.Info(fmt.Sprintf(copyInfoMsg, "volume store", "volume stores"))
		return fmt.Errorf("changes to existing volume stores are not supported")
	}

	// Copy data only for new volume stores.
	for label, url := range src.VolumeLocations {
		if _, ok := d.VolumeLocations[label]; !ok {
			d.VolumeLocations[label] = url
		}
	}

	return nil
}

// CopyNonEmpty will shallow copy src value to override existing value if the value is set.
// This copy will take care of relationship between variables, that means if any variable
// in NetworkConfig is not empty, the whole NetworkConfig will be copied. However, for
// container networks, changes to existing networks will not be overwritten.
func (d *Data) CopyNonEmpty(src *Data) error {
	// TODO: Add data copy here for each reconfigure items, to make sure specified variables present in the Data object.

	if src.ClientNetwork.IsSet() {
		d.ClientNetwork = src.ClientNetwork
	}
	if src.PublicNetwork.IsSet() {
		d.PublicNetwork = src.PublicNetwork
	}
	if src.ManagementNetwork.IsSet() {
		d.ManagementNetwork = src.ManagementNetwork
	}

	if src.ProxyIsSet {
		d.HTTPProxy = src.HTTPProxy
		d.HTTPSProxy = src.HTTPSProxy
	}

	if src.Debug.Debug != nil {
		d.Debug = src.Debug
	}

	if src.ContainerNetworks.IsSet() {
		if err := d.copyContainerNetworks(src); err != nil {
			return err
		}
	}

	if len(src.VolumeLocations) > 0 {
		if err := d.copyVolumeStores(src); err != nil {
			return err
		}
	}

	if src.OpsCredentials.IsSet {
		d.OpsCredentials = src.OpsCredentials
	}

	if src.Target.Thumbprint != "" {
		d.Target.Thumbprint = src.Target.Thumbprint
	}

	resourceIsSet := false
	if src.VCHCPULimitsMHz != nil {
		d.VCHCPULimitsMHz = src.VCHCPULimitsMHz
		resourceIsSet = true
	}
	if src.VCHCPUReservationsMHz != nil {
		d.VCHCPUReservationsMHz = src.VCHCPUReservationsMHz
		resourceIsSet = true
	}
	if src.VCHCPUShares != nil {
		d.VCHCPUShares = src.VCHCPUShares
		resourceIsSet = true
	}

	if src.VCHMemoryLimitsMB != nil {
		d.VCHMemoryLimitsMB = src.VCHMemoryLimitsMB
		resourceIsSet = true
	}
	if src.VCHMemoryReservationsMB != nil {
		d.VCHMemoryReservationsMB = src.VCHMemoryReservationsMB
		resourceIsSet = true
	}
	if src.VCHMemoryShares != nil {
		d.VCHMemoryShares = src.VCHMemoryShares
		resourceIsSet = true
	}
	d.ResourceLimits.IsSet = resourceIsSet

	d.Timeout = src.Timeout

	d.DNS = src.DNS

	d.RegistryCAs = src.RegistryCAs

	d.ContainerConfig.ContainerNameConvention = src.ContainerConfig.ContainerNameConvention

	return nil
}
