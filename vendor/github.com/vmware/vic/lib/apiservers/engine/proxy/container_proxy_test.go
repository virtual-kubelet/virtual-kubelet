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

package proxy

import (
	"testing"

	"github.com/docker/go-connections/nat"
	"github.com/stretchr/testify/assert"
)

func TestProcessVolumeParams(t *testing.T) {
	rawTestVolumes := []string{"/blah", "testVolume:/mount", "testVolume:/mount/path:r"}
	invalidVolume := "/dir:/dir"
	var processedTestVolumes []volumeFields

	for _, testString := range rawTestVolumes {
		processedFields, err := processVolumeParam(testString)
		assert.Nil(t, err)
		processedTestVolumes = append(processedTestVolumes, processedFields)
	}
	assert.Equal(t, 3, len(processedTestVolumes))

	assert.NotEmpty(t, processedTestVolumes[0].ID)
	assert.Equal(t, "/blah", processedTestVolumes[0].Dest)
	assert.Equal(t, "rw", processedTestVolumes[0].Flags)

	assert.Equal(t, "testVolume", processedTestVolumes[1].ID)
	assert.Equal(t, "/mount", processedTestVolumes[1].Dest)
	assert.Equal(t, "rw", processedTestVolumes[1].Flags)

	assert.Equal(t, "testVolume", processedTestVolumes[2].ID)
	assert.Equal(t, "/mount/path", processedTestVolumes[2].Dest)
	assert.Equal(t, "r", processedTestVolumes[2].Flags)

	invalidFields, _ := processVolumeParam(invalidVolume)
	assert.Equal(t, volumeFields{}, invalidFields)
}

func TestPort(t *testing.T) {
	portMap, bindingMap, err := nat.ParsePortSpecs([]string{
		"1236:1235/tcp",
		"1237:1235/tcp",
		"2345/udp", "80",
		"127.0.0.1::8080",
		"127.0.0.1:5279:8080",
	})
	if err != nil {
		t.Errorf("Failed to parse ports: %s", err.Error())
	}
	t.Logf("portMap: %s", portMap)
	t.Logf("bindingMap: %s", bindingMap)

	for p := range bindingMap {
		expected := bindingMap[p]
		for i := range expected {
			expected[i].HostIP = ""
		}

		bindings := fromPortbinding(p, bindingMap[p])
		t.Logf("binding: %s", bindings)
		_, outMap, err := nat.ParsePortSpecs(bindings)
		if err != nil {
			t.Errorf("Failed to parse back string bindings: %s", err)
		}
		for op := range outMap {
			assert.Equal(t, outMap[op], bindingMap[op])
		}

	}
}
