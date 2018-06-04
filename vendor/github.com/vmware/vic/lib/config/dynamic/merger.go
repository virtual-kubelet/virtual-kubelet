// Copyright 2016-2017 VMware, Inc. All Rights Reserved.
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

package dynamic

import (
	"fmt"

	"github.com/vmware/vic/lib/config"
	"github.com/vmware/vic/pkg/registry"
)

type merger struct {
}

func NewMerger() Merger {
	return &merger{}
}

// Merge merges two config objects together. For now only
// whitelist registries are merged.
func (m *merger) Merge(orig, other *config.VirtualContainerHostConfigSpec) (*config.VirtualContainerHostConfigSpec, error) {
	// merge strategy:
	//
	// origWl empty, otherWl empty => empty
	//
	// origWl empty, otherWl not empty => otherWl
	//
	// origWl not empty, otherWl empty => origWl
	//
	// origWl not empty, otherWl not empty => merge result
	// in this case, each entry in the resulting
	// whitelist must be a more restrictive
	// version of at least one entry in origWl
	//
	// The whitelist that is used is always otherWl
	// in this case given that the above rule is not
	// violated.
	otherWl, err := ParseRegistries(other.RegistryWhitelist)
	if err != nil {
		return nil, err
	}

	origWl, err := ParseRegistries(orig.RegistryWhitelist)
	if err != nil {
		return nil, err
	}

	var wl registry.Set
	if wl, err = origWl.Merge(otherWl, &whitelistMerger{}); err != nil {
		return nil, err
	}

	if len(origWl) > 0 {
		// check if every entry in wl is a subset of an
		// entry in origWl
		for _, e := range wl {
			found := false
			for _, o := range origWl {
				if o.Contains(e) {
					found = true
					break
				}
			}

			if !found {
				return nil, fmt.Errorf("whitelist merge allows entries that are not in the original whitelist")
			}
		}
	}

	// only use otherWl if its non-empty
	//
	// if otherWl is empty and origWl is
	// not empty, we use origWl, which
	// should be the same as wl after the
	// merge
	if len(otherWl) > 0 {
		wl = otherWl
	}

	res := *orig
	res.RegistryWhitelist = wl.Strings()
	return &res, nil
}

func ParseRegistries(regs []string) (registry.Set, error) {
	var s registry.Set
	for _, r := range regs {
		e := registry.ParseEntry(r)
		if e != nil {
			s = append(s, e)
			continue
		}

		return nil, fmt.Errorf("could not parse entry %s", r)
	}

	return s, nil
}

type whitelistMerger struct{}

// Merge merges two registry entries. The merge fails if merging orig and other would
// broaden orig's scope. The result of the merge is other if that is more restrictive.
// if orig equals other, the result is orig.
func (w *whitelistMerger) Merge(orig, other registry.Entry) (registry.Entry, error) {
	if orig.Equal(other) {
		return orig, nil
	}

	if other.Contains(orig) {
		return nil, fmt.Errorf("merge of %s and %s would broaden %s", orig, other, orig)
	}

	// more restrictive result is OK
	if orig.Contains(other) {
		return other, nil
	}

	// no merge
	return nil, nil
}
