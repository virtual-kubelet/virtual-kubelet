// Copyright 2016-2018 VMware, Inc. All Rights Reserved.
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

package validate

import (
	"context"
	"net"
	"net/url"

	log "github.com/Sirupsen/logrus"

	"github.com/vmware/vic/cmd/vic-machine/common"
	"github.com/vmware/vic/lib/config"
	"github.com/vmware/vic/lib/config/executor"
	"github.com/vmware/vic/lib/install/data"
	"github.com/vmware/vic/pkg/ip"
	"github.com/vmware/vic/pkg/trace"
)

var (
	inputConfigAdminPassword = "Admin!23"
	inputConfigOpsUser       = "ops-user@vsphere.local"
	inputConfigOpsPassword   = "ops-user-password"
)

var testInputConfigVPX = data.Data{
	Target: &common.Target{
		URL: &url.URL{
			Scheme:     "http",
			Opaque:     "",
			User:       url.UserPassword("administrator@vsphere.local", "Admin!23"),
			Host:       "", // This is set after the simulator starts
			Path:       "/DC0",
			RawPath:    "",
			ForceQuery: false,
			RawQuery:   "",
			Fragment:   "",
		},
		User:       "administrator@vsphere.local",
		Password:   &inputConfigAdminPassword,
		Thumbprint: "",
	},
	Debug:           common.Debug{},
	Compute:         common.Compute{ComputeResourcePath: "/DC0/host/DC0_C0/Resources/DC0_C0_RP1", DisplayName: "vch-test-1787"},
	VCHID:           common.VCHID{},
	ContainerConfig: common.ContainerConfig{},
	OpsCredentials: common.OpsCredentials{
		OpsUser:     &inputConfigOpsUser,
		OpsPassword: &inputConfigOpsPassword,
		IsSet:       false,
	},
	CertPEM:     nil,
	KeyPEM:      nil,
	ClientCAs:   nil,
	RegistryCAs: nil,
	Images: common.Images{ApplianceISO: "V1.2.0-RC1-100000-DD73850-appliance.iso",
		BootstrapISO: "V1.2.0-RC1-100000-DD73850-bootstrap.iso", OSType: "linux"},
	ImageDatastorePath: "LocalDS_0",
	VolumeLocations: map[string]*url.URL{
		"default": {
			Scheme:     "ds",
			Opaque:     "",
			User:       (*url.Userinfo)(nil),
			Host:       "",
			Path:       "LocalDS_0/volumes",
			RawPath:    "",
			ForceQuery: false,
			RawQuery:   "",
			Fragment:   "",
		},
		"local": {
			Scheme:     "ds",
			Opaque:     "",
			User:       (*url.Userinfo)(nil),
			Host:       "",
			Path:       "LocalDS_0/volumes_local",
			RawPath:    "",
			ForceQuery: false,
			RawQuery:   "",
			Fragment:   "",
		},
		"nfs": {
			Scheme:     "nfs",
			Opaque:     "",
			User:       (*url.Userinfo)(nil),
			Host:       "nfs-host",
			Path:       "vic-volumes:nas",
			RawPath:    "",
			ForceQuery: false,
			RawQuery:   "",
			Fragment:   "",
		},
	},
	BridgeNetworkName: "DC0_DVPG0",
	ClientNetwork: data.NetworkConfig{
		Name:         "VM Network",
		Destinations: nil,
		Gateway:      net.IPNet{},
		IP:           net.IPNet{},
	},
	PublicNetwork: data.NetworkConfig{
		Name:         "VM Network",
		Destinations: nil,
		Gateway:      net.IPNet{},
		IP:           net.IPNet{},
	},
	ManagementNetwork: data.NetworkConfig{
		Name:         "VM Network",
		Destinations: nil,
		Gateway:      net.IPNet{},
		IP:           net.IPNet{},
	},
	DNS: nil,
	ContainerNetworks: common.ContainerNetworks{
		MappedNetworks:          map[string]string{},
		MappedNetworksGateways:  map[string]net.IPNet{},
		MappedNetworksIPRanges:  map[string][]ip.Range{},
		MappedNetworksDNS:       map[string][]net.IP{},
		MappedNetworksFirewalls: map[string]executor.TrustLevel{},
	},
	ResourceLimits: common.ResourceLimits{},
	BridgeIPRange: &net.IPNet{
		IP:   []byte{0xac, 0x10, 0x0, 0x0},
		Mask: []byte{0xff, 0xf0, 0x0, 0x0},
	},
	InsecureRegistries:  nil,
	WhitelistRegistries: nil,
	HTTPSProxy:          (*url.URL)(nil),
	HTTPProxy:           (*url.URL)(nil),
	ProxyIsSet:          false,
	NumCPUs:             1,
	MemoryMB:            2048,
	Timeout:             180000000000,
	Force:               true,
	ResetInProgressFlag: false,
	AsymmetricRouting:   false,
	ScratchSize:         "8GB",
	Rollback:            false,
	SyslogConfig:        data.SyslogConfig{},
}

func getVcsimInputConfig(ctx context.Context, URL *url.URL) *data.Data {
	localInputConfig := testInputConfigVPX
	// Fix the URL to point to vcsim
	if URL != nil {
		// Update the Host from the URL
		localInputConfig.Target.URL.Host = URL.Host
	}
	// Copy the URL pointer
	localInputConfig.Target = common.NewTarget()
	*localInputConfig.Target = *testInputConfigVPX.Target
	localInputConfig.Target.URL = new(url.URL)
	*localInputConfig.Target.URL = *testInputConfigVPX.Target.URL
	return &localInputConfig
}

// This method allows to perform validation of a configuration when
// interacting with GO vmomi simulator, it skips some of the tests
// that otherwise would fail (e.g. Firewall)
func (v *Validator) vcsimValidate(ctx context.Context, localInputConfig *data.Data) (*config.VirtualContainerHostConfigSpec, error) {
	defer trace.End(trace.Begin(""))
	op := trace.FromContext(ctx, "validateForSim")
	log.Infof("Validating supplied configuration")

	conf := &config.VirtualContainerHostConfigSpec{}

	if err := v.datacenter(op, false); err != nil {
		return conf, err
	}

	v.basics(op, localInputConfig, conf)

	v.target(op, localInputConfig, conf)
	v.credentials(op, localInputConfig, conf)
	v.compute(op, localInputConfig, conf)
	v.storage(op, localInputConfig, conf)
	v.network(op, localInputConfig, conf)
	v.CheckLicense(op)
	v.checkDRS(op, localInputConfig)

	// Perform the higher level compatibility and consistency checks
	v.compatibility(op, conf)

	v.syslog(op, conf, localInputConfig)

	pool, err := v.ResourcePoolHelper(op, localInputConfig.ComputeResourcePath)
	v.NoteIssue(err)

	if pool == nil {
		return conf, v.ListIssues(op)
	}

	// Add the resource pool
	conf.ComputeResources = append(conf.ComputeResources, pool.Reference())

	// Add the VM
	vm, err := v.session.Finder.VirtualMachine(op, "/DC0/vm/DC0_C0_RP0_VM0")
	v.NoteIssue(err)

	if vm == nil {
		return conf, v.ListIssues(op)
	}

	vmRef := vm.Reference()
	conf.SetMoref(&vmRef)

	// TODO: determine if this is where we should turn the noted issues into message
	return conf, v.ListIssues(op)
}
