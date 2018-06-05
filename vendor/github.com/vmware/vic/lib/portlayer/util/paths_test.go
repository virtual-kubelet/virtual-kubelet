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

package util

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestImageStoreName(t *testing.T) {
	u, _ := url.Parse("/storage/images/imgstore/image")
	store, err := ImageStoreName(u)
	if !assert.NoError(t, err) {
		return
	}

	expectedStore := "imgstore"
	if !assert.Equal(t, expectedStore, store) {
		return
	}
}

func TestImageStoreNameErrors(t *testing.T) {
	u, _ := url.Parse("fail")
	_, err := ImageStoreName(u)
	expectedError := "invalid uri path"
	if err.Error() != expectedError {
		t.Errorf("Got: %s Expected: %s", err, expectedError)
	}

	u, _ = url.Parse("/storage:123")
	_, err = ImageStoreName(u)
	expectedError = "not a storage path"
	if err.Error() != expectedError {
		t.Errorf("Got: %s Expected: %s", err, expectedError)
	}

	u, _ = url.Parse("/storage")
	_, err = ImageStoreName(u)
	expectedError = "not a storage path"
	if err.Error() != expectedError {
		t.Errorf("Got: %s Expected: %s", err, expectedError)
	}
}

func TestImageURL(t *testing.T) {
	DefaultHost, _ = url.Parse("http://foo.com/")
	storeName := "storeName"
	imageName := "imageName"

	u, err := ImageURL(storeName, imageName)
	if err != nil {
		t.Errorf("ImageURL failed %v", err)
	}
	expectedURL := "http://foo.com/storage/images/storeName/imageName"
	if !assert.Equal(t, expectedURL, u.String()) {
		return
	}
}

func TestVolumeStoreName(t *testing.T) {
	u, _ := url.Parse("/storage/volumes/volstore/volume")
	store, err := VolumeStoreName(u)
	if !assert.NoError(t, err) {
		return
	}

	expectedStore := "volstore"
	if !assert.Equal(t, expectedStore, store) {
		return
	}
}

func TestVolumeStoreNameErrors(t *testing.T) {
	u, _ := url.Parse("fail")
	_, err := VolumeStoreName(u)
	expectedError := "invalid uri path"
	if err.Error() != expectedError {
		t.Errorf("Got: %s Expected: %s", err, expectedError)
	}

	u, _ = url.Parse("/storage:123")
	_, err = VolumeStoreName(u)
	expectedError = "not a storage path"
	if err.Error() != expectedError {
		t.Errorf("Got: %s Expected: %s", err, expectedError)
	}

	u, _ = url.Parse("/storage")
	_, err = VolumeStoreName(u)
	expectedError = "not a storage path"
	if err.Error() != expectedError {
		t.Errorf("Got: %s Expected: %s", err, expectedError)
	}
}

func TestVolumeURL(t *testing.T) {
	DefaultHost, _ = url.Parse("http://foo.com/")
	storeName := "storeName"
	volumeName := "volumeName"

	u, err := VolumeURL(storeName, volumeName)
	if err != nil {
		t.Errorf("VolumeURL failed %v", err)
	}
	expectedURL := "http://foo.com/storage/volumes/storeName/volumeName"
	if !assert.Equal(t, expectedURL, u.String()) {
		return
	}
}
