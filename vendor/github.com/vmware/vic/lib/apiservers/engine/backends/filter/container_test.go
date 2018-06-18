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

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/stretchr/testify/assert"

	"github.com/vmware/vic/lib/apiservers/engine/backends/cache"
	viccontainer "github.com/vmware/vic/lib/apiservers/engine/backends/container"
	"github.com/vmware/vic/lib/apiservers/portlayer/models"
)

func TestValidateContainerFilters(t *testing.T) {

	options := &types.ContainerListOptions{
		Filters: filters.NewArgs(),
	}
	options.Filters.Add("id", "12345")
	options.Filters.Add("status", "running")
	options.Filters.Add("exited", "143")
	options.Filters.Add("exited", "127")
	// valid status & exit
	listContext, err := ValidateContainerFilters(options, acceptedPsFilterTags, unSupportedPsFilters)
	assert.NoError(t, err)

	// we should have two exit codes added to the list
	// context
	assert.Equal(t, 2, len(listContext.exitAllowed))
	assert.Equal(t, options.Filters, listContext.Filters)

	// remove valid status and replace w/invalid
	options.Filters.Del("status", "running")
	options.Filters.Add("status", "jackedup")

	// invalid status
	_, err = ValidateContainerFilters(options, acceptedPsFilterTags, unSupportedPsFilters)
	assert.Error(t, err)

	// remove valid exit code and replace w/invalid
	options.Filters.Del("exited", "143")
	options.Filters.Add("exited", "abc")

	// invalid exit code
	_, err = ValidateContainerFilters(options, acceptedPsFilterTags, unSupportedPsFilters)
	assert.Error(t, err)

	// add an invalid container option
	options.Filters.Add("jojo", "jojo")

	// invalid container filter option
	_, err = ValidateContainerFilters(options, acceptedPsFilterTags, unSupportedPsFilters)
	assert.Error(t, err)

	options.Filters.Del("jojo", "jojo")

	// add before filter
	options.Filters = filters.NewArgs()
	options.Filters.Add("before", "1234")
	// fail because the before container isn't present
	_, err = ValidateContainerFilters(options, acceptedPsFilterTags, unSupportedPsFilters)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "No such container:")

	// add the container to the cache
	containerBefore := &viccontainer.VicContainer{
		ContainerID: "12345",
		Name:        "fuzzy",
	}
	cache.ContainerCache().AddContainer(containerBefore)

	// successful before validation
	_, err = ValidateContainerFilters(options, acceptedPsFilterTags, unSupportedPsFilters)
	assert.NoError(t, err)

	options.Filters.Add("since", "8888")

	// fail because the since container isn't present
	_, err = ValidateContainerFilters(options, acceptedPsFilterTags, unSupportedPsFilters)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "No such container:")

}

func TestDockerState(t *testing.T) {
	vicState := make([]string, 4, 4)
	vicState[0] = "Running"
	vicState[1] = "Stopped"
	vicState[2] = "Created"
	vicState[3] = "sammy"

	docker := make(map[string]bool)
	docker["created"] = true
	docker["running"] = true
	docker["exited"] = true

	// This is not a docker state, but is used to validate the
	// default switch in the tested function
	docker["sammy"] = false

	for i := range vicState {
		if _, ok := docker[DockerState(vicState[i])]; !ok {
			t.Errorf("vicState doesn't map to docker state: %s", vicState[i])
		}
	}

}

func TestIncludeContainer(t *testing.T) {

	ep := &models.EndpointConfig{
		Scope: "bridge",
	}
	eps := make([]*models.EndpointConfig, 0)

	vol := &models.VolumeConfig{
		Name: "fooVol",
	}
	vols := make([]*models.VolumeConfig, 0)

	contain := &models.ContainerInfo{
		ContainerConfig: &models.ContainerConfig{
			Names: []string{"jojo"},
		},
		ProcessConfig: &models.ProcessConfig{},
		VolumeConfig:  append(vols, vol),
		Endpoints:     append(eps, ep),
	}

	listCtx := &ContainerListContext{
		ContainerListOptions: &types.ContainerListOptions{
			Filters: filters.NewArgs()},
	}

	listCtx.Filters.Add("name", "jojo")
	assert.Equal(t, IncludeAction, IncludeContainer(listCtx, contain))

	listCtx.Limit = 1
	listCtx.Counter = listCtx.Limit
	assert.Equal(t, StopAction, IncludeContainer(listCtx, contain))

	// reset counter
	listCtx.Counter = 0

	// create exited map
	var s struct{}
	listCtx.exitAllowed = make(map[int]struct{})
	listCtx.exitAllowed[137] = s

	// exclude since no container exit code
	assert.Equal(t, ExcludeAction, IncludeContainer(listCtx, contain))

	startTime := int64(4444)
	contain.ProcessConfig.StartTime = startTime
	listCtx.ExitCode = 137
	assert.Equal(t, IncludeAction, IncludeContainer(listCtx, contain))

	// test network name
	listCtx.Filters = filters.NewArgs()
	listCtx.Filters.Add("network", "bridge")
	assert.Equal(t, IncludeAction, IncludeContainer(listCtx, contain))
	listCtx.Filters.Add("network", "fooNet")
	assert.Equal(t, IncludeAction, IncludeContainer(listCtx, contain))
	listCtx.Filters.Del("network", "bridge")
	listCtx.Filters.Del("network", "fooNet")
	listCtx.Filters.Add("network", "barNet")
	assert.Equal(t, ExcludeAction, IncludeContainer(listCtx, contain))

	listCtx.Filters = filters.NewArgs()
	listCtx.Filters.Add("network", "missed")
	assert.Equal(t, ExcludeAction, IncludeContainer(listCtx, contain))

	// test volume name
	listCtx.Filters = filters.NewArgs()
	listCtx.Filters.Add("volume", "fooVol")
	assert.Equal(t, IncludeAction, IncludeContainer(listCtx, contain))
	listCtx.Filters.Add("volume", "barVol")
	assert.Equal(t, IncludeAction, IncludeContainer(listCtx, contain))
	listCtx.Filters.Del("volume", "fooVol")
	listCtx.Filters.Del("volume", "barVol")
	listCtx.Filters.Add("volume", "quxVol")
	assert.Equal(t, ExcludeAction, IncludeContainer(listCtx, contain))

	// test volume and network filters together
	listCtx.Filters = filters.NewArgs()
	listCtx.Filters.Add("volume", "fooVol")
	listCtx.Filters.Add("network", "bridge")
	assert.Equal(t, IncludeAction, IncludeContainer(listCtx, contain))
	listCtx.Filters.Add("volume", "barVol")
	listCtx.Filters.Add("network", "fooNet")
	assert.Equal(t, IncludeAction, IncludeContainer(listCtx, contain))
	listCtx.Filters.Del("volume", "fooVol")
	assert.Equal(t, ExcludeAction, IncludeContainer(listCtx, contain))

	listCtx.Filters = filters.NewArgs()
	listCtx.Filters.Add("status", "stopped")
	assert.Equal(t, ExcludeAction, IncludeContainer(listCtx, contain))
}
