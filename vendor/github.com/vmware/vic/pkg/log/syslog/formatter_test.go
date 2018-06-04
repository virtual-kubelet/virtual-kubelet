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
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewFormatter(t *testing.T) {
	f := newFormatter("", RFC3164)
	assert.IsType(t, &localFormatter{}, f)

	f = newFormatter("tcp", RFC3164)
	assert.IsType(t, &rfc3164Formatter{}, f)

	f = newFormatter("tcp", 123)
	assert.Nil(t, f)
}

func TestFormatterFormats(t *testing.T) {
	var tests = []struct {
		format   string
		tsLayout string
		local    bool
		f        formatter
	}{
		{"<%d>%s %s[%d]: %s", time.Stamp, true, &localFormatter{}},
		{"<%d>%s %s %s[%d]: %s", time.RFC3339, false, &rfc3164Formatter{}},
	}

	for _, te := range tests {
		ts := time.Now()
		if te.local {
			assert.Equal(t, fmt.Sprintf(te.format, priority, ts.Format(te.tsLayout), tag, os.Getpid(), "foo"), te.f.Format(priority, ts, "host", tag, "foo"))
		} else {
			assert.Equal(t, fmt.Sprintf(te.format, priority, ts.Format(te.tsLayout), "host", tag, os.Getpid(), "foo"), te.f.Format(priority, ts, "host", tag, "foo"))
		}
	}
}
