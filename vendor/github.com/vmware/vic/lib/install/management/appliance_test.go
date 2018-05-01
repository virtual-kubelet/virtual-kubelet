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

package management

import (
	"context"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/vmware/vic/lib/config"
	"github.com/vmware/vic/pkg/trace"
)

func TestConfirmVolumeStores(t *testing.T) {
	op := trace.NewOperation(context.Background(), "TestConfirmVolumeStores")

	testVolumeLocations := map[string]*url.URL{
		"test1":        {},
		"test2":        {},
		"volume-store": {},
	}

	plVolumeStores1 := "test2 volume-store"
	plVolumeStores2 := "test1 test2"
	plVolumeStores3 := "volume-store"
	plVolumeStores4 := "test1 test2 volume-store"
	plVolumeStores5 := ""

	testconf := &config.VirtualContainerHostConfigSpec{}
	testconf.VolumeLocations = testVolumeLocations

	result1 := confirmVolumeStores(op, testconf, plVolumeStores1)
	assert.False(t, result1, "Failed first confirmVolumeStores check")

	result2 := confirmVolumeStores(op, testconf, plVolumeStores2)
	assert.False(t, result2, "Failed second confirmVolumeStores check")

	result3 := confirmVolumeStores(op, testconf, plVolumeStores3)
	assert.False(t, result3, "Failed third confirmVolumeStores check")

	result4 := confirmVolumeStores(op, testconf, plVolumeStores4)
	assert.True(t, result4, "Failed fourth confirmVolumeStores check")

	result5 := confirmVolumeStores(op, testconf, plVolumeStores5)
	assert.False(t, result5, "Failed fifth confirmVolumeStores check")
}
