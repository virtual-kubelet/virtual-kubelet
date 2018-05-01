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

package syslog

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	network  = "tcp"
	raddr    = "localhost:514"
	tag      = "test"
	priority = Info | Daemon
)

func TestMakeTag(t *testing.T) {
	p := filepath.Base(os.Args[0])
	var tests = []struct {
		prefix string
		proc   string
		out    string
	}{
		{
			prefix: "",
			proc:   "",
			out:    p,
		},
		{
			prefix: "",
			proc:   "foo",
			out:    "foo",
		},
		{
			prefix: "foo",
			proc:   "",
			out:    "foo" + sep + p,
		},
		{
			prefix: "bar",
			proc:   "foo",
			out:    "bar" + sep + "foo",
		},
	}

	for _, te := range tests {
		out := MakeTag(te.prefix, te.proc)
		assert.Equal(t, te.out, out)
	}
}

func TestDefaultDialerBadPriority(t *testing.T) {
	d := &defaultDialer{
		priority: -1,
	}

	w, err := d.dial()
	assert.Nil(t, w)
	assert.Error(t, err)

	d.priority = (Local7 | Debug) + 1
	w, err = d.dial()
	assert.Nil(t, w)
	assert.Error(t, err)
}
