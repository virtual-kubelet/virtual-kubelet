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

import "testing"
import "github.com/stretchr/testify/assert"

func TestParse(t *testing.T) {
	// valid ids
	var tests = []string{
		"abcdef01234567890123456789abcdefabcdef01234567890123456789abcdef",
		"abcdefabcdef", // short id
	}

	for _, te := range tests {
		id := Parse(te)
		assert.NotEqual(t, id, NilUID)
		assert.Equal(t, te, id.String())
	}

	// invalid ids
	tests = []string{
		"foobar",
		"",
		"abcde",
		"abcdefe",
		"abcdef01234567890123456789abcdefabcdef01234567890123456789abcdefe",
		"abcdef01234567890123456789abcdefabcdef01234567890123456789abcde",
	}

	for _, te := range tests {
		id := Parse(te)
		assert.Equal(t, id, NilUID)
	}

}

func TestTruncate(t *testing.T) {
	var tests = []struct {
		in, out UID
	}{
		{Parse("abcdef01234567890123456789abcdefabcdef01234567890123456789abcdef"), Parse("abcdef012345")},
		{Parse("abcdefabcdef"), Parse("abcdefabcdef")},
		{NilUID, NilUID},
	}

	for _, te := range tests {
		assert.Equal(t, te.out, te.in.Truncate())
	}

}

func TestNew(t *testing.T) {
	assert.NotEqual(t, NilUID, New())
}
