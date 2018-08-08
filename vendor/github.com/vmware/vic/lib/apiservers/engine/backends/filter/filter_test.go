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

func TestFilterCommon(t *testing.T) {
	cmdFilters := filters.NewArgs()
	id := "123"
	before := "456"

	fContext := &FilterContext{
		ID: id,
	}

	// no common filter
	assert.Equal(t, IncludeAction, filterCommon(fContext, cmdFilters))

	// exclude on Name
	cmdFilters.Add("name", "jojo")
	assert.Equal(t, ExcludeAction, filterCommon(fContext, cmdFilters))

	// exclude on ID
	cmdFilters.Add("id", before)
	assert.Equal(t, ExcludeAction, filterCommon(fContext, cmdFilters))

	// we've hit the before id exclude object
	fContext.ID = before
	fContext.BeforeID = &before
	cmdFilters.Add("before", before)
	assert.Equal(t, ExcludeAction, filterCommon(fContext, cmdFilters))

	// stop due to since
	since := "859"
	fContext.SinceID = &since
	fContext.ID = since
	cmdFilters.Add("since", since)
	assert.Equal(t, StopAction, filterCommon(fContext, cmdFilters))

	// exclude based on label mismatch
	fContext.Labels = createLabels()
	fContext.ID = id
	cmdFilters.Add("label", "joe")
	assert.Equal(t, ExcludeAction, filterCommon(fContext, cmdFilters))
}

func createLabels() map[string]string {
	labels := make(map[string]string)
	labels["prod"] = "ATX"
	labels["brown"] = "fox"
	return labels
}
