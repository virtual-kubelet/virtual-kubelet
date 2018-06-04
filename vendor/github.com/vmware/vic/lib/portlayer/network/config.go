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

package network

import (
	"github.com/vmware/govmomi/object"
	"github.com/vmware/vic/lib/config"
	"github.com/vmware/vic/pkg/vsphere/extraconfig"
)

type Configuration struct {
	source extraconfig.DataSource `vic:"0.1" scope:"read-only" recurse:"depth=0"`
	sink   extraconfig.DataSink   `vic:"0.1" scope:"read-only" recurse:"depth=0"`

	// Turn on debug logging
	DebugLevel int `vic:"0.1" scope:"read-only" key:"init/diagnostics/debug"`

	// Port Layer - network
	config.Network `vic:"0.1" scope:"read-only" key:"network"`

	// The bridge link
	BridgeLink Link `vic:"0.1" scope:"read-only" recurse:"depth=0"`

	// the vsphere portgroups corresponding to container network configuration
	PortGroups map[string]object.NetworkReference `vic:"0.1" scope:"read-only" recurse:"depth=0"`
}

func (c *Configuration) Encode() {
	extraconfig.Encode(c.sink, c)
}

func (c *Configuration) Decode() {
	extraconfig.Decode(c.source, c)
}
