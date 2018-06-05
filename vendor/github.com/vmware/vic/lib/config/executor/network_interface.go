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

package executor

import (
	"net"

	"github.com/vmware/vic/pkg/ip"
)

// NetworkEndpoint describes a network presence in the form a vNIC in sufficient detail that it can be:
// a. created - the vNIC added to a VM
// b. identified - the guestOS can determine which interface it corresponds to
// c. configured - the guestOS can configure the interface correctly
type NetworkEndpoint struct {
	// Common.Name - the nic alias requested (only one name and one alias possible in linux)
	// Common.ID - pci slot of the vnic allowing for interface identifcation in-guest
	Common

	// Whether this endpoint's IP was specified by the client (true if it was)
	Static bool `vic:"0.1" scope:"read-only" key:"static"`

	// IP address to assign
	IP *net.IPNet `vic:"0.1" scope:"read-only" key:"ip"`

	// Actual IP address assigned
	Assigned net.IPNet `vic:"0.1" scope:"read-write" key:"assigned"`

	// The network in which this information should be interpreted. This is embedded directly rather than
	// as a pointer so that we can ensure the data is consistent
	Network ContainerNetwork `vic:"0.1" scope:"read-only" key:"network"`

	// The list of exposed ports on the container
	Ports []string `vic:"0.1" scope:"read-only" key:"ports"`

	// whether or not this represents an internal network
	Internal bool `vic:"0.1" scope:"read-only" key:"internal"`
}

// ContainerNetwork is the data needed on a per container basis both for vSphere to ensure it's attached
// to the correct network, and in the guest to ensure the interface is correctly configured.
type ContainerNetwork struct {
	// Common.Name - the symbolic name for the network, e.g. web or backend
	// Common.ID - identifier of the underlay for the network
	Common

	Type string `vic:"0.1" scope:"read-write" key:"type"`

	// Destinations is a list of CIDRs used for routing traffic to the gateway
	Destinations []net.IPNet `vic:"0.1" scope:"read-only" key:"destinations"`

	// The network scope the IP belongs to.
	// The IP address is the default gateway
	Gateway net.IPNet `vic:"0.1" scope:"read-only" key:"gateway"`

	// Should this gateway be the default route for containers on the network
	Default bool `vic:"0.1" scope:"read-only" key:"default"`

	// The set of nameservers associated with this network - may be empty
	Nameservers []net.IP `vic:"0.1" scope:"read-only" key:"dns"`

	// The IP ranges for this network
	Pools []ip.Range `vic:"0.1" scope:"read-only" key:"pools"`

	// set of network wide links and aliases for this container on this network
	Aliases []string `vic:"0.1" scope:"hidden" key:"aliases"`

	// Level of trust configured for this network
	TrustLevel

	Assigned struct {
		Gateway     net.IPNet `vic:"0.1" scope:"read-write" key:"gateway"`
		Nameservers []net.IP  `vic:"0.1" scope:"read-write" key:"dns"`
	} `vic:"0.1" scope:"read-write" key:"assigned"`
}
