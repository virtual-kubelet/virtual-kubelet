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

package registry

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestSetMerge(t *testing.T) {
	var tests = []struct {
		orig, other Set
		res         Set
		err         error
	}{
		{},
		{
			orig:  nil,
			other: Set{ParseEntry("192.168.0.1")},
			res:   Set{ParseEntry("192.168.0.1")},
		},
		{
			orig:  Set{ParseEntry("192.168.0.1")},
			other: nil,
			res:   Set{ParseEntry("192.168.0.1")},
		},
		{
			orig:  Set{ParseEntry("192.168.0.1")},
			other: Set{ParseEntry("192.168.0.1")},
			res:   Set{ParseEntry("192.168.0.1")},
		},
		{
			orig:  Set{ParseEntry("192.168.0.1")},
			other: Set{ParseEntry("192.168.0.2")},
			res: Set{
				ParseEntry("192.168.0.1"),
				ParseEntry("192.168.0.2"),
			},
		},
		{
			orig: Set{
				ParseEntry("192.168.0.1"),
				ParseEntry("192.168.0.2"),
			},
			other: Set{
				ParseEntry("192.168.0.2"),
			},
			res: Set{
				ParseEntry("192.168.0.1"),
				ParseEntry("192.168.0.2"),
			},
		},
		{
			orig: Set{
				ParseEntry("192.168.0.1"),
				ParseEntry("192.168.0.2"),
			},
			other: Set{
				ParseEntry("192.168.0.3"),
			},
			res: Set{
				ParseEntry("192.168.0.1"),
				ParseEntry("192.168.0.2"),
				ParseEntry("192.168.0.3"),
			},
		},
	}

	for _, te := range tests {
		res, err := te.orig.Merge(te.other, nil)
		assert.Equal(t, te.err, err)
		assert.Len(t, res, len(te.res))
		for _, r := range res {
			found := false
			for _, r2 := range te.res {
				if r2.String() == r.String() {
					found = true
					break
				}
			}

			assert.True(t, found)
		}
	}
}

func TestSetMergeMergerError(t *testing.T) {
	m := &MockMerger{}
	m.On("Merge", mock.Anything, mock.Anything).Return(nil, assert.AnError)

	s := Set{ParseEntry("192.168.0.1")}
	other := Set{ParseEntry("192.168.0.1")}
	res, err := s.Merge(other, m)
	assert.Nil(t, res)
	assert.Error(t, err)
	assert.EqualValues(t, assert.AnError, err)
}

func TestSetMatch(t *testing.T) {
	e1 := &MockEntry{}
	e1.On("Match", mock.Anything).Return(false)
	e2 := &MockEntry{}
	e2.On("Match", mock.Anything).Return(true)

	s := Set{e1, e2}
	assert.True(t, s.Match("foo"))

	s = Set{e2}
	assert.True(t, s.Match("foo"))

	s = Set{e1}
	assert.False(t, s.Match("foo"))
}
