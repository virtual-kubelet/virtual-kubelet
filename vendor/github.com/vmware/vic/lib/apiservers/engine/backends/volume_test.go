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
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/vmware/vic/lib/apiservers/engine/proxy"
)

func TestExtractDockerMetadata(t *testing.T) {
	driver := "vsphere"
	volumeName := "testVolume"
	store := "storeName"
	testCap := "512"

	testOptMap := make(map[string]string)
	testOptMap[proxy.OptsVolumeStoreKey] = store
	testOptMap[proxy.OptsCapacityKey] = testCap

	testLabelMap := make(map[string]string)
	testLabelMap["someLabel"] = "this is a label"

	metaDataBefore := proxy.VolumeMetadata{
		Driver:     driver,
		Name:       volumeName,
		DriverOpts: testOptMap,
		Labels:     testLabelMap,
	}

	buf, err := json.Marshal(metaDataBefore)
	if !assert.NoError(t, err) {
		return
	}

	metadataMap := make(map[string]string)
	metadataMap[proxy.DockerMetadataModelKey] = string(buf)
	metadataAfter, err := extractDockerMetadata(metadataMap)
	if !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, metaDataBefore.DriverOpts[proxy.OptsCapacityKey], metadataAfter.DriverOpts[proxy.OptsCapacityKey])
	assert.Equal(t, metaDataBefore.DriverOpts[proxy.OptsVolumeStoreKey], metadataAfter.DriverOpts[proxy.OptsVolumeStoreKey])
	assert.Equal(t, metaDataBefore.Labels["someLabel"], metadataAfter.Labels["someLabel"])
	assert.Equal(t, metaDataBefore.Name, metadataAfter.Name)
	assert.Equal(t, metaDataBefore.Driver, metadataAfter.Driver)
}
