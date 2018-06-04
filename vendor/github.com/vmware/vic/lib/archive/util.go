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

package archive

import (
	"fmt"
	"path/filepath"
	"strings"
)

const (
	// CopyTo is used to indicate that the desired filter spec is for a CopyTo direction
	CopyTo = true

	// CopyFrom is used to indicate that the desired filter spec is for the CopyFrom direction
	CopyFrom = false
)

// GenerateFilterSpec will populate the appropriate relative Rebase and Strip paths based on the supplied scenarios.
// Inclusion/Exclusion should be constructed separately. Please also note that any mount that exists before the copy
// target that is not primary and comes before the primary target will have a bogus filterspec, since it would not be
// written or read to.
func GenerateFilterSpec(copyPath string, mountPoint string, primaryTarget bool, direction bool) FilterSpec {
	var filter FilterSpec

	// NOTE: this solidifies that this function is very heavily designed for docker cp behavior.
	// which is why this should mainly only be used by the docker personality
	// scrub trailing '/' before passing copyPath along and create a Clean fs path
	copyPath = strings.TrimSuffix(copyPath, "/")

	// Note I know they are just booleans, if that changes then this statement will not need to.
	if direction == CopyTo {
		filter = generateCopyToFilterSpec(copyPath, mountPoint, primaryTarget)
	} else {
		filter = generateCopyFromFilterSpec(copyPath, mountPoint, primaryTarget)
	}

	filter.Exclusions = make(map[string]struct{})
	filter.Inclusions = make(map[string]struct{})
	return filter
}

func generateCopyFromFilterSpec(copyPath string, mountPoint string, primaryTarget bool) FilterSpec {
	var filter FilterSpec

	// If the copyPath ends with '/.', we don't want the content's directory name
	// to be included in the resulting tar header. we do this by removing the directory
	// portion of the rebase path.
	// if the copyPath does not end with '/.', we only need the right most element of copyPath.
	first := filepath.Base(copyPath)
	if first == "." {
		first = ""
		copyPath = Clean(copyPath, false)
	}

	// primary target was provided so we wil need to split the target and take the right most element for the rebase.
	// then strip set the strip as the target path.
	if primaryTarget {
		filter.RebasePath = Clean(first, true)
		filter.StripPath = Clean(strings.TrimPrefix(copyPath, mountPoint), true)
		return filter
	}

	// non primary target was provided. in this case we will need rebase to include the right most member of the target(or "/") joined to the front of the mountPath - the target path. 3
	filter.RebasePath = Clean(filepath.Join(first, strings.TrimPrefix(mountPoint, copyPath)), true)
	filter.StripPath = ""
	return filter
}

func generateCopyToFilterSpec(copyPath string, mountPoint string, primaryTarget bool) FilterSpec {
	var filter FilterSpec

	// primary target was provided so we will need to rebase header assets for this mount to have the target in front for the write.
	if primaryTarget {
		filter.RebasePath = Clean(strings.TrimPrefix(copyPath, mountPoint), true)
		filter.StripPath = ""
		return filter
	}

	// non primary target, this implies that the asset header has part of the mount point path in it. We must strip out that part since the non primary target will be mounted and be looking at the world from it's own root "/"
	filter.RebasePath = ""
	filter.StripPath = Clean(strings.TrimPrefix(mountPoint, copyPath), true)

	return filter
}

func AddMountInclusionsExclusions(currentMount string, filter *FilterSpec, mounts []string, copyTarget string) error {
	if filter == nil {
		return fmt.Errorf("filterSpec for (%s) was nil, cannot add exclusions or inclusions", currentMount)
	}

	if filter.Exclusions == nil || filter.Inclusions == nil {
		return fmt.Errorf("either the inclusions or exclusions map was nil for (%s)", currentMount)
	}

	if strings.HasPrefix(copyTarget, currentMount) && copyTarget != currentMount {

		inclusion := Clean(strings.TrimPrefix(copyTarget, currentMount), true)

		if filepath.Base(copyTarget) == "." {
			inclusion += "/"
		}

		filter.Inclusions[inclusion] = struct{}{}
		filter.Exclusions[""] = struct{}{}
	} else {
		// this would be a mount that is after the target. It would mean we have to include root. then exclude any mounts after root.
		filter.Inclusions[""] = struct{}{}
	}

	for _, mount := range mounts {
		if strings.HasPrefix(mount, currentMount) && currentMount != mount {
			// exclusions are relative to the mount so the leading `/` should be removed unless we decide otherwise.
			exclusion := Clean(strings.TrimPrefix(mount, currentMount), true) + "/"
			filter.Exclusions[exclusion] = struct{}{}
		}
	}
	return nil
}

// Clean run filepath.Clean on the target and will remove leading
// and trailing slashes from the target path corresponding to supplied booleans
func Clean(path string, leading bool) string {
	path = filepath.Clean(path)
	// path returns '.' if the result of the Clean was an empty string.
	if path == "." {
		return ""
	}

	if leading {
		path = strings.TrimPrefix(path, "/")
	}

	return path
}
