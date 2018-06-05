// Copyright 2016 VMware, Inc. All Rights Reserved.
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

package parser

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// Manifest is a golang representation of the manifest json
type Manifest struct {
	Version      int
	Dependencies []ManifestEntry
}

func ParseManifest(input []byte) (Manifest, error) {
	m := Manifest{}
	err := json.Unmarshal(input, &m)
	return m, err
}

// ManifestByRepo is like Manifest, except that only the Dependencies for a specific repo are included
type ManifestByRepo struct {
	Repository   string
	Dependencies []ManifestEntry
}

func ParseManifestByRepo(input []byte) ([]ManifestByRepo, error) {
	m := []ManifestByRepo{}
	err := json.Unmarshal(input, &m)
	return m, err
}

type SortedManifestByRepo []ManifestByRepo

func (slice SortedManifestByRepo) Len() int {
	return len(slice)
}

func (slice SortedManifestByRepo) Less(i, j int) bool {
	return slice[i].Repository < slice[j].Repository
}

func (slice SortedManifestByRepo) Swap(i, j int) {
	slice[i], slice[j] = slice[j], slice[i]
}

// ManifestEntry includes the fields from the original manifest plus some additional fields:
//   HasTags - does the repository have tags
//   RevisionTag - is there a tag corresponding to this revision
//   SuggestedTag - if there is no tag, the most recent tag is listed
//   SuggestedRev - if a tag is suggested, the revision is also suggested
type ManifestEntry struct {
	Importpath   string
	Repository   string
	Vcs          string
	Revision     string
	HasTags      bool
	RevisionTag  string
	SuggestedTag string
	SuggestedRev string
	Branch       string
}

// TagData is the golang representation of the json returned from Github
type TagData struct {
	Name   string
	Commit TagDataCommit
}

// TagDataCommit is the golang representation of the json returned from Github
type TagDataCommit struct {
	Sha string
	Url string
}

// UntaggedException is the golang representation of a documented exception to revision not being tagged
type UntaggedException struct {
	Revision string
	Reason   string
}

func ReadFromStdin() ([]byte, error) {
	bio := bufio.NewReader(os.Stdin)
	input, err := bio.ReadBytes(127) // Read all input to EOF
	if err != io.EOF {
		return nil, fmt.Errorf("Failed to read input: %v\n", err)
	}
	return input, nil
}
