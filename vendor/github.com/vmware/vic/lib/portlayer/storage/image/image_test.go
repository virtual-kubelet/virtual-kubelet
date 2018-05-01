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

package image

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/vmware/vic/lib/constants"
	"github.com/vmware/vic/lib/portlayer/util"
)

func TestImageCopy(t *testing.T) {
	storeName := "testStore"
	ID := "testImageID"

	imageURL, err := util.ImageURL(storeName, ID)
	if !assert.NoError(t, err) {
		return
	}

	parentURL, err := util.ImageURL(storeName, constants.ScratchLayerID)
	if !assert.NoError(t, err) {
		return
	}

	img, err := Parse(imageURL)
	if !assert.NoError(t, err) || !assert.NotNil(t, img) {
		return
	}

	storeURL, err := util.ImageStoreNameToURL(storeName)
	if !assert.NoError(t, err) {
		return
	}

	expected := &Image{
		ID:         ID,
		SelfLink:   imageURL,
		ParentLink: parentURL,
		Store:      storeURL,
		Metadata: map[string][]byte{
			"1": {byte(1)},
			"2": {byte(2)},
			"3": []byte("three"),
		},
	}

	actual := expected.Copy().(*Image)

	if !assert.Equal(t, expected, actual) {
		return
	}

	actual.Metadata["4"] = []byte("four")

	if !assert.NotEqual(t, expected, actual) {
		return
	}
}
