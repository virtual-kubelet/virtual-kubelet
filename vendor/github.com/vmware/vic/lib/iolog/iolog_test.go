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
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"math/rand"
	"testing"
	"time"
)

type TestReadCloser struct {
	*bytes.Buffer
	io.Closer
}

type testClock struct{}

func (testClock) Now() time.Time {
	return time.Unix(191193300, 0)
}

var (
	tc         testClock
	expectedTs = time.Unix(0, tc.Now().UTC().UnixNano()).Format(RFC3339NanoFixed)
)

func TestWriteEntry(t *testing.T) {
	var buf bytes.Buffer
	w := NewLogWriter(&buf, tc)
	msg := "The quick brown fox jumped over the lazy dog\n"
	size := len(msg)

	n, err := w.Write([]byte(msg))
	if err != nil {
		t.Errorf(err.Error())
	}

	if n != size {
		t.Errorf("Wrote %d bytes, expected to write %d", n, len(msg))
	}

	b := buf.Bytes()
	expected := size + encodedHeaderLengthBytes
	if len(b) != expected {
		t.Errorf("Serialized entry was %d bytes, expected %d", len(b), expected)
	}

	entry := string(b[encodedHeaderLengthBytes:])
	if entry != msg {
		t.Errorf("Written message did not match original. Got %s, expected %s", entry, msg)
	}
}

func TestWriteLargeEntry(t *testing.T) {
	var buf bytes.Buffer
	w := NewLogWriter(&buf, tc)
	size := 32 * 1024
	msg := make([]byte, size)

	n, err := w.Write(msg)
	if err != nil {
		t.Errorf(err.Error())
	}
	w.Close()

	if n != size {
		t.Errorf("Wrote %d bytes, expected to write %d", n, len(msg))
	}

	// 32KB entry should have been broken into 8 4095B entries and 1 8B entry
	// for a total of 32KB in data + 9*encodedHeaderLengthBytes in headers
	expected := len(msg) + 9*encodedHeaderLengthBytes
	if len(buf.Bytes()) != expected {
		t.Errorf("Wrote %d bytes, expected to write %d", len(buf.Bytes()), expected)
	}
}

func TestWriteReadLargeEntry(t *testing.T) {
	var in bytes.Buffer
	w := NewLogWriter(&in, tc)
	rc := &TestReadCloser{Buffer: &in}
	r := NewLogReader(rc, false)

	rand.Seed(time.Now().Unix())
	data := make([]byte, 32*1024)
	n := 0
	for n < len(data) {
		w, err := rand.Read(data[n:])
		n += w
		if err != nil {
			break
		}
	}

	h := sha256.New()
	d := bytes.NewBuffer(data)
	if _, err := io.Copy(h, d); err != nil {
		t.Errorf("Error calculating sha256: %s", err)
	}
	shaSrc := fmt.Sprintf("%x", h.Sum(nil))

	n, err := w.Write(data)
	if n != len(data) {
		t.Errorf("Wrote %d bytes, expected to write %d", n, len(data))
	}
	w.Close()

	if err != nil {
		t.Errorf("Error writing random data: %s", err)
	}

	result := make([]byte, len(data))
	n = 0
	for n < len(data) {
		w, err := r.Read(result[n:])
		n += w
		if err != nil {
			break
		}
	}

	if err != nil {
		t.Errorf("Error reading random data: %s", err)
	}

	if len(result) != len(data) {
		t.Errorf("Expected result of size %d, got %d", len(data), len(result))
	}

	h = sha256.New()
	d = bytes.NewBuffer(result)
	if _, err := io.Copy(h, d); err != nil {
		t.Errorf("Error calculating sha256: %s", err)
	}
	shaDst := fmt.Sprintf("%x", h.Sum(nil))

	if shaSrc != shaDst {
		t.Errorf("Checksum failed: expected %s, got %s", shaSrc, shaDst)
	}
}

func TestSplit(t *testing.T) {
	var in bytes.Buffer

	w := NewLogWriter(&in, tc)
	numEntries := 5
	data := make([]byte, numEntries*maxEntrySizeBytes)
	entries := w.split(data)

	if len(entries) != numEntries {
		t.Errorf("Expected %d entries, got %d", numEntries, len(entries))
	}

	for _, entry := range entries {
		h, err := base64.StdEncoding.DecodeString(string(entry[:encodedHeaderLengthBytes]))
		if err != nil {
			t.Errorf("Error decoding base64 header: %s", err)
		}
		ts := time.Unix(0, int64(binary.LittleEndian.Uint64(h[:8])))

		expected := tc.Now().UTC().UnixNano()
		actual := ts.UnixNano()
		if expected != actual {
			t.Errorf("Expected unix timestamp %d, got %d", expected, actual)
		}

		s := binary.LittleEndian.Uint16(h[8:10])

		stream := (s & streamFlag) >> 3
		if stream != 0 {
			t.Errorf("Expected stream %d, got %d", 0, stream)
		}

		size := int(s >> 4)
		msg := entry[encodedHeaderLengthBytes:]
		if size != len(msg) {
			t.Errorf("Expected msg size %d, got %d", len(msg), size)
		}
	}
}

func TestTimestamps(t *testing.T) {
	var in bytes.Buffer
	w := NewLogWriter(&in, tc)
	rc := &TestReadCloser{Buffer: &in}
	r := NewLogReader(rc, true) // enable timestamp output

	msgs := []string{
		"Jerry, it’s Frank Costanza. Steinbrenner’s here. George is dead. Call me back.\n",
		"When you look annoyed all the time, people think that you’re busy.\n",
		"You put the balm on? Who told you to put the balm on? I didn't tell you to put the balm on. Why'd you put the balm on?\n",
		"At the Festivus dinner, you gather your family around and you tell them all the ways they have disappointed you over the past year.\n",
	}

	for _, msg := range msgs {
		n, err := w.Write([]byte(msg))
		if n != len(msg) {
			t.Errorf("Wrote %d bytes, expected to write %d", n, len(msg))
		}
		if err != nil {
			t.Errorf("Error writing message: %s", err)
		}
	}
	w.Close()

	tslen := len(expectedTs)
	results := [][]byte{}
	for _, m := range msgs {
		msg := make([]byte, len(m)+tslen+1) // we need room for the message, the timestamp, and a space
		_, err := r.Read(msg)
		if err != nil && err != io.EOF {
			t.Errorf("Error reading message: %s", err)
		}
		results = append(results, msg)
	}

	if len(results) != len(msgs) {
		t.Errorf("Expected results of size %d, got %d", len(msgs), len(results))
	}
	for i, result := range results {
		ts := string(result[:tslen])
		msg := result[tslen+1:]
		if string(ts) != expectedTs {
			t.Errorf("Expected timestamp %s, got %s", expectedTs, ts)
		}
		if string(msg) != msgs[i] {
			t.Errorf("Expected result %s, got %s", msgs[i], string(msg))
		}
	}
}
