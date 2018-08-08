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

package create

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseGatewaySpec(t *testing.T) {
	var tests = []struct {
		in   string
		dest []string
		gw   string
		err  error
	}{
		{
			in: "10.10.10.10",
			gw: "10.10.10.10",
		},
		{
			in:   "10.12.0.0/16:10.10.10.10",
			dest: []string{"10.12.0.0/16"},
			gw:   "10.10.10.10",
		},
		{
			in:   "10.13.0.0/16,10.12.0.0/16:10.10.10.10",
			dest: []string{"10.13.0.0/16", "10.12.0.0/16"},
			gw:   "10.10.10.10",
		},
	}

	for _, te := range tests {
		dest, gw, err := parseGatewaySpec(te.in)
		if te.err != nil {
			assert.EqualError(t, err, te.err.Error())
		} else {
			assert.NoError(t, err)
		}

		assert.NotNil(t, gw)
		assert.Equal(t, te.gw, gw.IP.String())

		assert.Equal(t, len(te.dest), len(dest))
		for _, d := range te.dest {
			found := false
			for _, d2 := range dest {
				if d2.String() == d {
					found = true
					break
				}
			}

			assert.True(t, found)
		}
	}
}

func TestFlags(t *testing.T) {
	c := NewCreate()
	flags := c.Flags()
	numberOfFlags := 62
	assert.Equal(t, numberOfFlags, len(flags), "Missing flags during Create.")
}

func TestProcessBridgeNetwork(t *testing.T) {
	c := NewCreate()

	c.BridgeIPRange = "172.16.0.0.0"
	improperBridgeIPRange := c.ProcessBridgeNetwork()
	assert.NotNil(t, improperBridgeIPRange)

	c.BridgeIPRange = "172.16.0.0/12"
	properBridgeIPRange := c.ProcessBridgeNetwork()
	assert.Nil(t, properBridgeIPRange, "Bridge Network IP Range can't be parsed. Range must be in CIDR format, e.g., 172.16.0.0/12.")
}

func TestSetFields(t *testing.T) {
	c := NewCreate()
	option := c.SetFields()
	assert.NotNil(t, option)
}

func TestProcessSysLog(t *testing.T) {
	c := NewCreate()
	c.SyslogAddr = ""
	r := c.ProcessSyslog()
	assert.Nil(t, r, "Should be nil, SyslogAddr is empty")
}
