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

/*
* ValidateFilters will evalute the provided filters against the docker acceptedFilters and
* the filters vic currently doesn't support (unSupportedFilters)
 */
func ValidateFilters(cmdFilters filters.Args, acceptedFilters map[string]bool, unSupportedFilters map[string]bool) error {

	var err error

	// ensure provided filter args are accepted
	if err = cmdFilters.Validate(acceptedFilters); err != nil {
		return err
	}

	// verify that vic supports the provided filter args
	// will only make it here if all the filters are valid
	if err = validateSupport(cmdFilters, unSupportedFilters); err != nil {
		return err
	}

	return err
}

/*
*	validateSupport will ensure the provided filter arguments are implemented
*  	by vic
 */
func validateSupport(cmdFilters filters.Args, unSupported map[string]bool) error {

	for filter := range unSupported {
		vals := cmdFilters.Get(filter)
		if len(vals) > 0 {
			return fmt.Errorf("filter %s is not currently supported by vic", filter)
		}
	}

	return nil
}
