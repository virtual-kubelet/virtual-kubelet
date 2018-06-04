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
	"strconv"

	"github.com/docker/docker/api/types"

	"github.com/vmware/vic/lib/apiservers/engine/backends/cache"
	"github.com/vmware/vic/lib/apiservers/portlayer/models"
)

// reused from docker/docker/daemon/list.go
type ContainerListContext struct {
	FilterContext

	// Counter is the container iteration index for this context
	Counter int
	// ExitCode for the passed container
	ExitCode int
	// exitAllowed is a list of exit codes allowed to filter with
	exitAllowed map[int]struct{}
	// ContainerListOptions is the filters set by the user
	*types.ContainerListOptions
}

// IncludeContainer will evaluate the filter criteria in listContext against the provided
// container and determine what action to take. There are three options:
//  * IncludeAction
//  * ExcludeAction
//  * StopAction
func IncludeContainer(listContext *ContainerListContext, container *models.ContainerInfo) FilterAction {

	// if we need to filter on name add to the listContext
	if listContext.Filters.Include("name") {
		// containerConfig allows for multiple names, but only 1 ever
		// assigned
		listContext.Name = container.ContainerConfig.Names[0]
	}

	// filter common requirements
	act := filterCommon(&listContext.FilterContext, listContext.Filters)
	if act != IncludeAction {
		return act
	}

	// Stop iteration when the index is over the limit
	if listContext.Limit > 0 && listContext.Counter == listContext.Limit {
		return StopAction
	}

	// Do we have exit codes to evaluate
	if len(listContext.exitAllowed) > 0 {

		// Is the containers exitCode in the validatedList?
		_, ok := listContext.exitAllowed[listContext.ExitCode]

		// only include container whose exit code is in the list and that's
		// not currently running and has been started previously
		// note "Running" state is congruent with PortLayer and not docker
		if !ok || container.ContainerConfig.State == "Running" || container.ProcessConfig.StartTime == 0 {
			return ExcludeAction
		}
	}

	state := DockerState(container.ContainerConfig.State)
	// Do not include container if its status doesn't match the filter
	if !listContext.Filters.Match("status", state) {
		return ExcludeAction
	}

	// Filter on network name
	if listContext.Filters.Include("network") {
		netFilterValues := listContext.Filters.Get("network")

		// Exclude the container if its network(s) match no supplied filter values
		exists := false
		for i := range netFilterValues {
			for j := range container.Endpoints {
				if netFilterValues[i] == container.Endpoints[j].Scope {
					exists = true
					break
				}
			}
		}
		if !exists {
			return ExcludeAction
		}
	}

	// Filter on volume name
	if listContext.Filters.Include("volume") {
		volFilterValues := listContext.Filters.Get("volume")

		// Exclude the container if its volume(s) match no supplied filter values
		exists := false
		for i := range volFilterValues {
			for j := range container.VolumeConfig {
				if volFilterValues[i] == container.VolumeConfig[j].Name {
					exists = true
					break
				}
			}
		}
		if !exists {
			return ExcludeAction
		}
	}

	return IncludeAction
}

// ValidateContainerFilters validates that the container filters are
// valid docker filters / values and supported by VIC.
// The function reuses Docker's filter validation.
func ValidateContainerFilters(options *types.ContainerListOptions, acceptedFilters map[string]bool, unSupportedFilters map[string]bool) (*ContainerListContext, error) {
	containerFilters := options.Filters

	// ensure filter options are valid and supported by vic
	if err := ValidateFilters(containerFilters, acceptedFilters, unSupportedFilters); err != nil {
		return nil, err
	}

	// we need all containers for these options, so set the All flag
	if options.Limit > 0 || options.Latest {
		options.All = true
	}

	var s struct{}
	filtExited := make(map[int]struct{})

	err := containerFilters.WalkValues("exited", func(value string) error {
		code, err := strconv.Atoi(value)
		if err != nil {
			return err
		}
		// add valid exit code to map
		filtExited[code] = s
		return nil
	})
	if err != nil {
		return nil, err
	}

	err = containerFilters.WalkValues("status", func(value string) error {
		if !IsValidDockerState(value) {
			return fmt.Errorf("Unrecognised filter value for status: %s", value)
		}
		options.All = true
		return nil
	})
	if err != nil {
		return nil, err
	}

	// return value
	listContext := &ContainerListContext{
		FilterContext:        FilterContext{},
		exitAllowed:          filtExited,
		ContainerListOptions: options,
	}

	err = containerFilters.WalkValues("before", func(value string) error {
		var err error
		before := cache.ContainerCache().GetContainer(value)
		if before == nil {
			err = fmt.Errorf("No such container: %s", value)
		} else {
			listContext.BeforeID = &before.ContainerID
		}
		return err
	})
	if err != nil {
		return nil, err
	}

	err = containerFilters.WalkValues("since", func(value string) error {
		var err error
		since := cache.ContainerCache().GetContainer(value)
		if since == nil {
			err = fmt.Errorf("No such container: %s", value)
		} else {
			listContext.SinceID = &since.ContainerID
		}
		return err
	})
	if err != nil {
		return nil, err
	}

	return listContext, nil
}

// DockerState will attempt to transform the passed state
// to a valid docker state
// valid states are listed in the func IsValidContainerState
func DockerState(containerState string) string {
	var state string
	switch containerState {
	case "Stopped":
		state = "exited"
	case "Running":
		state = "running"
	case "Created":
		state = "created"
	default:
		// not sure what to do, so just return
		// what was given
		state = containerState
	}
	return state
}

// IsValidDockerState will verify the provided state is
// a valid docker container state
func IsValidDockerState(s string) bool {

	if s != "paused" &&
		s != "restarting" &&
		s != "removing" &&
		s != "running" &&
		s != "dead" &&
		s != "created" &&
		s != "exited" {
		return false
	}
	return true
}
