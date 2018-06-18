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

package filter

import (
	"testing"

	"github.com/docker/docker/api/types/filters"
	"github.com/stretchr/testify/assert"
)

// TODO: what can we do here...don't like this, but can't
// reference upper package due to import constraints
// valid filters as of docker commit 49bf474
var acceptedImageFilterTags = map[string]bool{
	"dangling":  true,
	"label":     true,
	"before":    true,
	"since":     true,
	"reference": true,
}

// currently not supported by vic
var unSupportedImageFilters = map[string]bool{
	"dangling": false,
}

// valid filters as of docker commit 49bf474
var acceptedPsFilterTags = map[string]bool{
	"ancestor":  true,
	"before":    true,
	"exited":    true,
	"id":        true,
	"isolation": true,
	"label":     true,
	"name":      true,
	"status":    true,
	"health":    true,
	"since":     true,
	"volume":    true,
	"network":   true,
	"is-task":   true,
}

// currently not supported by vic
var unSupportedPsFilters = map[string]bool{
	"ancestor":  false,
	"health":    false,
	"isolation": false,
	"is-task":   false,
}

// valid volume filters as of Docker v1.13
var acceptedVolumeFilterTags = map[string]bool{
	"dangling": true,
	"name":     true,
	"driver":   true,
	"label":    true,
}

func TestValidateFilters(t *testing.T) {
	args := filters.NewArgs()
	args.Add("id", "12345")
	args.Add("name", "jojo")

	// valid container filter
	assert.NoError(t, ValidateFilters(args, acceptedPsFilterTags, unSupportedPsFilters))

	// unsupported container filter
	args.Add("isolation", "windows")
	assert.Error(t, ValidateFilters(args, acceptedPsFilterTags, unSupportedPsFilters))

	// invalid container filter
	args.Add("failure", "yoyo")
	assert.Error(t, ValidateFilters(args, acceptedPsFilterTags, unSupportedPsFilters))

	// unsupported image filter
	args = filters.NewArgs()
	args.Add("dangling", "true")
	assert.Error(t, ValidateFilters(args, acceptedImageFilterTags, unSupportedImageFilters))

	// invalid image filter
	args.Add("failure", "yoyo")
	assert.Error(t, ValidateFilters(args, acceptedImageFilterTags, unSupportedImageFilters))

	// valid image filter
	args = filters.NewArgs()
	args.Add("label", "124")
	assert.NoError(t, ValidateFilters(args, acceptedImageFilterTags, unSupportedImageFilters))

	// valid volume filter
	args = filters.NewArgs()
	args.Add("name", "vol")
	assert.NoError(t, ValidateFilters(args, acceptedVolumeFilterTags, nil))

	// invalid volume filter
	args.Add("mountpoint", "/volumes")
	assert.Error(t, ValidateFilters(args, acceptedVolumeFilterTags, nil))
}
