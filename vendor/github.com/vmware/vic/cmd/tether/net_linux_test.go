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

// +build linux

package main

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vishvananda/netlink"

	"github.com/vmware/vic/lib/config/executor"
	"github.com/vmware/vic/lib/tether"
	"github.com/vmware/vic/pkg/vsphere/extraconfig"
)

// addInterface utility method to add an interface to Mocked
// This assigns the interface name and returns the "slot" as a string
func addInterface(name string, mocker *Mocker) string {
	mocker.maxSlot++

	mocker.Interfaces[name] = &Interface{
		LinkAttrs: netlink.LinkAttrs{
			Name:  name,
			Index: mocker.maxSlot,
		},
		Up: true,
	}

	return strconv.Itoa(mocker.maxSlot)
}

func TestOutboundRuleAndCmd(t *testing.T) {
	t.Skip("https://github.com/vmware/vic/issues/5965")

	mocker := testSetup(t)
	defer testTeardown(t, mocker)

	bridge := addInterface("eth1", mocker)

	ip, _ := netlink.ParseIPNet("172.16.0.2/24")
	gwIP, _ := netlink.ParseIPNet("172.16.0.1/24")

	cfg := executor.ExecutorConfig{
		ExecutorConfigCommon: executor.ExecutorConfigCommon{
			ID:   "outboundrule",
			Name: "tether_test_executor",
		},
		Diagnostics: executor.Diagnostics{
			DebugLevel: 3,
		},
		Networks: map[string]*executor.NetworkEndpoint{
			"bridge": {
				Common: executor.Common{
					ID: bridge,
					// interface rename
					Name: "bridge",
				},
				Network: executor.ContainerNetwork{
					Common: executor.Common{
						Name: "bridge",
					},
					Default: true,
					Gateway: *gwIP,
				},
				Static: true,
				IP:     ip,
			},
		},

		Sessions: map[string]*executor.SessionConfig{
			"outboundrule": {
				Common: executor.Common{
					ID:   "outboundrule",
					Name: "tether_test_session",
				},
				Tty:    false,
				Active: true,

				Cmd: executor.Cmd{
					// test relative path
					Path: "./date",
					Args: []string{"./date", "--reference=/"},
					Env:  []string{"PATH="},
					Dir:  "/bin",
				},
			},
		},
	}

	_, src, _ := StartTether(t, &cfg, mocker)

	fmt.Println("Waiting for tether start")
	<-mocker.Started

	// wait for tether to exit
	fmt.Println("Waiting for tether exit")
	<-mocker.Cleaned

	result := tether.ExecutorConfig{}
	extraconfig.Decode(src, &result)

	// confirm outbound rules configured
	// this should modify state depending on prior rule state

	// confirm command ran - necessary to detect early exit due to net config error
	// TODO: this should be modifed to fail if the last rule to be configured hasn't completed with expected output
	// when this is run. Pending mocked iptables interface
	assert.Equal(t, "true", result.Sessions["outboundrule"].Started, "Expected command to have been started successfully")
	assert.Equal(t, 0, result.Sessions["outboundrule"].ExitStatus, "Expected command to have exited cleanly")

}
