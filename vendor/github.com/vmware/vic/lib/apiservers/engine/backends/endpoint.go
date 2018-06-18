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

package backends

import (
	"fmt"
	"net"
	"net/http"

	derr "github.com/docker/docker/api/errors"
	"github.com/docker/libnetwork"
	"github.com/docker/libnetwork/types"

	"github.com/vmware/vic/lib/apiservers/portlayer/models"
)

var notImplementedError = derr.NewErrorWithStatusCode(fmt.Errorf("not implemented"), http.StatusInternalServerError)

type endpoint struct {
	ep *models.EndpointConfig
	sc *models.ScopeConfig
}

// A system generated id for this endpoint.
func (e *endpoint) ID() string {
	return e.ep.ID
}

// Name returns the name of this endpoint.
func (e *endpoint) Name() string {
	return e.ep.Name
}

// Network returns the name of the vicnetwork to which this endpoint is attached.
func (e *endpoint) Network() string {
	return e.ep.Scope
}

// Join joins the sandbox to the endpoint and populates into the sandbox
// the vicnetwork resources allocated for the endpoint.
func (e *endpoint) Join(sandbox libnetwork.Sandbox, options ...libnetwork.EndpointOption) error {
	return notImplementedError
}

// Leave detaches the vicnetwork resources populated in the sandbox.
func (e *endpoint) Leave(sandbox libnetwork.Sandbox, options ...libnetwork.EndpointOption) error {
	return notImplementedError
}

// Return certain operational data belonging to this endpoint
func (e *endpoint) Info() libnetwork.EndpointInfo {
	return e
}

// DriverInfo returns a collection of driver operational data related to this endpoint retrieved from the driver
func (e *endpoint) DriverInfo() (map[string]interface{}, error) {
	return nil, notImplementedError
}

// Delete and detaches this endpoint from the vicnetwork.
func (e *endpoint) Delete(force bool) error {
	return notImplementedError
}

// Iface returns InterfaceInfo, go interface that can be used
// to get more information on the interface which was assigned to
// the endpoint by the driver. This can be used after the
// endpoint has been created.
func (e *endpoint) Iface() libnetwork.InterfaceInfo {
	return e
}

// Gateway returns the IPv4 gateway assigned by the driver.
// This will only return a valid value if a container has joined the endpoint.
func (e *endpoint) Gateway() net.IP {
	return net.ParseIP(e.sc.Gateway)
}

// GatewayIPv6 returns the IPv6 gateway assigned by the driver.
// This will only return a valid value if a container has joined the endpoint.
func (e *endpoint) GatewayIPv6() net.IP {
	return nil
}

// StaticRoutes returns the list of static routes configured by the vicnetwork
// driver when the container joins a vicnetwork
func (e *endpoint) StaticRoutes() []*types.StaticRoute {
	return nil
}

// Sandbox returns the attached sandbox if there, nil otherwise.
func (e *endpoint) Sandbox() libnetwork.Sandbox {
	return newSandbox(e.ep.Container)
}

// MacAddress returns the MAC address assigned to the endpoint.
func (e *endpoint) MacAddress() net.HardwareAddr {
	return nil
}

// Address returns the IPv4 address assigned to the endpoint.
func (e *endpoint) Address() *net.IPNet {
	ip := net.ParseIP(e.ep.Address)
	if ip == nil {
		return nil
	}

	_, snet, err := net.ParseCIDR(e.sc.Subnet)
	if err != nil {
		return nil
	}

	return &net.IPNet{IP: ip, Mask: snet.Mask}
}

// AddressIPv6 returns the IPv6 address assigned to the endpoint.
func (e *endpoint) AddressIPv6() *net.IPNet {
	return nil
}

func (e *endpoint) LinkLocalAddresses() []*net.IPNet {
	return nil
}
