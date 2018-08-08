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

func TestValidateVolumeFilters(t *testing.T) {

	// Valid filters
	volFilters := filters.NewArgs()
	volFilters.Add("dangling", "true")
	volFilters.Add("name", "mulder")
	volFilters.Add("driver", "aliens")
	volFilterContext, err := ValidateVolumeFilters(volFilters, acceptedVolumeFilterTags, nil)
	assert.NoError(t, err)
	assert.Equal(t, volFilterContext.Dangling, true)

	// Change a filter's value
	volFilters.Del("dangling", "true")
	volFilters.Add("dangling", "false")
	volFilterContext, err = ValidateVolumeFilters(volFilters, acceptedVolumeFilterTags, nil)
	assert.NoError(t, err)
	assert.Equal(t, volFilterContext.Dangling, false)

	// Valid filter with invalid value
	volFilters.Del("dangling", "false")
	volFilters.Add("dangling", "no")
	volFilterContext, err = ValidateVolumeFilters(volFilters, acceptedVolumeFilterTags, nil)
	assert.Error(t, err)

	// Invalid filter
	volFilters.Add("mountpoint", "/volumes")
	_, err = ValidateVolumeFilters(volFilters, acceptedVolumeFilterTags, nil)
	assert.Error(t, err)
}

func TestIncludeVolume(t *testing.T) {

	// Filter by dangling=true
	volFilters := filters.NewArgs()
	volFilters.Add("dangling", "true")
	volFilterContext := &VolumeFilterContext{
		FilterContext: FilterContext{
			Name:   "scully",
			Labels: map[string]string{"samplelabel": ""},
		},
		Driver:   "science",
		Joined:   false,
		Dangling: true,
	}
	action := IncludeVolume(volFilters, volFilterContext)
	assert.Equal(t, action, IncludeAction)

	// Filter by dangling=false
	volFilters.Del("dangling", "true")
	volFilters.Add("dangling", "false")
	volFilterContext.Dangling = false
	action = IncludeVolume(volFilters, volFilterContext)
	assert.Equal(t, action, ExcludeAction)

	// Filter by name and dangling=false
	volFilters.Add("name", "scul")
	volFilterContext.Joined = true
	action = IncludeVolume(volFilters, volFilterContext)
	assert.Equal(t, action, IncludeAction)

	// Filter by name, dangling=false and driver=science
	volFilters.Add("driver", "science")
	action = IncludeVolume(volFilters, volFilterContext)
	assert.Equal(t, action, IncludeAction)

	// Filter by incorrect name, dangling=false and incorrect driver
	volFilterContext.Name = "mulder"
	volFilterContext.Driver = "aliens"
	action = IncludeVolume(volFilters, volFilterContext)
	assert.Equal(t, action, ExcludeAction)

	// Filter by name, dangling=false and incorrect driver
	volFilterContext.Name = "scully"
	volFilterContext.Driver = "science"
	volFilters.Del("driver", "science")
	volFilters.Add("driver", "sci")
	action = IncludeVolume(volFilters, volFilterContext)
	assert.Equal(t, action, ExcludeAction)

	// Filter by correct label
	volFilters = filters.NewArgs()
	volFilters.Add("label", "samplelabel")
	action = IncludeVolume(volFilters, volFilterContext)
	assert.Equal(t, action, IncludeAction)

	// Filter by incorrect label
	volFilters.Del("label", "samplelabel")
	volFilters.Add("label", "wronglabel")
	action = IncludeVolume(volFilters, volFilterContext)
	assert.Equal(t, action, ExcludeAction)
}
