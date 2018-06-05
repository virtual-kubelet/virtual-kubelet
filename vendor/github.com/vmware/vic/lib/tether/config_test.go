// Copyright 2016 VMware, Inc. All Rights Reserved.
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

package tether

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/vmware/vic/lib/config/executor"
	"github.com/vmware/vic/pkg/ip"
	"github.com/vmware/vic/pkg/vsphere/extraconfig"
)

var (
	localhost, lmask, _ = net.ParseCIDR("127.0.0.2/24")
	gateway, gmask, _   = net.ParseCIDR("127.0.0.1/24")
)

func TestToExtraConfig(t *testing.T) {
	exec := executor.ExecutorConfig{
		ExecutorConfigCommon: executor.ExecutorConfigCommon{
			ID:   "deadbeef",
			Name: "configtest",
		},
		Sessions: map[string]*executor.SessionConfig{
			"deadbeef": {
				Cmd: executor.Cmd{
					Path: "/bin/bash",
					Args: []string{"/bin/bash", "-c", "echo hello"},
					Dir:  "/",
					Env:  []string{"HOME=/", "PATH=/bin"},
				},
			},
			"beefed": {
				Cmd: executor.Cmd{
					Path: "/bin/bash",
					Args: []string{"/bin/bash", "-c", "echo goodbye"},
					Dir:  "/",
					Env:  []string{"HOME=/", "PATH=/bin"},
				},
			},
		},
		Networks: map[string]*executor.NetworkEndpoint{
			"eth0": {
				Static: true,
				IP:     &net.IPNet{IP: localhost, Mask: lmask.Mask},
				Network: executor.ContainerNetwork{
					Common: executor.Common{
						Name: "notsure",
					},
					Gateway:      net.IPNet{IP: gateway, Mask: gmask.Mask},
					Destinations: []net.IPNet{},
					Nameservers:  []net.IP{},
					Pools:        []ip.Range{},
					Aliases:      []string{},
				},
			},
		},
	}

	exec.Networks["eth0"].Network.Assigned.Nameservers = []net.IP{}

	// encode exec package's ExecutorConfig
	encoded := map[string]string{}
	extraconfig.Encode(extraconfig.MapSink(encoded), exec)

	// decode into this package's ExecutorConfig
	var decoded ExecutorConfig
	extraconfig.Decode(extraconfig.MapSource(encoded), &decoded)

	// the source and destination structs are different - we're doing a sparse comparison
	expectedNet := exec.Networks["eth0"]
	actualNet := decoded.Networks["eth0"]

	assert.Equal(t, expectedNet.Common, actualNet.Common)
	assert.Equal(t, expectedNet.Static, actualNet.Static)
	assert.Equal(t, expectedNet.Assigned, actualNet.Assigned)
	assert.Equal(t, expectedNet.Network, actualNet.Network)

	expectedSession := exec.Sessions["deadbeef"]
	actualSession := decoded.Sessions["deadbeef"]

	assert.Equal(t, expectedSession.Cmd.Path, actualSession.Cmd.Path)
	assert.Equal(t, expectedSession.Cmd.Args, actualSession.Cmd.Args)
	assert.Equal(t, expectedSession.Cmd.Dir, actualSession.Cmd.Dir)
	assert.Equal(t, expectedSession.Cmd.Env, actualSession.Cmd.Env)
}
