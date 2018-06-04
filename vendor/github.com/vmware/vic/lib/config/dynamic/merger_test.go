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
package dynamic

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/vmware/vic/lib/config"
	"github.com/vmware/vic/pkg/registry"
)

func TestWhitelistMerger(t *testing.T) {
	var tests = []struct {
		orig, other registry.Entry
		res         registry.Entry
		err         error
	}{
		{
			orig:  registry.ParseEntry("10.10.10.10"),
			other: registry.ParseEntry("10.10.10.10"),
			res:   registry.ParseEntry("10.10.10.10"),
		},
		{
			orig:  registry.ParseEntry("10.10.10.10"),
			other: registry.ParseEntry("10.10.10.20"),
		},
		{
			orig:  registry.ParseEntry("10.10.10.10/24"),
			other: registry.ParseEntry("10.10.10.10/24"),
			res:   registry.ParseEntry("10.10.10.10/24"),
		},
		{
			other: registry.ParseEntry("10.10.10.10/24"),
			orig:  registry.ParseEntry("192.168.1.0/24"),
		},
		{
			orig:  registry.ParseEntry("10.10.10.10/24"),
			other: registry.ParseEntry("10.10.10.10/16"),
			err:   assert.AnError,
		},
		{
			orig:  registry.ParseEntry("10.10.10.10/16"),
			other: registry.ParseEntry("10.10.10.10/24"),
			res:   registry.ParseEntry("10.10.10.10/24"),
		},
		{
			orig:  registry.ParseEntry("*.google.com"),
			other: registry.ParseEntry("*.google.com"),
			res:   registry.ParseEntry("*.google.com"),
		},
		{
			orig:  registry.ParseEntry("*.yahoo.com"),
			other: registry.ParseEntry("*.google.com"),
		},
		{
			orig:  registry.ParseEntry("*.google.com"),
			other: registry.ParseEntry("mail.google.com"),
			res:   registry.ParseEntry("mail.google.com"),
		},
		{
			orig:  registry.ParseEntry("mail.google.com"),
			other: registry.ParseEntry("*.google.com"),
			err:   assert.AnError,
		},
		{
			orig:  registry.ParseEntry("192.168.1.1:123"),
			other: registry.ParseEntry("192.168.1.1"),
			err:   assert.AnError,
		},
		{
			orig:  registry.ParseEntry("192.168.1.1"),
			other: registry.ParseEntry("192.168.1.1:123"),
			res:   registry.ParseEntry("192.168.1.1"),
		},
		{
			orig:  registry.ParseEntry("foo:123"),
			other: registry.ParseEntry("foo"),
			err:   assert.AnError,
		},
		{
			orig:  registry.ParseEntry("foo"),
			other: registry.ParseEntry("foo:123"),
			res:   registry.ParseEntry("foo"),
		},
		{
			orig:  registry.ParseEntry("http://foo"),
			other: registry.ParseEntry("foo:123"),
		},
		{
			orig:  registry.ParseEntry("http://foo"),
			other: registry.ParseEntry("http://foo:123"),
		},
		{
			orig:  registry.ParseEntry("http://foo:123"),
			other: registry.ParseEntry("http://foo"),
		},
		{
			orig:  registry.ParseEntry("http://foo/bar"),
			other: registry.ParseEntry("http://foo"),
			err:   assert.AnError,
		},
		{
			orig:  registry.ParseEntry("https://foo/bar"),
			other: registry.ParseEntry("http://foo/bar"),
		},
		{
			orig:  registry.ParseEntry("https://foo"),
			other: registry.ParseEntry("foo"),
			err:   assert.AnError,
		},
	}

	m := &whitelistMerger{}

	for _, te := range tests {
		res, err := m.Merge(te.orig, te.other)
		if te.err == nil {
			assert.Nil(t, err, "case: orig: %s, other: %s, err: %v, res: %s", te.orig, te.other, te.err, te.res)
		} else {
			assert.NotNil(t, err, "case: orig: %s, other: %s, err: %v, res: %s", te.orig, te.other, te.err, te.res)
		}

		if te.res == nil {
			assert.Nil(t, res, "case: orig: %s, other: %s, err: %v, res: %s", te.orig, te.other, te.err, te.res)
		} else {
			assert.True(t, te.res.Equal(res), "%s merge %s, %s (expected) == %s (actual)", te.orig, te.other, te.res, res)
		}
	}
}

func TestMergerMergeWhitelist(t *testing.T) {
	var tests = []struct {
		orig, other, res *config.VirtualContainerHostConfigSpec
		err              error
	}{
		{
			orig: &config.VirtualContainerHostConfigSpec{
				Registry: config.Registry{
					RegistryWhitelist: []string{"docker.io"},
				},
			},
			other: &config.VirtualContainerHostConfigSpec{
				Registry: config.Registry{
					RegistryWhitelist: nil,
				},
			},
			res: &config.VirtualContainerHostConfigSpec{
				Registry: config.Registry{
					RegistryWhitelist: []string{"docker.io"},
				},
			},
		},
		// disallow the merge if the other whitelist
		// has an entry that is not allowed on the
		// original whitelist
		{
			orig: &config.VirtualContainerHostConfigSpec{
				Registry: config.Registry{
					RegistryWhitelist: []string{"docker.io"},
				},
			},
			other: &config.VirtualContainerHostConfigSpec{
				Registry: config.Registry{
					RegistryWhitelist: []string{"foo.docker.io", "malicious.io"},
				},
			},
			err: assert.AnError,
		},
		{
			orig: &config.VirtualContainerHostConfigSpec{
				Registry: config.Registry{
					RegistryWhitelist: []string{"*.docker.io"},
				},
			},
			other: &config.VirtualContainerHostConfigSpec{
				Registry: config.Registry{
					RegistryWhitelist: []string{"bar.docker.io", "foo.docker.io"},
				},
			},
			res: &config.VirtualContainerHostConfigSpec{
				Registry: config.Registry{
					RegistryWhitelist: []string{"bar.docker.io", "foo.docker.io"},
				},
			},
		},
		// result of a merge is always the other if
		// its a subset of the original
		{
			orig: &config.VirtualContainerHostConfigSpec{
				Registry: config.Registry{
					RegistryWhitelist: []string{"docker.io", "harbor.ci.local"},
				},
			},
			other: &config.VirtualContainerHostConfigSpec{
				Registry: config.Registry{
					RegistryWhitelist: []string{"docker.io"},
				},
			},
			res: &config.VirtualContainerHostConfigSpec{
				Registry: config.Registry{
					RegistryWhitelist: []string{"docker.io"},
				},
			},
		},
		// empty original whitelist and non-empty other
		// whitelist results in other whitelist
		{
			orig: &config.VirtualContainerHostConfigSpec{
				Registry: config.Registry{
					RegistryWhitelist: nil,
				},
			},
			other: &config.VirtualContainerHostConfigSpec{
				Registry: config.Registry{
					RegistryWhitelist: []string{"docker.io"},
				},
			},
			res: &config.VirtualContainerHostConfigSpec{
				Registry: config.Registry{
					RegistryWhitelist: []string{"docker.io"},
				},
			},
		},
		// more permissive other whitelist results in error
		{
			orig: &config.VirtualContainerHostConfigSpec{
				Registry: config.Registry{
					RegistryWhitelist: []string{"foo.docker.io"},
				},
			},
			other: &config.VirtualContainerHostConfigSpec{
				Registry: config.Registry{
					RegistryWhitelist: []string{"*.docker.io"},
				},
			},
			err: assert.AnError,
		},
		// less permissive other whitelist results in other whitelist
		{
			orig: &config.VirtualContainerHostConfigSpec{
				Registry: config.Registry{
					RegistryWhitelist: []string{"*.docker.io"},
				},
			},
			other: &config.VirtualContainerHostConfigSpec{
				Registry: config.Registry{
					RegistryWhitelist: []string{"foo.docker.io"},
				},
			},
			res: &config.VirtualContainerHostConfigSpec{
				Registry: config.Registry{
					RegistryWhitelist: []string{"foo.docker.io"},
				},
			},
		},
	}

	m := NewMerger()
	for _, te := range tests {
		t.Logf("orig: %+v, other: %+v, err: %+v", te.orig.RegistryWhitelist, te.other.RegistryWhitelist, te.err)
		if te.res != nil {
			t.Logf("res: %+v", te.res.RegistryWhitelist)
		}
		res, err := m.Merge(te.orig, te.other)
		if te.err != nil {
			assert.NotNil(t, err, "expected error, got nil")
			assert.Nil(t, res)
			continue
		}

		assert.Nil(t, err, "expected no error, got \"%+v\"", err)
		assert.NotNil(t, res)
		assert.Len(t, res.RegistryWhitelist, len(te.res.RegistryWhitelist), "expected %v, got %v", te.res.RegistryWhitelist, res.RegistryWhitelist)
		for i := range res.RegistryWhitelist {
			found := false
			for j := range te.res.RegistryWhitelist {
				if res.RegistryWhitelist[i] == te.res.RegistryWhitelist[j] {
					found = true
					break
				}
			}

			assert.True(t, found, "expected whitelist %v, got %v", te.res.RegistryWhitelist, res.RegistryWhitelist)
			if !found {
				t.FailNow()
			}
		}
	}
}
