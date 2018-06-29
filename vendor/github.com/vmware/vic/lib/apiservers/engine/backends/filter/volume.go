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
	"fmt"

	"github.com/docker/docker/api/types/filters"
)

// VolumeFilterContext stores volume information used while filtering
type VolumeFilterContext struct {
	FilterContext

	// Dangling is the value of the dangling filter if supplied
	Dangling bool

	// Joined tells whether the volume is joined to a container or not
	Joined bool

	// Driver is the volume's driver
	Driver string
}

// ValidateVolumeFilters checks that the supplied filters are valid and supported
// and returns a context used in the IncludeVolume func.
func ValidateVolumeFilters(volFilters filters.Args, acceptedFilters, unSupportedFilters map[string]bool) (*VolumeFilterContext, error) {

	if err := ValidateFilters(volFilters, acceptedFilters, unSupportedFilters); err != nil {
		return nil, err
	}

	volFilterContext := &VolumeFilterContext{}

	// Set value of dangling filter if it's supplied
	if volFilters.Include("dangling") {

		// Validate dangling filter's usage (per Docker code)
		// Supported formats: dangling={true, false, 1, 0}
		if volFilters.ExactMatch("dangling", "true") || volFilters.ExactMatch("dangling", "1") {
			volFilterContext.Dangling = true
		} else if !volFilters.ExactMatch("dangling", "false") && !volFilters.ExactMatch("dangling", "0") {
			return nil, fmt.Errorf("invalid filter 'dangling=%s'", volFilters.Get("dangling"))
		}
	}

	return volFilterContext, nil
}

// IncludeVolume evaluates volume filters and the filter context and returns
// an action to indicate whether to include the volume in the output or not.
func IncludeVolume(volumeFilters filters.Args, volFilterContext *VolumeFilterContext) FilterAction {

	// Filter by name and label
	action := filterCommon(&volFilterContext.FilterContext, volumeFilters)
	if action != IncludeAction {
		return action
	}

	if volumeFilters.Include("dangling") {
		// Exclude the volume if dangling=false or it is joined,
		// and if dangling=true or it is not joined
		if (!volFilterContext.Dangling || volFilterContext.Joined) && (volFilterContext.Dangling || !volFilterContext.Joined) {
			return ExcludeAction
		}
	}

	if volumeFilters.Include("driver") {
		if !volumeFilters.ExactMatch("driver", volFilterContext.Driver) {
			return ExcludeAction
		}
	}

	return IncludeAction
}
