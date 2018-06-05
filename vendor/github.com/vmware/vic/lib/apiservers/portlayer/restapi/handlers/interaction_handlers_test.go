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

package handlers

import (
	"io"
	"strings"
	"testing"

	log "github.com/Sirupsen/logrus"
)

func init() {
	log.SetLevel(log.DebugLevel)
}

// A ChunkReader reads C bytes from R in each Read
type ChunkReader struct {
	R io.Reader
	C int64
}

func NewChunkReader(r io.Reader, c int64) io.Reader {
	return &ChunkReader{r, c}
}

func (l *ChunkReader) Read(p []byte) (n int, err error) {
	return l.R.Read(p[:l.C])
}

func TestNewFlushingReaderWithInitBytes(t *testing.T) {

	var tests = []struct {
		in  string
		err error
	}{
		{attachStdinInitString, nil},
		{attachStdinInitString + "# uname -a", nil},
		{"@^" + attachStdinInitString + "# uname -a", nil},
		{attachStdinInitString[:2], io.EOF},
	}

	for _, test := range tests {
		for i := 1; i < len(test.in)+1; i++ {
			buf := make([]byte, 64)

			lr := NewChunkReader(strings.NewReader(test.in), int64(i))

			f := NewFlushingReaderWithInitBytes(lr, []byte(attachStdinInitString))
			_, err := f.readDetectInit(buf)
			if err != test.err {
				t.Error(err)
			}
		}
	}
}

func TestNewFlushingReaderWithInitBytesPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("The function did not panic")
		}
	}()

	// pass a smaller buffer to cause it to panic
	str := attachStdinInitString
	var buf []byte

	lr := NewChunkReader(strings.NewReader(str), 1)
	f := NewFlushingReaderWithInitBytes(lr, []byte(attachStdinInitString))
	f.readDetectInit(buf)
}
