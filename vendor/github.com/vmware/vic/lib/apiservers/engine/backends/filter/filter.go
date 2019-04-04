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
	"github.com/docker/docker/api/types/filters"
)

// FilterAction represents possible results during filtering
type FilterAction int

const (
	IncludeAction FilterAction = iota
	ExcludeAction
	StopAction
)

// FilterContext will hold the common filter requirements
type FilterContext struct {
	// ID of object to filter
	ID string
	// Name of object to filter
	Name string
	// BeforeID is the filter to ignore objects that appear before the one given
	BeforeID *string
	// SinceID is the filter to stop iterating
	SinceID *string
	// Labels of object to filter
	Labels map[string]string
}

// filterCommon will filter the common criteria across objects
func filterCommon(filterContext *FilterContext, cmdFilters filters.Args) FilterAction {

	// have we made it to the beforeID
	if filterContext.BeforeID != nil {
		if filterContext.ID == *filterContext.BeforeID {
			filterContext.BeforeID = nil
		}
		return ExcludeAction
	}

	// Stop iteration when the object arrives to the filter object
	if filterContext.SinceID != nil {
		if filterContext.ID == *filterContext.SinceID {
			return StopAction
		}
	}

	// Do not include object if any of the labels don't match
	if !cmdFilters.MatchKVList("label", filterContext.Labels) {
		return ExcludeAction
	}

	// Do not include if the id doesn't match
	if !cmdFilters.Match("id", filterContext.ID) {
		return ExcludeAction
	}

	if !cmdFilters.Match("name", filterContext.Name) {
		return ExcludeAction
	}

	return IncludeAction
}
