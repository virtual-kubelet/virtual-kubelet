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

package extraconfig

import (
	"testing"

	"strings"

	"github.com/stretchr/testify/assert"
)

func visibleRO(key string) string {
	return calculateKey(calculateScope([]string{ReadOnlyScope}), "", key)
}

func visibleRONonpersistent(key string) string {
	return calculateKey(calculateScope([]string{ReadOnlyScope, NonPersistentSuffix}), "", key)
}

func visibleRW(key string) string {
	return calculateKey(calculateScope([]string{ReadWriteScope}), "", key)
}

func hidden(key string) string {
	return calculateKey(calculateScope([]string{HiddenScope}), "", key)
}

func TestHidden(t *testing.T) {
	scopes := []string{HiddenScope}

	key := calculateKey(calculateScope(scopes), "a/b", "c")

	assert.Equal(t, "a/b/c", key, "Key should remain hidden")
}

func TestHide(t *testing.T) {
	scopes := []string{HiddenScope}

	key := calculateKey(calculateScope(scopes), defaultGuestInfoPrefix()+"/a/b", "c")

	assert.Equal(t, "a/b/c", key, "Key should be hidden")
}

func TestReveal(t *testing.T) {
	scopes := []string{ReadOnlyScope}

	key := calculateKey(calculateScope(scopes), "a/b", "c")

	assert.Equal(t, defaultGuestInfoPrefix()+"/a/b/c", key, "Key should be exposed")
}

func TestVisibleReadOnly(t *testing.T) {
	scopes := []string{ReadOnlyScope}

	key := calculateKey(calculateScope(scopes), defaultGuestInfoPrefix()+"/a/b", "c")

	assert.Equal(t, defaultGuestInfoPrefix()+"/a/b/c", key, "Key should be remain visible and read-only")
}

func TestVisibleReadWrite(t *testing.T) {
	scopes := []string{ReadWriteScope}

	key := calculateKey(calculateScope(scopes), defaultGuestInfoPrefix()+".a.b", "c")

	assert.Equal(t, defaultGuestInfoPrefix()+".a.b.c", key, "Key should be remain visible and read-write")
}

func TestTopLevelReadOnly(t *testing.T) {
	scopes := []string{ReadOnlyScope}

	key := calculateKey(calculateScope(scopes), "", "a")

	assert.Equal(t, defaultGuestInfoPrefix()+"/a", key, "Key should be visible and read-only")
}

func TestReadOnlyToReadWrite(t *testing.T) {
	scopes := []string{ReadWriteScope}

	key := calculateKey(calculateScope(scopes), defaultGuestInfoPrefix()+"/a/b", "c")

	assert.Equal(t, defaultGuestInfoPrefix()+".a.b.c", key, "Key should be visible and change to read-write")
}

func TestReadWriteToReadOnly(t *testing.T) {
	scopes := []string{ReadOnlyScope}

	key := calculateKey(calculateScope(scopes), defaultGuestInfoPrefix()+".a.b", "c")

	assert.Equal(t, defaultGuestInfoPrefix()+"/a/b/c", key, "Key should be visible and change to read-only")
}

func TestCompoundKey(t *testing.T) {
	scopes := []string{ReadWriteScope}

	key := calculateKey(calculateScope(scopes), defaultGuestInfoPrefix()+".a", "b/c")

	assert.Equal(t, defaultGuestInfoPrefix()+".a.b.c", key, "Key should be visible and read-write")
}

func TestNoScopes(t *testing.T) {
	scopes := []string{}

	key := calculateKey(calculateScope(scopes), defaultGuestInfoPrefix()+".a/b", "c")
	assert.Equal(t, "a/b/c", key, "Key should be completely proscriptive")

	key = calculateKey(calculateScope(scopes), defaultGuestInfoPrefix()+".a.b", "c")
	assert.Equal(t, "a.b/c", key, "Key should be hidden")

	key = calculateKey(calculateScope(scopes), "a.b", "c")
	assert.Equal(t, "a.b/c", key, "Key should remain hidden")
}

func TestSecret(t *testing.T) {
	scopes := []string{SecretSuffix, ReadWriteScope}

	key := calculateKey(calculateScope(scopes), defaultGuestInfoPrefix()+".a.b", "c")

	assert.Equal(t, defaultGuestInfoPrefix()+".a.b.c"+suffixSeparator+SecretSuffix, key, "Key should have secret suffix")
}

func TestNonpersistent(t *testing.T) {
	scopes := []string{NonPersistentSuffix, ReadWriteScope}

	key := calculateKey(calculateScope(scopes), defaultGuestInfoPrefix()+".a.b", "c")

	assert.Equal(t, defaultGuestInfoPrefix()+".a.b.c"+suffixSeparator+NonPersistentSuffix, key, "Key should have non-persistent suffix")
}

func TestMultipleSuffixes(t *testing.T) {
	scopes := []string{NonPersistentSuffix, SecretSuffix, ReadWriteScope}

	key := calculateKey(calculateScope(scopes), defaultGuestInfoPrefix()+".a.b", "c")

	assert.True(t, strings.Contains(key, suffixSeparator+SecretSuffix) && strings.Contains(key, suffixSeparator+NonPersistentSuffix), "Key should contain both secret and non-persistent suffix")
}

func TestCalculateKeys(t *testing.T) {
	type AStruct struct {
		I int
	}
	type Type struct {
		ExecutorConfig ExecutorConfig `vic:"0.1" scope:"hidden" key:"executorconfig"`
		Array          []AStruct      `vic:"0.1" scope:"read-write" key:"array"`
		Ptr            *AStruct       `vic:"0.1" scope:"read-only" key:"ptr"`
		Str            string         `vic:"0.1" scope:"read-only" key:"str"`
		Bytes          []uint8        `vic:"0.1" scope:"read-write" key:"bytes"`
	}

	ec := Type{
		ExecutorConfig: ExecutorConfig{
			Sessions: map[string]SessionConfig{
				"Session1": {
					Common: Common{
						ID:   "SessionID",
						Name: "SessionName",
					},
					Tty: true,
					Cmd: Cmd{
						Path: "/vmware",
						Args: []string{"/bin/imagec", "-standalone"},
						Env:  []string{"PATH=/bin", "USER=imagec"},
						Dir:  "/",
					},
				},
			},
		},
		Array: []AStruct{
			{I: 0},
		},
		Ptr: &AStruct{
			I: 1,
		},
		Str:   "foo",
		Bytes: []byte{0xd, 0xe, 0xa, 0xd, 0xb, 0xe, 0xe, 0xf},
	}

	var tests = []struct {
		in  string
		out []string
	}{
		{
			"ExecutorConfig.*",
			[]string{
				visibleRO("executorconfig/common"),
				hidden("executorconfig/sessions"),
				"executorconfig/Key",
			},
		},
		{
			"ExecutorConfig.Sessions.*",
			[]string{"executorconfig/sessions" + Separator + "Session1"},
		},
		{
			"ExecutorConfig.Sessions.Session1.Cmd.Args",
			[]string{"executorconfig/sessions" + Separator + "Session1/cmd/args"},
		},
		{
			"ExecutorConfig.Sessions.*.Cmd.Args.*",
			[]string{"executorconfig/sessions" + Separator + "Session1/cmd/args~"},
		},
		{
			"ExecutorConfig.Sessions.*.Cmd.Args.0",
			[]string{"executorconfig/sessions" + Separator + "Session1/cmd/args~"},
		},
		{
			"Array.0.I",
			[]string{visibleRW("array" + Separator + "0/I")},
		},
		{
			"Array.*",
			[]string{visibleRW("array" + Separator + "0")},
		},
		{
			"Ptr.I",
			[]string{visibleRO("ptr/I")},
		},
		{
			"Str",
			[]string{visibleRO("str")},
		},
		{
			"Bytes",
			[]string{visibleRW("bytes")},
		},
		{
			"Bytes.0",
			[]string{visibleRW("bytes")},
		},
		{
			"Bytes.*",
			[]string{visibleRW("bytes")},
		},
	}

	for _, te := range tests {
		keys := CalculateKeys(ec, te.in, "")
		assert.Equal(t, te.out, keys)
	}

	panicTests := []string{
		"Array.1.I",
		"Array.0.i",
		"Array.f.i",
		"ExecutorConfig.foo",
		"foo",
		"ExecutorConfig.Sessions.foo",
		"Str.*",
		"Str.foo",
	}

	for _, te := range panicTests {
		assert.Panics(t, func() {
			CalculateKeys(ec, te, "")
		})
	}
}
