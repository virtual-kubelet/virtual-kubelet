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

package backends

import (
	"context"
	"testing"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types"
	"github.com/stretchr/testify/assert"

	"github.com/vmware/vic/pkg/trace"
)

type MockCopyToData struct {
	containerDestPath string
	tarAssetName      string
	expectedPrefix    string
}

type ReaderFilters struct {
	rebase  string
	strip   string
	exclude []string
	include string
}
type MockCopyFromData struct {
	containerSourcePath string
	expectedPrefices    []string
	expectedFilterSpecs map[string]ReaderFilters
}

func TestFindArchiveWriter(t *testing.T) {
	mounts := []types.MountPoint{
		{Name: "volA", Destination: "/mnt/A"},
		{Name: "volAB", Destination: "/mnt/A/AB"},
		{Name: "volB", Destination: "/mnt/B"},
		{Name: "R/W", Destination: "/"},
	}

	mockData := []MockCopyToData{
		// mock data for tar asset as a file and container dest path including a mount point
		{
			containerDestPath: "/mnt/A/",
			tarAssetName:      "file.txt",
			expectedPrefix:    "/mnt/A",
		},
		{
			containerDestPath: "/mnt/A/AB",
			tarAssetName:      "file.txt",
			expectedPrefix:    "/mnt/A/AB",
		},
		// mock data for tar asset containing a mount point and the container dest path as /
		{
			containerDestPath: "/",
			tarAssetName:      "mnt/A/file.txt",
			expectedPrefix:    "/mnt/A",
		},
		{
			containerDestPath: "/",
			tarAssetName:      "mnt/A/AB/file.txt",
			expectedPrefix:    "/mnt/A/AB",
		},
		// mock data for cases that do not involve mount points
		{
			containerDestPath: "/",
			tarAssetName:      "test/file.txt",
			expectedPrefix:    "/",
		},
	}

	for _, data := range mockData {
		op := trace.NewOperation(context.Background(), "")
		writerMap := NewArchiveStreamWriterMap(op, mounts, data.containerDestPath)
		aw, err := writerMap.FindArchiveWriter(data.containerDestPath, data.tarAssetName)
		assert.Nil(t, err, "Expected success from finding archive writer for container dest %s and tar asset path %s", data.containerDestPath, data.tarAssetName)
		assert.NotNil(t, aw, "Expected non-nil archive writer")
		if aw != nil {
			assert.Contains(t, aw.mountPoint.Destination, data.expectedPrefix,
				"Expected to find prefix %s for container dest %s and tar asset path %s",
				data.expectedPrefix, data.containerDestPath, data.tarAssetName)
		}
	}
}

func TestFindArchiveReaders(t *testing.T) {
	mounts := []types.MountPoint{
		{Name: "volA", Destination: "/mnt/A"},     //mount point
		{Name: "volAB", Destination: "/mnt/A/AB"}, //mount point
		{Name: "volB", Destination: "/mnt/B"},     //mount point
		{Name: "R/W", Destination: "/"},           //container base volume
	}

	mockData := []MockCopyFromData{
		// case 1: Get all mount prefix
		{
			containerSourcePath: "/",
			expectedPrefices:    []string{"/", "/mnt/A", "/mnt/B", "/mnt/A/AB"},
			expectedFilterSpecs: map[string]ReaderFilters{
				"/": {
					rebase:  "",
					strip:   "",
					exclude: []string{"mnt/A/", "mnt/B/", "mnt/A/AB/"},
					include: "",
				},
				"/mnt/A": {
					rebase:  "mnt/A",
					strip:   "",
					exclude: []string{"AB/"},
					include: "",
				},
				"/mnt/B": {
					rebase:  "mnt/B",
					strip:   "",
					exclude: []string{},
					include: "",
				},
				"/mnt/A/AB": {
					rebase:  "mnt/A/AB",
					strip:   "",
					exclude: []string{},
					include: "",
				},
			},
		},
		{
			containerSourcePath: "/mnt",
			expectedPrefices:    []string{"/", "/mnt/A", "/mnt/B", "/mnt/A/AB"},
			expectedFilterSpecs: map[string]ReaderFilters{
				"/": {
					rebase:  "mnt",
					strip:   "mnt",
					exclude: []string{"mnt/A/", "mnt/B/", "mnt/A/AB/"},
					include: "",
				},
				"/mnt/A": {
					rebase:  "mnt/A",
					strip:   "",
					exclude: []string{"AB/"},
					include: "",
				},
				"/mnt/B": {
					rebase:  "mnt/B",
					strip:   "",
					exclude: []string{},
					include: "",
				},
				"/mnt/A/AB": {
					rebase:  "mnt/A/AB",
					strip:   "",
					exclude: []string{},
					include: "",
				},
			},
		},
		{
			containerSourcePath: "/mnt/",
			expectedPrefices:    []string{"/", "/mnt/A", "/mnt/B", "/mnt/A/AB"},
			expectedFilterSpecs: map[string]ReaderFilters{
				"/": {
					rebase:  "mnt",
					strip:   "mnt",
					exclude: []string{"mnt/A/", "mnt/B/", "mnt/A/AB/"},
					include: "",
				},
				"/mnt/A": {
					rebase:  "mnt/A",
					strip:   "",
					exclude: []string{"AB/"},
					include: "",
				},
				"/mnt/B": {
					rebase:  "mnt/B",
					strip:   "",
					exclude: []string{},
					include: "",
				},
				"/mnt/A/AB": {
					rebase:  "mnt/A/AB",
					strip:   "",
					exclude: []string{},
					include: "",
				},
			},
		},
		// case 2: Do not include /mnt/B
		{
			containerSourcePath: "/mnt/A",
			expectedPrefices:    []string{"/", "/mnt/A", "/mnt/A/AB"},
			expectedFilterSpecs: map[string]ReaderFilters{
				"/": {
					rebase:  "A",
					strip:   "mnt/A",
					exclude: []string{"mnt/A/", "mnt/A/AB/"},
					include: "/mnt/A",
				},
				"/mnt/A": {
					rebase:  "A",
					strip:   "",
					exclude: []string{"AB/"},
				},
				"/mnt/A/AB": {
					rebase:  "A/AB",
					exclude: []string{},
					include: "",
				},
			},
		},
		// case 3: Return only the container base "/"
		{
			containerSourcePath: "/mnt/not-a-mount",
			expectedPrefices:    []string{"/"},
			expectedFilterSpecs: map[string]ReaderFilters{
				"/": {
					rebase:  "not-a-mount",
					strip:   "mnt/not-a-mount",
					exclude: []string{""},
					include: "mnt/not-a-mount",
				},
			},
		},
		{
			containerSourcePath: "/etc/",
			expectedPrefices:    []string{"/"},
			expectedFilterSpecs: map[string]ReaderFilters{
				"/": {
					rebase:  "etc",
					strip:   "etc",
					exclude: []string{""},
					include: "etc",
				},
			},
		},
		// case 4: Check inclusion filter
		{
			containerSourcePath: "/mnt/A/a/file.txt",
			expectedPrefices:    []string{"/mnt/A"},
			expectedFilterSpecs: map[string]ReaderFilters{
				"/mnt/A": {
					rebase:  "file.txt",
					strip:   "a/file.txt",
					exclude: []string{""},
					include: "a/file.txt",
				},
			},
		},
	}

	for i, data := range mockData {
		op := trace.NewOperation(context.Background(), "")
		readerMap := NewArchiveStreamReaderMap(op, mounts, data.containerSourcePath)
		archiveReaders, err := readerMap.FindArchiveReaders(data.containerSourcePath)
		assert.Nil(t, err, "Expected success from finding archive readers for container source %s", data.containerSourcePath)
		assert.NotNil(t, archiveReaders, "Expected an array of archive readers but got nil for container source path %s", data.containerSourcePath)
		assert.NotEmpty(t, archiveReaders, "Expected an array of archive readers %s with more than one items", data.containerSourcePath)

		log.Debugf("Data = %#v", data)
		pa := PrefixArray(archiveReaders)
		nonOverlap := UnionMinusIntersection(pa, data.expectedPrefices)
		assert.Empty(t, nonOverlap, "Found mismatch in the prefix array and expected array for source path %s.  Non-overlapped result = %#v", data.containerSourcePath, nonOverlap)

		// Check filter spec
		for _, ar := range archiveReaders {
			currPath := ar.mountPoint.Destination
			assert.Equal(t, data.expectedFilterSpecs[currPath].rebase, ar.filterSpec.RebasePath, "rebase filterspec not correct")
			assert.Equal(t, data.expectedFilterSpecs[currPath].strip, ar.filterSpec.StripPath, "strip filterspec not correct")
			for _, ex := range data.expectedFilterSpecs[currPath].exclude {
				_, ok := ar.filterSpec.Exclusions[ex]
				assert.True(t, ok, "Did not find %s in exclusion map for reader %s in mock #%d", ex, currPath, i)
			}
		}

		// Check inclusion filter
		if len(archiveReaders) == 1 {
			ar := archiveReaders[0]
			currPath := ar.mountPoint.Destination

			expectedInclusion := data.expectedFilterSpecs[currPath].include

			assert.Len(t, ar.filterSpec.Inclusions, 1, "Expected only 1 inclusion filter for %s but got %d in mock #%d", data.containerSourcePath, len(ar.filterSpec.Inclusions), i)

			if len(ar.filterSpec.Inclusions) == 1 {
				_, ok := ar.filterSpec.Inclusions[expectedInclusion]
				assert.True(t, ok, "Expected inclusion filter to contain %s in mock #%d", expectedInclusion, i)

				// Sanity check to make sure include isn't in exclusion.  This should never happen.
				for _, ex := range data.expectedFilterSpecs[currPath].exclude {
					assert.NotEqual(t, expectedInclusion, ex, "Expected inclusion %s not to be in exclusion list %#v in mock #%d", expectedInclusion, ex, i)
				}
			}
		}
	}
}

func PrefixArray(readers []*ArchiveReader) (pa []string) {
	for _, reader := range readers {
		pa = append(pa, reader.mountPoint.Destination)
	}

	log.Debugf("prefix array - %#v", pa)
	return
}

func UnionMinusIntersection(A, B []string) (res []string) {
	test := make(map[string]bool)

	log.Debugf("Looking for non overlapping in array A-%#v and array B-%#v", A, B)

	for _, data := range A {
		test[data] = true
	}

	for _, data := range B {
		if _, ok := test[data]; ok {
			delete(test, data)
		} else {
			res = append(res, data)
		}
	}

	for key := range test {
		res = append(res, key)
	}

	log.Debugf("Resulting non overlapped array - %#v", res)

	return
}
