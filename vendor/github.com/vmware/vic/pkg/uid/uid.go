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

package uid

import (
	"regexp"

	"github.com/docker/docker/pkg/stringid"
)

// UID is a unique id
type UID string

// NilUID is a placeholder for an empty ID
const NilUID UID = UID("")

var (
	idRegex      = regexp.MustCompile("^[0-9a-f]{64}$")
	shortIDRegex = regexp.MustCompile("^[0-9a-f]{12}$")
)

// New generates a UID
func New() UID {
	return Parse(stringid.GenerateNonCryptoID())
}

// Parse converts a string to UID
func Parse(u string) UID {
	if idRegex.MatchString(u) || shortIDRegex.MatchString(u) {
		return UID(u)
	}

	return NilUID
}

// Truncate returns the truncated UID
func (u UID) Truncate() UID {
	return Parse(stringid.TruncateID(string(u)))
}

// String converts a UID to a string
func (u UID) String() string {
	return string(u)
}
