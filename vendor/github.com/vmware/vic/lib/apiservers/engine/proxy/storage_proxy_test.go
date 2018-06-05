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

package proxy

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/vmware/vic/lib/apiservers/portlayer/models"
)

func TestFillDockerVolume(t *testing.T) {
	testResponse := &models.VolumeResponse{
		Driver: "vsphere",
		Name:   "Test Volume",
		Label:  "Test Label",
	}
	testLabels := make(map[string]string)
	testLabels["TestMeta"] = "custom info about my volume"

	dockerVolume := NewVolumeModel(testResponse, testLabels)

	assert.Equal(t, "vsphere", dockerVolume.Driver)
	assert.Equal(t, "Test Volume", dockerVolume.Name)
	assert.Equal(t, "Test Label", dockerVolume.Mountpoint)
	assert.Equal(t, "custom info about my volume", dockerVolume.Labels["TestMeta"])
}

func TestTranslatVolumeRequestModel(t *testing.T) {
	testLabels := make(map[string]string)
	testLabels["TestMeta"] = "custom info about my volume"

	testDriverArgs := make(map[string]string)
	testDriverArgs["testarg"] = "important driver stuff"
	testDriverArgs[OptsVolumeStoreKey] = "testStore"
	testDriverArgs[OptsCapacityKey] = "12MB"

	testRequest, err := newVolumeCreateReq("testName", "vsphere", testDriverArgs, testLabels)
	if !assert.Error(t, err) {
		return
	}

	delete(testDriverArgs, "testarg")
	testRequest, err = newVolumeCreateReq("testName", "vsphere", testDriverArgs, testLabels)
	if !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, "testName", testRequest.Name)
	assert.Equal(t, "", testRequest.DriverArgs["testarg"]) // unsupported keys should just be empty
	assert.Equal(t, "testStore", testRequest.Store)
	assert.Equal(t, "vsphere", testRequest.Driver)
	assert.Equal(t, int64(12), testRequest.Capacity)

	testMetaDatabuf, err := createVolumeMetadata(testRequest, testDriverArgs, testLabels)
	if !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, testMetaDatabuf, testRequest.Metadata[DockerMetadataModelKey])
	assert.Nil(t, err)
}

func TestValidateDriverArgs(t *testing.T) {
	testMap := make(map[string]string)
	testStore := "Mystore"
	testCap := "12MB"
	testBadCap := "This is not valid!"
	testModel := models.VolumeRequest{
		Driver:     "vsphere",
		DriverArgs: testMap,
		Name:       "testModel",
	}

	err := validateDriverArgs(testMap, &testModel)
	if !assert.Equal(t, "default", testModel.Store) || !assert.Equal(t, int64(-1), testModel.Capacity) || !assert.NoError(t, err) {
		return
	}

	testMap[OptsVolumeStoreKey] = testStore
	testMap[OptsCapacityKey] = testCap
	err = validateDriverArgs(testMap, &testModel)
	if !assert.Equal(t, testStore, testModel.Store) || !assert.Equal(t, int64(12), testModel.Capacity) || !assert.NoError(t, err) {
		return
	}

	//This is a negative test case. We want an error
	testMap[OptsCapacityKey] = testBadCap
	err = validateDriverArgs(testMap, &testModel)
	if !assert.Equal(t, testStore, testModel.Store) || !assert.Equal(t, int64(12), testModel.Capacity) || !assert.Error(t, err) {
		return
	}

	testMap[OptsCapacityKey] = testCap
	delete(testMap, OptsVolumeStoreKey)
	err = validateDriverArgs(testMap, &testModel)
	if !assert.Equal(t, "default", testModel.Store) || !assert.Equal(t, int64(12), testModel.Capacity) || !assert.NoError(t, err) {
		return
	}
}

func TestNormalizeDriverArgs(t *testing.T) {
	testOptMap := make(map[string]string)
	testOptMap["VOLUMESTORE"] = "foo"
	testOptMap["CAPACITY"] = "bar"

	normalizeDriverArgs(testOptMap)

	assert.Equal(t, testOptMap["volumestore"], "foo")
	assert.Equal(t, testOptMap["capacity"], "bar")

	testOptMap["bogus"] = "bogus"

	err := normalizeDriverArgs(testOptMap)
	assert.Error(t, err, "expected: bogus is not a supported option")
}
