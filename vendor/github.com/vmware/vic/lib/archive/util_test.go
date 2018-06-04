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
	"testing"

	"github.com/stretchr/testify/assert"
)

// INPUT VALIDATION TESTS

func TestNilSpecForAddInclusionExclusion(t *testing.T) {
	var emptyMounts []string
	err := AddMountInclusionsExclusions("", nil, emptyMounts, "")
	assert.Error(t, err)

	emptySpec := FilterSpec{
		RebasePath: "",
		StripPath:  "",
		Exclusions: nil,
		Inclusions: nil,
	}

	err = AddMountInclusionsExclusions("", &emptySpec, emptyMounts, "")
	assert.Error(t, err)
}

// COPY TO TESTS

func TestComplexWriteSpec(t *testing.T) {
	copyTarget := "/mnt/vols"
	direction := CopyTo

	mounts := []testMount{

		{
			mount:              "/",
			CopyTarget:         copyTarget,
			primaryMountTarget: true,
			direction:          direction,
		},
		{
			mount:              "/mnt/vols/A",
			CopyTarget:         copyTarget,
			primaryMountTarget: false,
			direction:          direction,
		},
		{
			mount:              "/mnt/vols/B",
			CopyTarget:         copyTarget,
			primaryMountTarget: false,
			direction:          direction,
		},
		{
			mount:              "/mnt/vols/A/subvols/AB",
			CopyTarget:         copyTarget,
			primaryMountTarget: false,
			direction:          direction,
		},
	}

	expectedFilterSpecs := map[string]FilterSpec{
		"/": {
			RebasePath: "mnt/vols",
			StripPath:  "",
			Exclusions: make(map[string]struct{}),
			Inclusions: make(map[string]struct{}),
		},
		"/mnt/vols/A": {
			RebasePath: "",
			StripPath:  "A",
			Exclusions: make(map[string]struct{}),
			Inclusions: make(map[string]struct{}),
		},
		"/mnt/vols/B": {
			RebasePath: "",
			StripPath:  "B",
			Exclusions: make(map[string]struct{}),
			Inclusions: make(map[string]struct{}),
		},
		"/mnt/vols/A/subvols/AB": {
			RebasePath: "",
			StripPath:  "A/subvols/AB",
			Exclusions: make(map[string]struct{}),
			Inclusions: make(map[string]struct{}),
		},
	}

	assertStripRebase(t, mounts, expectedFilterSpecs)

}

func TestWriteIntoMountSpec(t *testing.T) {
	copyTarget := "/mnt/vols/A/a/path"
	direction := CopyTo

	mounts := []testMount{
		// "/" will exist but since it is past the write path we do not care about the filterspec. It will be filled out as a non primary target in any case. and is a non primary target that comes before the primary target.
		{
			mount:              "/",
			CopyTarget:         copyTarget,
			primaryMountTarget: false,
			direction:          direction,
		},
		{
			mount:              "/mnt/vols/A",
			CopyTarget:         copyTarget,
			primaryMountTarget: true,
			direction:          direction,
		},
	}

	expectedFilterSpecs := map[string]FilterSpec{
		// not a valid spec since this is past the copy target. The filterspec will be completely bogus. The generateFilterSpec function assumes you have given it a target that lives along the CopyTarget. In our case here we have "/" as the mount point and the target as "/mnt/vols/A/a/path"
		"/": {
			RebasePath: "",
			StripPath:  "",
			Exclusions: make(map[string]struct{}),
			Inclusions: make(map[string]struct{}),
		},
		"/mnt/vols/A": {
			RebasePath: "a/path",
			StripPath:  "",
			Exclusions: make(map[string]struct{}),
			Inclusions: make(map[string]struct{}),
		},
	}

	assertStripRebase(t, mounts, expectedFilterSpecs)

}

func TestWriteFileIntoMountSpec(t *testing.T) {
	copyTarget := "/mnt/vols/A/a/path/file.txt"
	direction := CopyTo

	mounts := []testMount{
		{
			mount:              "/mnt/vols/A",
			CopyTarget:         copyTarget,
			primaryMountTarget: true,
			direction:          direction,
		},
	}

	expectedFilterSpecs := map[string]FilterSpec{
		"/mnt/vols/A": {
			RebasePath: "a/path/file.txt",
			StripPath:  "",
			Exclusions: make(map[string]struct{}),
			Inclusions: make(map[string]struct{}),
		},
	}

	assertStripRebase(t, mounts, expectedFilterSpecs)

}

func TestWriteFilespec(t *testing.T) {
	copyTarget := "/path/file.txt"
	direction := CopyTo

	mounts := []testMount{
		{
			mount:              "/",
			CopyTarget:         copyTarget,
			primaryMountTarget: true,
			direction:          direction,
		},
	}

	expectedFilterSpecs := map[string]FilterSpec{
		"/": {
			RebasePath: "path/file.txt",
			StripPath:  "",
			Exclusions: make(map[string]struct{}),
			Inclusions: make(map[string]struct{}),
		},
	}

	assertStripRebase(t, mounts, expectedFilterSpecs)

}

// COPY FROM TESTS

func TestComplexReadOfRootSpec(t *testing.T) {
	copyTarget := "/"
	direction := CopyFrom

	mounts := []testMount{

		{
			mount:              "/",
			CopyTarget:         copyTarget,
			primaryMountTarget: true,
			direction:          direction,
		},
		{
			mount:              "/mnt/vols/A",
			CopyTarget:         copyTarget,
			primaryMountTarget: false,
			direction:          direction,
		},
		{
			mount:              "/mnt/vols/B",
			CopyTarget:         copyTarget,
			primaryMountTarget: false,
			direction:          direction,
		},
		{
			mount:              "/mnt/vols/A/subvols/AB",
			CopyTarget:         copyTarget,
			primaryMountTarget: false,
			direction:          direction,
		},
	}

	expectedFilterSpecs := map[string]FilterSpec{
		"/": {
			RebasePath: "",
			StripPath:  "",
			Exclusions: make(map[string]struct{}),
			Inclusions: make(map[string]struct{}),
		},
		"/mnt/vols/A": {
			RebasePath: "mnt/vols/A",
			StripPath:  "",
			Exclusions: make(map[string]struct{}),
			Inclusions: make(map[string]struct{}),
		},
		"/mnt/vols/B": {
			RebasePath: "mnt/vols/B",
			StripPath:  "",
			Exclusions: make(map[string]struct{}),
			Inclusions: make(map[string]struct{}),
		},
		"/mnt/vols/A/subvols/AB": {
			RebasePath: "mnt/vols/A/subvols/AB",
			StripPath:  "",
			Exclusions: make(map[string]struct{}),
			Inclusions: make(map[string]struct{}),
		},
	}

	assertStripRebase(t, mounts, expectedFilterSpecs)

}

func TestComplexReadSpec(t *testing.T) {
	copyTarget := "/mnt/vols"
	direction := CopyFrom

	mounts := []testMount{

		{
			mount:              "/",
			CopyTarget:         copyTarget,
			primaryMountTarget: true,
			direction:          direction,
		},
		{
			mount:              "/mnt/vols/A",
			CopyTarget:         copyTarget,
			primaryMountTarget: false,
			direction:          direction,
		},
		{
			mount:              "/mnt/vols/B",
			CopyTarget:         copyTarget,
			primaryMountTarget: false,
			direction:          direction,
		},
		{
			mount:              "/mnt/vols/A/subvols/AB",
			CopyTarget:         copyTarget,
			primaryMountTarget: false,
			direction:          direction,
		},
	}

	expectedFilterSpecs := map[string]FilterSpec{
		"/": {
			RebasePath: "vols",
			StripPath:  "mnt/vols",
			Exclusions: make(map[string]struct{}),
			Inclusions: make(map[string]struct{}),
		},
		"/mnt/vols/A": {
			RebasePath: "vols/A",
			StripPath:  "",
			Exclusions: make(map[string]struct{}),
			Inclusions: make(map[string]struct{}),
		},
		"/mnt/vols/B": {
			RebasePath: "vols/B",
			StripPath:  "",
			Exclusions: make(map[string]struct{}),
			Inclusions: make(map[string]struct{}),
		},
		"/mnt/vols/A/subvols/AB": {
			RebasePath: "vols/A/subvols/AB",
			StripPath:  "",
			Exclusions: make(map[string]struct{}),
			Inclusions: make(map[string]struct{}),
		},
	}

	assertStripRebase(t, mounts, expectedFilterSpecs)

}

func TestReadDirectorySlashdot(t *testing.T) {
	copyTarget := "/mnt/vols/."
	direction := CopyFrom

	mounts := []testMount{

		{
			mount:              "/",
			CopyTarget:         copyTarget,
			primaryMountTarget: true,
			direction:          direction,
		},
		{
			mount:              "/mnt/vols/A",
			CopyTarget:         copyTarget,
			primaryMountTarget: false,
			direction:          direction,
		},
	}

	expectedFilterSpecs := map[string]FilterSpec{
		"/": {
			RebasePath: "",
			StripPath:  "mnt/vols",
			Exclusions: make(map[string]struct{}),
			Inclusions: make(map[string]struct{}),
		},
		"/mnt/vols/A": {
			RebasePath: "A",
			StripPath:  "",
			Exclusions: make(map[string]struct{}),
			Inclusions: make(map[string]struct{}),
		},
	}

	assertStripRebase(t, mounts, expectedFilterSpecs)

}

func TestComplexReadWithEndingSlashSpec(t *testing.T) {
	copyTarget := "/mnt/vols/"
	direction := CopyFrom

	mounts := []testMount{

		{
			mount:              "/",
			CopyTarget:         copyTarget,
			primaryMountTarget: true,
			direction:          direction,
		},
		{
			mount:              "/mnt/vols/A",
			CopyTarget:         copyTarget,
			primaryMountTarget: false,
			direction:          direction,
		},
		{
			mount:              "/mnt/vols/B",
			CopyTarget:         copyTarget,
			primaryMountTarget: false,
			direction:          direction,
		},
		{
			mount:              "/mnt/vols/A/subvols/AB",
			CopyTarget:         copyTarget,
			primaryMountTarget: false,
			direction:          direction,
		},
	}

	expectedFilterSpecs := map[string]FilterSpec{
		"/": {
			RebasePath: "vols",
			StripPath:  "mnt/vols",
			Exclusions: make(map[string]struct{}),
			Inclusions: make(map[string]struct{}),
		},
		"/mnt/vols/A": {
			RebasePath: "vols/A",
			StripPath:  "",
			Exclusions: make(map[string]struct{}),
			Inclusions: make(map[string]struct{}),
		},
		"/mnt/vols/B": {
			RebasePath: "vols/B",
			StripPath:  "",
			Exclusions: make(map[string]struct{}),
			Inclusions: make(map[string]struct{}),
		},
		"/mnt/vols/A/subvols/AB": {
			RebasePath: "vols/A/subvols/AB",
			StripPath:  "",
			Exclusions: make(map[string]struct{}),
			Inclusions: make(map[string]struct{}),
		},
	}

	assertStripRebase(t, mounts, expectedFilterSpecs)

}

func TestReadIntoMountSpec(t *testing.T) {
	copyTarget := "/mnt/vols/A/a/path"
	direction := CopyFrom

	mounts := []testMount{
		{
			mount:              "/mnt/vols/A",
			CopyTarget:         copyTarget,
			primaryMountTarget: true,
			direction:          direction,
		},
	}

	expectedFilterSpecs := map[string]FilterSpec{
		"/mnt/vols/A": {
			RebasePath: "path",
			StripPath:  "a/path",
			Exclusions: make(map[string]struct{}),
			Inclusions: make(map[string]struct{}),
		},
	}

	assertStripRebase(t, mounts, expectedFilterSpecs)

}

func TestReadLevelOneDirpec(t *testing.T) {
	copyTarget := "/mnt/"
	direction := CopyFrom

	mounts := []testMount{
		{
			mount:              "/",
			CopyTarget:         copyTarget,
			primaryMountTarget: true,
			direction:          direction,
		},
	}

	expectedFilterSpecs := map[string]FilterSpec{
		"/": {
			RebasePath: "mnt",
			StripPath:  "mnt",
			Exclusions: make(map[string]struct{}),
			Inclusions: make(map[string]struct{}),
		},
	}

	assertStripRebase(t, mounts, expectedFilterSpecs)

}

func TestReadLevelOneNoTrailingSlash(t *testing.T) {
	copyTarget := "/mnt"
	direction := CopyFrom

	mounts := []testMount{
		{
			mount:              "/",
			CopyTarget:         copyTarget,
			primaryMountTarget: true,
			direction:          direction,
		},
	}

	expectedFilterSpecs := map[string]FilterSpec{
		"/": {
			RebasePath: "mnt",
			StripPath:  "mnt",
			Exclusions: make(map[string]struct{}),
			Inclusions: make(map[string]struct{}),
		},
	}

	assertStripRebase(t, mounts, expectedFilterSpecs)

}

func TestReadFileSpec(t *testing.T) {
	copyTarget := "/mnt/vols/A/a/path/file.txt"
	direction := CopyFrom

	mounts := []testMount{
		// "/" will exist but since it is past the write path we do not care about the filterspec. It will be filled out as a non primary target in any case.
		{
			mount:              "/",
			CopyTarget:         copyTarget,
			primaryMountTarget: true,
			direction:          direction,
		},
	}

	expectedFilterSpecs := map[string]FilterSpec{
		// not since this is past the copy target the filterspec will be completely bogus. The generateFilterSpec function assumes you have given it a target that lives along the CopyTarget. In our case here we have "/" as the mount point and the target as "/mnt/vols/A/a/path"
		"/": {
			RebasePath: "file.txt",
			StripPath:  "mnt/vols/A/a/path/file.txt",
			Exclusions: make(map[string]struct{}),
			Inclusions: make(map[string]struct{}),
		},
	}

	assertStripRebase(t, mounts, expectedFilterSpecs)

}

func TestReadFileInMountSpec(t *testing.T) {
	copyTarget := "/mnt/vols/A/a/path/file.txt"
	direction := CopyFrom

	mounts := []testMount{
		{
			mount:              "/mnt/vols/A",
			CopyTarget:         copyTarget,
			primaryMountTarget: true,
			direction:          direction,
		},
	}

	expectedFilterSpecs := map[string]FilterSpec{
		"/mnt/vols/A": {
			RebasePath: "file.txt",
			StripPath:  "a/path/file.txt",
			Exclusions: make(map[string]struct{}),
			Inclusions: make(map[string]struct{}),
		},
	}

	assertStripRebase(t, mounts, expectedFilterSpecs)

}

func TestReadAtMountBoundarySpec(t *testing.T) {
	copyTarget := "/mnt/vols/A"
	direction := CopyFrom

	mounts := []testMount{
		{
			mount:              "/mnt/vols/A",
			CopyTarget:         copyTarget,
			primaryMountTarget: true,
			direction:          direction,
		},
	}

	expectedFilterSpecs := map[string]FilterSpec{
		"/mnt/vols/A": {
			RebasePath: "A",
			StripPath:  "",
			Exclusions: make(map[string]struct{}),
			Inclusions: make(map[string]struct{}),
		},
	}

	assertStripRebase(t, mounts, expectedFilterSpecs)

}

// ADD INCLUSIONS/EXCLUSIONS TESTS

func TestAddExclusionsNonNested(t *testing.T) {
	copyTarget := "/mnt"
	testMounts := []string{
		"/",
		"/mnt/A",
		"/mnt/B",
		"/mnt/C",
	}

	expectedResults := map[string]FilterSpec{
		"/": {
			Exclusions: map[string]struct{}{
				"":       {},
				"mnt/A/": {},
				"mnt/B/": {},
				"mnt/C/": {},
			},
			Inclusions: map[string]struct{}{
				"mnt": {},
			},
		},
		"/mnt/A": {
			Exclusions: make(map[string]struct{}),
			Inclusions: map[string]struct{}{
				"": {},
			},
		},
		"/mnt/B": {
			Exclusions: make(map[string]struct{}),
			Inclusions: map[string]struct{}{
				"": {},
			},
		},
		"/mnt/C": {
			Exclusions: make(map[string]struct{}),
			Inclusions: map[string]struct{}{
				"": {},
			},
		},
	}

	for _, mount := range testMounts {
		spec := FilterSpec{
			Exclusions: make(map[string]struct{}),
			Inclusions: make(map[string]struct{}),
		}

		err := AddMountInclusionsExclusions(mount, &spec, testMounts, copyTarget)

		if !assert.Nil(t, err) {
			return
		}

		for k := range expectedResults[mount].Inclusions {
			_, ok := spec.Inclusions[k]

			if !assert.True(t, ok, "did not find expected entry (%s) in inclusions map for mount (%s). ", k, mount) {
				return
			}
		}

		expectedSpec := expectedResults[mount]
		expectedLength := len(expectedSpec.Exclusions)
		actualLength := len(spec.Exclusions)
		if !assert.Equal(t, expectedLength, actualLength, "there were %d entries instead of %d for the exclusions generated for mount (%s)", actualLength, expectedLength, mount) {
			return
		}

		for path := range expectedSpec.Exclusions {
			_, ok := spec.Exclusions[path]

			if !assert.True(t, ok, "Should have found (%s) in the exclusion map for %s", path, mount) {
				return
			}

		}

	}

}

func TestAddExclusionsTrailingSlash(t *testing.T) {
	copyTarget := "/mnt/"
	testMounts := []string{
		"/",
	}

	expectedResults := map[string]FilterSpec{
		"/": {
			Exclusions: map[string]struct{}{
				"": {},
			},
			Inclusions: map[string]struct{}{
				"mnt": {},
			},
		},
	}

	for _, mount := range testMounts {
		spec := FilterSpec{
			Exclusions: make(map[string]struct{}),
			Inclusions: make(map[string]struct{}),
		}

		err := AddMountInclusionsExclusions(mount, &spec, testMounts, copyTarget)

		if !assert.Nil(t, err) {
			return
		}

		for k := range expectedResults[mount].Inclusions {
			_, ok := spec.Inclusions[k]

			if !assert.True(t, ok, "did not find expected entry (%s) in inclusions map for mount (%s). ", k, mount) {
				return
			}
		}

		expectedSpec := expectedResults[mount]
		expectedLength := len(expectedSpec.Exclusions)
		actualLength := len(spec.Exclusions)
		if !assert.Equal(t, expectedLength, actualLength, "there were %d entries instead of %d for the exclusions generated for mount (%s)", actualLength, expectedLength, mount) {
			return
		}

		for path := range expectedSpec.Exclusions {
			_, ok := spec.Exclusions[path]

			if !assert.True(t, ok, "Should have found (%s) in the exclusion map for %s", path, mount) {
				return
			}

		}

	}

}

func TestAddExclusionsTrailingSlashdot(t *testing.T) {
	copyTarget := "/mnt/."
	testMounts := []string{
		"/",
		"/mnt/A",
		"/mnt/B",
		"/mnt/C",
		"/mnt/A/dir/AB",
		"/mnt/A/dir/AC",
	}

	expectedResults := map[string]FilterSpec{
		"/": {
			Exclusions: map[string]struct{}{
				"":              {},
				"mnt/A/":        {},
				"mnt/B/":        {},
				"mnt/C/":        {},
				"mnt/A/dir/AB/": {},
				"mnt/A/dir/AC/": {},
			},
			Inclusions: map[string]struct{}{
				"mnt/": {},
			},
		},
		"/mnt/A": {
			Exclusions: map[string]struct{}{
				"dir/AB/": {},
				"dir/AC/": {},
			},
			Inclusions: map[string]struct{}{
				"": {},
			},
		},
		"/mnt/B": {
			Exclusions: make(map[string]struct{}),
			Inclusions: map[string]struct{}{
				"": {},
			},
		},
		"/mnt/C": {
			Exclusions: make(map[string]struct{}),
			Inclusions: map[string]struct{}{
				"": {},
			},
		},
		"/mnt/A/dir/AB": {
			Exclusions: make(map[string]struct{}),
			Inclusions: map[string]struct{}{
				"": {},
			},
		},
		"/mnt/A/dir/AC": {
			Exclusions: make(map[string]struct{}),
			Inclusions: map[string]struct{}{
				"": {},
			},
		},
	}

	for _, mount := range testMounts {
		spec := FilterSpec{
			Exclusions: make(map[string]struct{}),
			Inclusions: make(map[string]struct{}),
		}

		err := AddMountInclusionsExclusions(mount, &spec, testMounts, copyTarget)

		if !assert.Nil(t, err) {
			return
		}

		expectedSpec := expectedResults[mount]
		expectedLength := len(expectedSpec.Exclusions)
		actualLength := len(spec.Exclusions)
		if !assert.Equal(t, expectedLength, actualLength, "there were %d entries instead of %d for the exclusions generated for mount (%s)", actualLength, expectedLength, mount) {
			fmt.Printf("Expected exclusions: %+v, actual: %+v", expectedSpec.Exclusions, spec.Exclusions)
			return
		}

		expectedSpec = expectedResults[mount]
		expectedLength = len(expectedSpec.Inclusions)
		actualLength = len(spec.Inclusions)
		if !assert.Equal(t, expectedLength, actualLength, "there were %d entries instead of %d for the inclusions generated for mount (%s)", actualLength, expectedLength, mount) {
			fmt.Printf("Expected inclusions: %+v, actual: %+v", expectedSpec.Inclusions, spec.Inclusions)
			return
		}

		for path := range expectedSpec.Exclusions {
			_, ok := spec.Exclusions[path]

			if !assert.True(t, ok, "Should have found (%s) in the exclusion map for %s \n exclusion map: %s", path, mount, spec.Exclusions) {
				return
			}

		}

		for path := range expectedSpec.Inclusions {
			_, ok := spec.Inclusions[path]

			if !assert.True(t, ok, "Should have found (%s) in the inclusion map for %s \n inclusion map: %s", path, mount, spec.Exclusions) {
				return
			}

		}

	}

}

func TestAddExclusionsNestedMounts(t *testing.T) {
	copyTarget := "/mnt"
	testMounts := []string{
		"/",
		"/mnt/A",
		"/mnt/B",
		"/mnt/C",
		"/mnt/A/dir/AB",
		"/mnt/A/dir/AC",
	}

	expectedResults := map[string]FilterSpec{
		"/": {
			Exclusions: map[string]struct{}{
				"":              {},
				"mnt/A/":        {},
				"mnt/B/":        {},
				"mnt/C/":        {},
				"mnt/A/dir/AB/": {},
				"mnt/A/dir/AC/": {},
			},
			Inclusions: map[string]struct{}{
				"mnt": {},
			},
		},
		"/mnt/A": {
			Exclusions: map[string]struct{}{
				"dir/AB/": {},
				"dir/AC/": {},
			},
			Inclusions: map[string]struct{}{
				"": {},
			},
		},
		"/mnt/B": {
			Exclusions: make(map[string]struct{}),
			Inclusions: map[string]struct{}{
				"": {},
			},
		},
		"/mnt/C": {
			Exclusions: make(map[string]struct{}),
			Inclusions: map[string]struct{}{
				"": {},
			},
		},
		"/mnt/A/dir/AB": {
			Exclusions: make(map[string]struct{}),
			Inclusions: map[string]struct{}{
				"": {},
			},
		},
		"/mnt/A/dir/AC": {
			Exclusions: make(map[string]struct{}),
			Inclusions: map[string]struct{}{
				"": {},
			},
		},
	}

	for _, mount := range testMounts {
		spec := FilterSpec{
			Exclusions: make(map[string]struct{}),
			Inclusions: make(map[string]struct{}),
		}

		err := AddMountInclusionsExclusions(mount, &spec, testMounts, copyTarget)

		if !assert.Nil(t, err) {
			return
		}

		expectedSpec := expectedResults[mount]
		expectedLength := len(expectedSpec.Exclusions)
		actualLength := len(spec.Exclusions)
		if !assert.Equal(t, expectedLength, actualLength, "there were %d entries instead of %d for the exclusions generated for mount (%s)", actualLength, expectedLength, mount) {
			return
		}

		expectedSpec = expectedResults[mount]
		expectedLength = len(expectedSpec.Inclusions)
		actualLength = len(spec.Inclusions)
		if !assert.Equal(t, expectedLength, actualLength, "there were %d entries instead of %d for the inclusions generated for mount (%s)", actualLength, expectedLength, mount) {
			return
		}

		for path := range expectedSpec.Exclusions {
			_, ok := spec.Exclusions[path]

			if !assert.True(t, ok, "Should have found (%s) in the exclusion map for %s \n exclusion map: %s", path, mount, spec.Exclusions) {
				return
			}

		}

		for path := range expectedSpec.Inclusions {
			_, ok := spec.Inclusions[path]

			if !assert.True(t, ok, "Should have found (%s) in the inclusion map for %s \n inclusion map: %s", path, mount, spec.Exclusions) {
				return
			}

		}

	}

}

func TestAddExclusionsMountTarget(t *testing.T) {
	copyTarget := "/mnt/vols/A"
	testMounts := []string{
		"/",
		"/mnt/vols/A",
	}

	expectedResults := map[string]FilterSpec{
		"/mnt/vols/A": {
			Exclusions: make(map[string]struct{}),
			Inclusions: map[string]struct{}{
				"": {},
			},
		},
	}

	for _, mount := range testMounts {

		if mount == "/" {
			continue
		}

		spec := FilterSpec{
			Exclusions: make(map[string]struct{}),
			Inclusions: make(map[string]struct{}),
		}

		err := AddMountInclusionsExclusions(mount, &spec, testMounts, copyTarget)

		if !assert.Nil(t, err) {
			return
		}

		expectedSpec := expectedResults[mount]
		expectedLength := len(expectedSpec.Exclusions)
		actualLength := len(spec.Exclusions)
		if !assert.Equal(t, expectedLength, actualLength, "there were %d entries instead of %d for the exclusions generated for mount (%s)", actualLength, expectedLength, mount) {
			return
		}

		expectedSpec = expectedResults[mount]
		expectedLength = len(expectedSpec.Inclusions)
		actualLength = len(spec.Inclusions)
		if !assert.Equal(t, expectedLength, actualLength, "there were %d entries instead of %d for the inclusions generated for mount (%s)", actualLength, expectedLength, mount) {
			return
		}

		for path := range expectedSpec.Exclusions {
			_, ok := spec.Exclusions[path]

			if !assert.True(t, ok, "Should have found (%s) in the exclusion map for %s \n exclusion map: %s", path, mount, spec.Exclusions) {
				return
			}

		}

		for path := range expectedSpec.Inclusions {
			_, ok := spec.Inclusions[path]

			if !assert.True(t, ok, "Should have found (%s) in the inclusion map for %s \n inclusion map: %s", path, mount, spec.Exclusions) {
				return
			}

		}
	}

}

func TestAddExclusionsRootTarget(t *testing.T) {
	copyTarget := "/"
	testMounts := []string{
		"/",
		"/mnt/vols/A",
	}

	expectedResults := map[string]FilterSpec{
		"/": {
			Exclusions: map[string]struct{}{
				"mnt/vols/A/": {},
			},
			Inclusions: map[string]struct{}{
				"": {},
			},
		},
		"/mnt/vols/A": {
			Exclusions: make(map[string]struct{}),
			Inclusions: map[string]struct{}{
				"": {},
			},
		},
	}

	for _, mount := range testMounts {

		if mount == "/" {
			continue
		}

		spec := FilterSpec{
			Exclusions: make(map[string]struct{}),
			Inclusions: make(map[string]struct{}),
		}

		err := AddMountInclusionsExclusions(mount, &spec, testMounts, copyTarget)

		if !assert.Nil(t, err) {
			return
		}

		expectedSpec := expectedResults[mount]
		expectedLength := len(expectedSpec.Exclusions)
		actualLength := len(spec.Exclusions)
		if !assert.Equal(t, expectedLength, actualLength, "there were %d entries instead of %d for the exclusions generated for mount (%s)", actualLength, expectedLength, mount) {
			return
		}

		expectedSpec = expectedResults[mount]
		expectedLength = len(expectedSpec.Inclusions)
		actualLength = len(spec.Inclusions)
		if !assert.Equal(t, expectedLength, actualLength, "there were %d entries instead of %d for the inclusions generated for mount (%s)", actualLength, expectedLength, mount) {
			return
		}

		for path := range expectedSpec.Exclusions {
			_, ok := spec.Exclusions[path]

			if !assert.True(t, ok, "Should have found (%s) in the exclusion map for %s \n exclusion map: %s", path, mount, spec.Exclusions) {
				return
			}

		}

		for path := range expectedSpec.Inclusions {
			_, ok := spec.Inclusions[path]

			if !assert.True(t, ok, "Should have found (%s) in the inclusion map for %s \n inclusion map: %s", path, mount, spec.Exclusions) {
				return
			}

		}
	}

}

// test utility functions and structs

type testMount struct {
	mount              string
	CopyTarget         string
	primaryMountTarget bool
	direction          bool
}

func assertStripRebase(t *testing.T, mounts []testMount, expectedFilterSpecs map[string]FilterSpec) {
	for _, v := range mounts {
		actualFilterSpec := GenerateFilterSpec(v.CopyTarget, v.mount, v.primaryMountTarget, v.direction)
		expectedFilterSpec := expectedFilterSpecs[v.mount]

		if !assert.Equal(t, expectedFilterSpec.RebasePath, actualFilterSpec.RebasePath, "rebase path check failed (%s)", v.mount) {
			return
		}

		if !assert.Equal(t, expectedFilterSpec.StripPath, actualFilterSpec.StripPath, "strip path check failed (%s)", v.mount) {
			return
		}

	}
}
