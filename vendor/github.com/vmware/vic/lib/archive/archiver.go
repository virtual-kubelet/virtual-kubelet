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

package archive

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/vmware/vic/pkg/trace"
)

// FilterType specifies what type of filter to apply in a FilterSpec
type FilterType int

const (
	// Include specifies a path inclusion.
	// Note: An included path that sits under an excluded path should be included.
	// Example: if / is excluded, and /files is included, /files should  be added to the archive.
	Include = iota
	// Exclude specifies a path exclusion.
	Exclude
	// Rebase specifies a path rebase.
	// The rebase path is prepended to the path in the archive header.
	// for an Export this will ensure proper headers on the way out.
	// for an Import it will ensure that the tar unpacking occurs in
	// right location
	Rebase
	// Strip specifies a path strip.
	// The inverse of a rebase, the path is stripped from the header path
	// before writing to disk.
	Strip
)

// FilterSpec describes rules for handling specified paths during archival
type FilterSpec struct {
	Inclusions map[string]struct{}
	Exclusions map[string]struct{}
	RebasePath string
	StripPath  string
}

// Archiver defines an API for creating archives consisting of data that
// exist between a child and parent layer, as well as unpacking archives
// to container filesystems
type Archiver interface {

	// Export reads the delta between child and ancestor layers, returning
	// the difference as a tar archive.
	//
	// store - the store containing the two layers
	// id - must inherit from ancestor if ancestor is specified
	// ancestor - the old layer against which to diff. If omitted, the entire filesystem will be included
	// spec - describes filters on paths found in the data (include, exclude, rebase, strip)
	// data - include file data in the tar archive if true, headers only otherwise
	Export(op trace.Operation, store *url.URL, id, ancestor string, spec *FilterSpec, data bool) (io.ReadCloser, error)

	// Import will process the input tar stream based on the supplied path spec and write the stream to the
	// target device.
	//
	// store - the device store containing the target device
	// id - the id for the device that is contained within the store
	// spec - describes filters on paths found in the data (include, exclude, rebase, strip)
	// tarstream - the tar stream that is to be written to the target on the device
	Import(op trace.Operation, store *url.URL, id string, spec *FilterSpec, tarstream io.ReadCloser) error
}

// CreateFilterSpec creates a FilterSpec from a supplied map
func CreateFilterSpec(op trace.Operation, spec map[string]FilterType) (*FilterSpec, error) {
	fs := &FilterSpec{
		Inclusions: make(map[string]struct{}),
		Exclusions: make(map[string]struct{}),
	}

	if spec == nil {
		return fs, nil
	}

	for k, v := range spec {
		switch v {
		case Include:
			fs.Inclusions[k] = struct{}{}
		case Exclude:
			fs.Exclusions[k] = struct{}{}
		case Rebase:
			if fs.RebasePath != "" {
				return nil, fmt.Errorf("error creating filter spec: only one rebase path allowed")
			}
			fs.RebasePath = k
		case Strip:
			if fs.StripPath != "" {
				return nil, fmt.Errorf("error creating filter spec: only one strip path allowed")
			}
			fs.StripPath = k
		default:
			return nil, fmt.Errorf("invalid filter specification: %d", v)
		}
	}

	return fs, nil
}

// Decodes a base64 encoded string from EncodeFilterSpec into a FilterSpec
func DecodeFilterSpec(op trace.Operation, spec *string) (*FilterSpec, error) {
	var filterSpec FilterSpec

	// empty spec means don't apply any filtering
	if spec != nil && len(*spec) > 0 {
		decoded, err := base64.StdEncoding.DecodeString(*spec)
		if err != nil {
			op.Errorf("Unable to decode filter spec: %s", err)
			return nil, err
		}
		op.Debugf("decoded spec: %+s", string(decoded))

		if len(decoded) > 0 {
			if err = json.Unmarshal(decoded, &filterSpec); err != nil {
				op.Errorf("Unable to unmarshal decoded spec: %s", err)
				return nil, err
			}
		}
	}

	// normalize empty spec
	if filterSpec.Inclusions == nil {
		op.Debugf("Empty inclusion set")
		filterSpec.Inclusions = make(map[string]struct{})
	}
	if filterSpec.Exclusions == nil {
		op.Debugf("Empty exclusion set")
		filterSpec.Exclusions = make(map[string]struct{})
	}

	return &filterSpec, nil
}

// Encode the filter spec
func EncodeFilterSpec(op trace.Operation, spec *FilterSpec) (*string, error) {
	mashalled, err := json.Marshal(spec)
	if err != nil {
		op.Errorf("Unable to encode filter spec: %s", err)
		return nil, err
	}

	encoded := base64.StdEncoding.EncodeToString(mashalled)
	op.Debugf("encodedFilter = %s", encoded)

	return &encoded, nil
}

// Excludes returns true if the provided filter excludes the provided filepath
// If the spec is completely empty it will include everything.
// If an inclusion is set, but not exclusion, then we'll only return matches for the inclusions.
func (spec *FilterSpec) Excludes(op trace.Operation, filePath string) bool {
	if spec == nil {
		return false
	}

	inclusionLength := -1
	exclusionLength := -1

	for path := range spec.Inclusions {
		if strings.HasPrefix(filePath, path) {
			if len(path) > inclusionLength {
				// more specific inclusion, so update
				inclusionLength = len(path)
			}
		}
	}

	for path := range spec.Exclusions {
		if strings.HasPrefix(filePath, path) {
			if len(path) > exclusionLength {
				// more specific exclusion, so update
				exclusionLength = len(path)
			}
		}
	}

	return inclusionLength < exclusionLength
}
