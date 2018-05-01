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

package iolog

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"io"
	"sync"
	"time"
)

// Clock defines an interface that wraps time.Now()
type Clock interface {
	Now() time.Time
}

// LogClock is an implementation of the Clock interface that
// returns time.Now() for use in the iolog package
type LogClock struct{}

// Now returns the local time time.Now()
func (LogClock) Now() time.Time {
	return time.Now()
}

// LogWriter tags log entries with a descriptive header and writes
// them to the underlying io.Writer
type LogWriter struct {
	Clock
	w    io.Writer
	prev []byte

	m      sync.Mutex
	closed bool
}

// NewLogWriter wraps an io.WriteCloser in a LogWriter
func NewLogWriter(w io.Writer, clock Clock) *LogWriter {
	return &LogWriter{
		w:     w,
		Clock: clock,
	}
}

// Write scans the supplied buffer, breaking off indvidual entries
// and writing them to the underlying Writer, flushing any leftover bytes
func (lw *LogWriter) Write(p []byte) (n int, err error) {
	var (
		start, end, i int
		entry         []byte
	)

	for {
		i = 0
		if i = bytes.IndexByte(p[start:], '\n') + 1; i == 0 {
			break
		}
		end = start + i
		entry = p[start:end]

		// do we have bytes left over from the last call?
		if lw.prev != nil {
			entry = append(lw.prev, entry...)
			lw.prev = nil
		}

		// we have a complete entry, let's write it
		n, err = lw.write(entry)
		if err != nil {
			return n, err
		}

		// advance starting index
		start = end
	}

	if start < len(p) {
		// save the rest of the buffer for the next call
		lw.prev = append(lw.prev, p[start:]...)
	}

	return len(p), err
}

func (lw *LogWriter) write(b []byte) (int, error) {
	var (
		err  error
		n, w int
	)
	for _, entry := range lw.split(b) {
		w, err = lw.w.Write(entry)
		n += w
		if err != nil {
			break
		}
	}
	// if we have to return with error, we should let the caller know
	// how many of the provided bytes were written, not including our header
	return n - encodedHeaderLengthBytes, err
}

// split breaks a log entry into smaller entries if necessary, adding a header to each
func (lw *LogWriter) split(b []byte) [][]byte {
	// break the entry up into multiple entries if necessary
	entries := [][]byte{}
	for len(b) > maxEntrySizeBytes {
		entries = append(entries, b[:maxEntrySizeBytes])
		b = b[maxEntrySizeBytes:]
	}
	entries = append(entries, b)

	for i := range entries {

		entry := entries[i]

		// Each entry has a 10-byte header that describes the entry as follows:
		// The first 8 bytes are the timestamp in int64 unix epoch format
		// The first 12 bits of the final two bytes represent the size of the entry
		// 0x8 represents the stream - 0 for stdout, 1 for stderr
		// 0x4 currently unused, reserved for a future flag
		// 0x2 currently unused, reserved for a future flag
		// 0x1 currently unused, reserved for a future flag
		header := make([]byte, headerLengthBytes)
		// prepare the header
		size := len(entry)
		s := uint16(size << 4) // make some room for stream and partial flags
		stream := 0            // TODO(jzt): defaults to 0 (stdout) until we figure out how to add stream tag
		s |= uint16(stream << 3)
		binary.LittleEndian.PutUint64(header[:8], uint64(lw.Now().UTC().UnixNano()))
		binary.LittleEndian.PutUint16(header[8:], s)

		// base64 encode the header
		encodedHeader := base64.StdEncoding.EncodeToString(header)
		entries[i] = append([]byte(encodedHeader), entry...)
	}
	return entries
}

// Close will flush the remaining bytes that have not yet been written
func (lw *LogWriter) Close() (err error) {
	lw.m.Lock()
	defer lw.m.Unlock()

	if lw.closed {
		return nil
	}

	// flush buffer if there are leftover bytes
	if lw.prev != nil {
		var n, w int
		for n < len(lw.prev) {
			w, err = lw.write(lw.prev[n:])
			n += w
			if err != nil {
				break
			}
		}
		// reset lw.prev
		lw.prev = nil
	}

	if c, ok := lw.w.(io.Closer); ok {
		c.Close()
		lw.closed = true
	}
	return err
}
