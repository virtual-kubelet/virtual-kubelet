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
	"path"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types/filters"

	"github.com/vmware/vic/lib/apiservers/engine/backends/cache"
)

type ImageListContext struct {
	FilterContext

	// Tags for an image filtered by reference
	Tags []string
	// Digests for an image filtered by reference
	Digests []string
}

/*
* ValidateImageFilters will validate the image filters are
* valid docker filters / values and supported by vic.
*
* The function will reuse dockers filter validation
*
 */
func ValidateImageFilters(cmdFilters filters.Args, acceptedFilters map[string]bool, unSupportedFilters map[string]bool) (*ImageListContext, error) {

	// ensure filter options are valid and supported by vic
	if err := ValidateFilters(cmdFilters, acceptedFilters, unSupportedFilters); err != nil {
		return nil, err
	}

	// return value
	imgFilterContext := &ImageListContext{}

	err := cmdFilters.WalkValues("before", func(value string) error {
		before, err := cache.ImageCache().Get(value)
		if before == nil {
			err = fmt.Errorf("No such image: %s", value)
		} else {
			imgFilterContext.BeforeID = &before.ImageID
		}
		return err
	})
	if err != nil {
		return nil, err
	}

	err = cmdFilters.WalkValues("since", func(value string) error {
		since, err := cache.ImageCache().Get(value)
		if since == nil {
			err = fmt.Errorf("No such image: %s", value)
		} else {
			imgFilterContext.SinceID = &since.ImageID
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	return imgFilterContext, nil

}

/*
*	IncludeImage will evaluate the filter criteria in filterContext against the provided
* 	image and determine what action to take.  There are three options:
*		* IncludeAction
*		* ExcludeAction
*		* StopAction
*
 */
func IncludeImage(imgFilters filters.Args, listContext *ImageListContext) FilterAction {

	// filter common requirements
	act := filterCommon(&listContext.FilterContext, imgFilters)
	if act != IncludeAction {
		return act
	}

	// filter on image reference
	if imgFilters.Include("reference") {
		// references for this imageID
		refs := cache.RepositoryCache().References(listContext.ID)
		// reference filters
		refFilters := imgFilters.Get("reference")

		// reset the tags / digests
		listContext.Tags = nil
		listContext.Digests = nil

		// iterate of reporsitory references and filters
		for _, ref := range refs {
			for _, rf := range refFilters {
				// match on complete ref ie. busybox:latest
				// #nosec: Errors unhandled.
				matchRef, _ := path.Match(rf, ref.String())
				// match on repo only ie. busybox
				// #nosec: Errors unhandled.
				matchName, _ := path.Match(rf, ref.Name())
				// if either matched then add to tag / digest
				if matchRef || matchName {
					if _, ok := ref.(reference.Canonical); ok {
						listContext.Digests = append(listContext.Digests, ref.String())
					}
					if _, ok := ref.(reference.NamedTagged); ok {
						listContext.Tags = append(listContext.Tags, ref.String())
					}
				}
			}
		}
		// if there were no reference matches then exclude the image
		if len(listContext.Tags) == 0 && len(listContext.Digests) == 0 {
			return ExcludeAction
		}

	}
	return IncludeAction
}
