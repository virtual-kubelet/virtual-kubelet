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

package plugin5

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/vmware/vic/pkg/vsphere/extraconfig"
)

func TestMigrateRegistry(t *testing.T) {

	inConfig := []url.URL{
		{Path: "1.1.1.1:5000"},
		{Path: "2.2.2.2"},
		{Host: "3.3.3.3:6000"},
		{Host: "3.3.3.3"},
		{Host: "vic.vmware.com"},
		{Path: "[1234:1234:0:1234::11]:7000"},
		{Host: "[1234:1234:0:1234::11]:7000"},
		{Path: "3.3.3.3/foo/bar"},
	}

	outConfig := []url.URL{
		{Host: "1.1.1.1:5000"},
		{Host: "2.2.2.2"},
		{Host: "3.3.3.3:6000"},
		{Host: "3.3.3.3"},
		{Host: "vic.vmware.com"},
		{Host: "[1234:1234:0:1234::11]:7000"},
		{Host: "[1234:1234:0:1234::11]:7000"},
		{Host: "3.3.3.3", Path: "foo/bar"},
	}

	m := MigrateRegistry{}

	vch := VirtualContainerHostConfigSpec{
		Registry{
			InsecureRegistries: inConfig,
		}}

	mapData := make(map[string]string)
	extraconfig.Encode(extraconfig.MapSink(mapData), vch)

	err := m.Migrate(nil, nil, mapData)
	assert.NoError(t, err)

	vchMigrated := extraconfig.Decode(extraconfig.MapSource(mapData), vch)
	assert.Equal(t, outConfig, vchMigrated.(VirtualContainerHostConfigSpec).InsecureRegistries)
}
