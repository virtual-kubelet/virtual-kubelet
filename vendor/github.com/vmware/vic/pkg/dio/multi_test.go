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

package dio

import (
	"bytes"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	count = 32

	base    = "base functionality"
	dynamic = "dynamic add/remove functionality"
)

type filterf func(idx int) bool

func FilterBuffers(in []*bytes.Buffer, fn filterf) []*bytes.Buffer {
	var out []*bytes.Buffer

	for i := 0; i < len(in); i++ {
		if fn(i) {
			out = append(out, in[i])
		}
	}
	return out
}

func FilterWriters(in []io.Writer, fn filterf) []io.Writer {
	var out []io.Writer

	for i := 0; i < len(in); i++ {
		if fn(i) {
			out = append(out, in[i])
		}
	}
	return out
}

func FilterReaders(in []io.Reader, fn filterf) []io.Reader {
	var out []io.Reader

	for i := 0; i < len(in); i++ {
		if fn(i) {
			out = append(out, in[i])
		}
	}
	return out
}

func write(t *testing.T, mwriter DynamicMultiWriter, p []byte) {
	n, err := mwriter.Write(p)
	if err != nil {
		t.Errorf("write: %v", err)
	}
	if n != len(p) {
		t.Errorf("short write: %d != %d", n, len(p))
	}
}

func read(t *testing.T, mreader DynamicMultiReader, limit int) []byte {
	total := 0

	var buf = make([]byte, 32*1024)
	for {
		n, err := mreader.Read(buf[total:limit])
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Errorf("read: %v", err)
		}

		total += n
		if total >= limit {
			break
		}
	}

	return buf[:limit]
}

func each(t *testing.T, buffers []*bytes.Buffer, s string) {
	for i := range buffers {
		assert.Equal(t, buffers[i].String(), s)
	}
}

// TestMultiWrite creates multi writers and writes to them, then removes some of them and writes again
func TestMultiWrite(t *testing.T) {
	var wg sync.WaitGroup

	var writers []io.Writer
	var buffers []*bytes.Buffer

	// create & initialize writers and buffers
	for i := 0; i < count; i++ {
		var buffer bytes.Buffer

		reader, writer := io.Pipe()

		writers = append(writers, writer)
		buffers = append(buffers, &buffer)

		wg.Add(1)
		go func() {
			// set up a goroutine so we don't block writes
			io.CopyN(&buffer, reader, int64(len(base)))

			wg.Done()
		}()
	}

	// create the multi writer
	mwriter := MultiWriter(writers...)

	// write and ensure io.Copy returns
	write(t, mwriter, []byte(base))
	wg.Wait()

	each(t, buffers, base)
}

// TestMultiWrite creates bunch of multi writers and writes to them, then adds more and writes again
func TestWriteAdd(t *testing.T) {
	var wg sync.WaitGroup
	var wgAdded sync.WaitGroup

	var writers []io.Writer
	var buffers []*bytes.Buffer

	// create & initialize writers and buffers into two categories
	// *Added ones will be added to multi writer later
	for i := 0; i < count; i++ {
		var buffer bytes.Buffer

		reader, writer := io.Pipe()

		writers = append(writers, writer)
		buffers = append(buffers, &buffer)

		if i%3 != 0 {
			wg.Add(1)
		}
		wgAdded.Add(1)
		go func(i int) {
			if i%3 != 0 {
				io.CopyN(&buffer, reader, int64(len(base)))
				wg.Done()
			}
			io.CopyN(&buffer, reader, int64(len(dynamic)))

			wgAdded.Done()
		}(i)
	}

	writersAdded := FilterWriters(writers, func(i int) bool { return i%3 == 0 })
	writersLeft := FilterWriters(writers, func(i int) bool { return i%3 != 0 })

	buffersAdded := FilterBuffers(buffers, func(i int) bool { return i%3 == 0 })
	buffersLeft := FilterBuffers(buffers, func(i int) bool { return i%3 != 0 })

	// create the multi writer
	mwriter := MultiWriter(writersLeft...)

	// write and ensure io.Copy returns
	write(t, mwriter, []byte(base))
	wg.Wait()

	each(t, buffersLeft, base)

	// add skipped writers to the writer
	mwriter.Add(writersAdded...)

	// write and ensure io.Copy returns
	write(t, mwriter, []byte(dynamic))
	wgAdded.Wait()

	each(t, buffersLeft, base+dynamic)
	each(t, buffersAdded, dynamic)
}

// TestMultiWrite creates multi writers and writes to them, then removes some of them and writes again
func TestWriteRemove(t *testing.T) {
	var wg sync.WaitGroup
	var wgRemoved sync.WaitGroup

	var writers []io.Writer
	var buffers []*bytes.Buffer

	// create & initialize writers and buffers into two categories
	// *Removed ones will be filtered out from multi writer later
	for i := 0; i < count; i++ {
		var buffer bytes.Buffer

		reader, writer := io.Pipe()

		writers = append(writers, writer)
		buffers = append(buffers, &buffer)

		// set up a goroutine so we don't block writes
		wg.Add(1)
		wgRemoved.Add(1)
		go func(i int) {
			// set up a goroutine so we don't block writes
			io.CopyN(&buffer, reader, int64(len(base)))

			wg.Done()

			if i%3 == 0 {
				wgRemoved.Done()
				return
			}

			io.CopyN(&buffer, reader, int64(len(dynamic)))

			wgRemoved.Done()
		}(i)
	}

	// create the multi writer
	mwriter := MultiWriter(writers...)

	// write and ensure io.Copy returns
	write(t, mwriter, []byte(base))
	wg.Wait()

	each(t, buffers, base)

	// remove the writers
	for i := 0; i < count; i++ {
		if i%3 == 0 {
			mwriter.Remove(writers[i])
		}
	}

	// write and ensure io.Copy returns
	write(t, mwriter, []byte(dynamic))
	wgRemoved.Wait()

	buffersLeft := FilterBuffers(buffers, func(i int) bool { return i%3 != 0 })
	buffersRemoved := FilterBuffers(buffers, func(i int) bool { return i%3 == 0 })

	each(t, buffersLeft, base+dynamic)
	each(t, buffersRemoved, base)

}

// TestMultiRead creates multi readers and reads from them
func TestMultiRead(t *testing.T) {
	var wg sync.WaitGroup

	var readers []io.Reader

	// create & initialize writers and buffers
	for i := 0; i < count; i++ {
		reader, writer := io.Pipe()

		readers = append(readers, reader)

		wg.Add(1)
		go func() {
			// set up a  goroutine so we don't block reads
			io.CopyN(writer, bytes.NewReader([]byte(base)), int64(len(base)))

			wg.Done()
		}()
	}

	// create the multi writer
	mreader := MultiReader(readers...)

	expected := strings.Repeat(base, count)
	// read and ensure io.Copy returns
	buffer := read(t, mreader, len(expected))
	wg.Wait()

	assert.Equal(t, expected, string(buffer))
}

// TestMultiRead creates multi readers and reads from them, then adds mores and reads again
func TestReadAdd(t *testing.T) {
	var wg sync.WaitGroup
	var wgAdded sync.WaitGroup
	var wgLeft sync.WaitGroup

	var readers []io.Reader
	var pipereaders []io.PipeReader

	wgLeft.Add(1)
	// create & initialize writers and buffers
	for i := 0; i < count; i++ {
		reader, writer := io.Pipe()

		readers = append(readers, reader)

		if i%3 != 0 {
			pipereaders = append(pipereaders, *reader)

			wg.Add(1)
		}
		wgAdded.Add(1)
		go func(i int) {
			if i%3 != 0 {
				io.CopyN(writer, bytes.NewReader([]byte(base)), int64(len(base)))
				wg.Done()
			}
			wgLeft.Wait()
			io.CopyN(writer, bytes.NewReader([]byte(dynamic)), int64(len(dynamic)))

			wgAdded.Done()
		}(i)
	}

	readersAdded := FilterReaders(readers, func(i int) bool { return i%3 == 0 })
	readersLeft := FilterReaders(readers, func(i int) bool { return i%3 != 0 })

	// create the multi writer
	mreader := MultiReader(readersLeft...)

	expected := strings.Repeat(base, len(readersLeft))
	// read and ensure io.Copy returns
	buffer := read(t, mreader, len(expected))
	wg.Wait()

	assert.Equal(t, expected, string(buffer))

	// close the initial set otherwise they will block
	for i := range pipereaders {
		pipereaders[i].Close()
	}

	wgLeft.Done()

	// add the rest of the readers
	mreader.Add(readersAdded...)

	expected = strings.Repeat(dynamic, len(readersAdded))
	// read and ensure io.Copy returns
	buffer = read(t, mreader, len(expected))
	wgAdded.Wait()

	assert.Equal(t, expected, string(buffer))

}

// TestReadRemove creates multi readers and reads from them, then removes some and reads again
func TestReadRemove(t *testing.T) {
	var wg sync.WaitGroup
	var wgRemoved sync.WaitGroup
	var wgLeft sync.WaitGroup

	var readers []io.Reader
	var writers []io.Writer

	wgLeft.Add(1)
	// create & initialize writers and buffers
	for i := 0; i < count; i++ {
		reader, writer := io.Pipe()

		readers = append(readers, reader)
		writers = append(writers, writer)

		// set up a goroutine so we don't block writes
		wg.Add(1)
		wgRemoved.Add(1)
		go func(i int) {
			// set up a goroutine so we don't block writes
			io.CopyN(writer, bytes.NewReader([]byte(base)), int64(len(base)))

			wg.Done()

			if i%3 == 0 {
				wgRemoved.Done()
				return
			}

			wgLeft.Wait()

			io.CopyN(writer, bytes.NewReader([]byte(dynamic)), int64(len(dynamic)))

			wgRemoved.Done()
		}(i)

	}

	// create the multi writer
	mreader := MultiReader(readers...)

	expected := strings.Repeat(base, count)
	// read and ensure io.Copy returns
	buffer := read(t, mreader, len(expected))
	wg.Wait()

	assert.Equal(t, expected, string(buffer))
	wgLeft.Done()

	readersLeft := FilterReaders(readers, func(i int) bool { return i%3 != 0 })
	readersRemoved := FilterReaders(readers, func(i int) bool { return i%3 == 0 })

	for i := range readersRemoved {
		mreader.Remove(readersRemoved[i])
	}

	expected = strings.Repeat(dynamic, len(readersLeft))
	// read and ensure io.Copy returns
	buffer = read(t, mreader, len(expected))
	wgRemoved.Wait()

	assert.Equal(t, expected, string(buffer))

}
