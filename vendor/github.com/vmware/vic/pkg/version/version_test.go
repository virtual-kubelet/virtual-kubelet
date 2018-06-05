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

package version

import (
	"errors"
	"testing"
)

var (
	a = &Build{
		Version:     "v1.2.3",
		GitCommit:   "aaaaaaa",
		BuildDate:   "2009/11/10@23:00:00",
		BuildNumber: "10",
		State:       "",
	}

	b = &Build{
		Version:     "v1.2.3",
		GitCommit:   "bbbbbbb",
		BuildDate:   "2009/11/10@23:00:01",
		BuildNumber: "10",
		State:       "",
	}

	c = &Build{
		Version:     "v1.2.4",
		GitCommit:   "aaaaaaa",
		BuildDate:   "2009/11/10@23:00:00",
		BuildNumber: "10",
		State:       "",
	}

	d = &Build{
		Version:     "v1.2.3",
		GitCommit:   "aaaaaaa",
		BuildDate:   "2009/11/10@23:00:00",
		BuildNumber: "11",
		State:       "",
	}

	e = &Build{
		Version:     "v1.2.3",
		GitCommit:   "aaaaaaa",
		BuildDate:   "2009/11/10@23:00:00",
		BuildNumber: "",
		State:       "",
	}

	f = &Build{
		Version:     "v1.2.3",
		GitCommit:   "aaaaaaa",
		BuildDate:   "2009/11/10@23:00:00",
		BuildNumber: "wow",
		State:       "",
	}
)

func TestEqual(t *testing.T) {
	var tests = []struct {
		b1, b2   *Build
		expected bool
	}{
		{a, a, true},
		{a, b, true},
		{a, c, true},
		{a, d, false},
	}

	for _, te := range tests {
		res := te.b1.Equal(te.b2)
		if res != te.expected {
			t.Errorf("%s %s Got: %t Expected: %t", te.b1, te.b2, res, te.expected)
		}
	}
}

func TestIsOlder(t *testing.T) {
	var tests = []struct {
		b1, b2      *Build
		expected    bool
		expectedErr error
	}{
		{a, a, false, nil},
		{a, b, false, nil},
		{a, c, false, nil},
		{a, d, true, nil},
		{a, e, false, errors.New("")},
		{a, f, false, errors.New("")},
	}

	for _, te := range tests {
		res, err := te.b1.IsOlder(te.b2)
		if te.expectedErr != nil {
			if err == nil {
				t.Errorf("%s %s Got error: %s Expected error: %s", te.b1, te.b2, err, te.expectedErr)
			}
		}

		if res != te.expected {
			t.Errorf("%s %s Got: %t Expected: %t", te.b1, te.b2, res, te.expected)
		}
	}
}

func TestIsNewer(t *testing.T) {
	var tests = []struct {
		b1, b2      *Build
		expected    bool
		expectedErr error
	}{
		{a, a, false, nil},
		{a, b, false, nil},
		{b, a, false, nil},
		{a, c, false, nil},
		{c, a, false, nil},
		{a, d, false, nil},
		{d, a, true, nil},
		{a, e, false, errors.New("")},
		{a, f, false, errors.New("")},
	}

	for _, te := range tests {
		res, err := te.b1.IsNewer(te.b2)
		if te.expectedErr != nil {
			if err == nil {
				t.Errorf("%s %s Got error: %s Expected error: %s", te.b1, te.b2, err, te.expectedErr)
			}
		}

		if res != te.expected {
			t.Errorf("%s %s Got: %t Expected: %t", te.b1, te.b2, res, te.expected)
		}
	}
}

func TestUserAgent(t *testing.T) {
	for _, v := range []string{"0.0.1", "v0.0.1"} {
		Version = v

		a := UserAgent("foo")
		if a != "foo/0.0.1" {
			t.Error(a)
		}
	}

}
