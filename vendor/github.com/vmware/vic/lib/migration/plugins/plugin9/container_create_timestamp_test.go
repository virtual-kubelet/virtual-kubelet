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

package plugin9

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/vmware/vic/lib/config/executor"
	"github.com/vmware/vic/pkg/vsphere/extraconfig"
)

// TestMigrateContainerCreateTimestamp tests this package's ContainerCreateTimestampVersion.Migrate.
func TestMigrateContainerCreateTimestamp(t *testing.T) {
	secSinceEpoch := int64(1510950922)
	oldCreateTime := secSinceEpoch
	newCreateTime := secSinceEpoch * 1e9
	layerID := "abcdefg"
	execConfig := executor.ExecutorConfig{
		CreateTime: oldCreateTime,
		// Supply an extra field that's not accessed in the plugin9 migrator to
		// ensure that unneeded fields aren't dropped from the returned data.
		LayerID: layerID,
	}

	c := ContainerCreateTimestampVersion{}
	mapData := make(map[string]string)
	extraconfig.Encode(extraconfig.MapSink(mapData), execConfig)

	err := c.Migrate(nil, nil, mapData)
	assert.NoError(t, err)

	newConf := extraconfig.Decode(extraconfig.MapSource(mapData), execConfig)
	assert.Equal(t, newCreateTime, newConf.(executor.ExecutorConfig).CreateTime)
	assert.Equal(t, layerID, newConf.(executor.ExecutorConfig).LayerID)
}
