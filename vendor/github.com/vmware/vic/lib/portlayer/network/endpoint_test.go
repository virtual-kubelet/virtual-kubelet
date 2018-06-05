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
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEndpointNameID(t *testing.T) {
	c := &Container{id: "foo", name: "bar"}
	s := &Scope{
		gateway: net.ParseIP("10.10.10.1"),
		subnet:  &net.IPNet{IP: net.ParseIP("10.10.10.0"), Mask: net.CIDRMask(24, 32)},
	}
	e := Endpoint{
		container: c,
		scope:     s,
		ip:        net.ParseIP("10.10.10.10"),
		static:    true,
		ports:     make(map[Port]interface{}),
	}

	assert.Equal(t, c.ID(), e.ID())
	assert.Equal(t, c.Name(), e.Name())
}

func TestEndpointCopy(t *testing.T) {
	c := &Container{id: "foo"}
	s := &Scope{
		gateway: net.ParseIP("10.10.10.1"),
		subnet:  &net.IPNet{IP: net.ParseIP("10.10.10.0"), Mask: net.CIDRMask(24, 32)},
	}
	e := Endpoint{
		container: c,
		scope:     s,
		ip:        net.ParseIP("10.10.10.10"),
		static:    true,
		ports:     make(map[Port]interface{}),
	}

	p, err := ParsePort("80/tcp")
	assert.NoError(t, err, "")
	e.ports[p] = nil

	other := e.copy()

	assert.Equal(t, other.ID(), e.ID())
	assert.Equal(t, other.container, c)
	assert.Equal(t, other.container, e.container)
	assert.Equal(t, other.scope, s)
	assert.Equal(t, other.scope, e.scope)
	assert.True(t, other.ip.Equal(e.ip), "other.ip (%s) != e.ip (%s)", other.ip, e.ip)
	assert.True(t, other.Gateway().Equal(e.Gateway()), "other.Gateway() (%s) != e.Gateway() (%s)", other.Gateway(), e.Gateway())
	assert.True(t, other.Subnet().IP.Equal(e.Subnet().IP), "other.Subnet() (%s) != e.Subnet() (%s)", other.Subnet(), e.Subnet())
	assert.Equal(t, other.Subnet().Mask, e.Subnet().Mask, "other.Subnet() (%s) != e.Subnet() (%s)", other.Subnet(), e.Subnet())
	assert.EqualValues(t, other.ports, e.ports)

	// make sure .ports is a copy
	other.ports["foo"] = nil
	assert.NotContains(t, e.ports, "foo")
}
