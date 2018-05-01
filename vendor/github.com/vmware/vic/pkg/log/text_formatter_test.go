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

package log

import (
	"bufio"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func BenchmarkFormatNonEmpty(b *testing.B) {
	f := NewTextFormatter()
	e := &logrus.Entry{
		Time:    time.Now(),
		Level:   logrus.InfoLevel,
		Message: "the quick brown fox jumps over the lazy dog",
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f.Format(e)
	}
}

func BenchmarkFormatEmpty(b *testing.B) {
	f := NewTextFormatter()
	e := &logrus.Entry{
		Time:  time.Now(),
		Level: logrus.InfoLevel,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f.Format(e)
	}
}

func TestFormatEmpty(t *testing.T) {
	ti := time.Now()
	e := &logrus.Entry{Time: ti, Level: logrus.InfoLevel}
	f := NewTextFormatter()

	b, err := f.Format(e)
	assert.NoError(t, err)
	assert.Equal(t, fmt.Sprintf("%s %s \n", ti.Format(f.TimestampFormat), levelToString(e.Level)), string(b))
}

func TestFormatNonEmpty(t *testing.T) {
	ti := time.Now()
	m := "foo bar baz"
	e := &logrus.Entry{Time: ti, Level: logrus.InfoLevel, Message: m}
	f := NewTextFormatter()

	b, err := f.Format(e)
	assert.NoError(t, err)
	assert.Equal(t, fmt.Sprintf("%s %s %s\n", ti.Format(f.TimestampFormat), levelToString(e.Level), m), string(b))

	// test with multiple lines
	pre := fmt.Sprintf("%s %s ", ti.Format(f.TimestampFormat), levelToString(e.Level))
	var tests = []struct {
		in  string
		out []string
	}{
		{
			"foo",
			[]string{
				pre + "foo",
			},
		},
		{
			"\n",
			[]string{
				pre,
				"",
			},
		},
		{
			"foo\n",
			[]string{
				pre + "foo",
				"",
			},
		},
		{
			"\nfoo\n",
			[]string{
				pre + "",
				"foo",
				"",
			},
		},
		{
			"foo\n",
			[]string{
				pre + "foo",
				"",
			},
		},
		{
			"foo \nbar\n baz ",
			[]string{
				pre + "foo ",
				"bar",
				" baz ",
			},
		},
	}

	for idx, te := range tests {
		e.Message = te.in
		b, err = f.Format(e)
		assert.NoError(t, err)
		s := bufio.NewScanner(strings.NewReader(string(b)))
		i := 0
		for s.Scan() {
			assert.True(t, i < len(te.out), "case %d", idx)
			assert.Equal(t, te.out[i], s.Text(), "case %d", idx)
			i++
		}
	}
}
