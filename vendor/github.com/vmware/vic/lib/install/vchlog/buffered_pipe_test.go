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

package vchlog

import (
	. "io"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Part 1. Test basic pipe functionality
// (Referred to: https://golang.org/src/io/pipe_test.go)

// Test a single read and write
func TestSingleReadWrite(t *testing.T) {
	testByte := []byte("hello world")
	toRead := make([]byte, 64)
	bp := NewBufferedPipe()
	writeDone := make(chan int)

	// write to the pipe
	go write(t, bp, testByte, len(testByte), writeDone)

	// read from the pipe
	read(t, bp, toRead, len(testByte), nil)
	assert.Equal(t, string(testByte), string(toRead[0:len(testByte)]),
		"expected %s, read %s", string(testByte), string(toRead[0:len(testByte)]))

	<-writeDone
	bp.Close()
}

// Test a sequence of reads and writes
func readSequence(t *testing.T, r Reader, c chan int) {
	buf := make([]byte, 64)
	for {
		n, err := r.Read(buf)
		if err == EOF {
			c <- 0
			break
		}
		assert.Nil(t, err, "read error: %v", err)
		c <- n
	}
}

func TestSequenceReadsWrites(t *testing.T) {
	readDone := make(chan int)
	bp := NewBufferedPipe()
	buf := make([]byte, 64)

	// fire the reader
	go readSequence(t, bp, readDone)

	// write to the pipe
	for i := 0; i < 5; i++ {
		l := 5 + i*10
		toWrite := buf[0:l]
		write(t, bp, toWrite, l, nil)

		n := <-readDone
		assert.Equal(t, l, n, "wrote %d bytes, read %d bytes", l, n)
	}

	bp.Close()
	n := <-readDone
	assert.Equal(t, 0, n, "final read should be 0, got %d", n)
}

// Part 2. Test buffered pipe functionalities

// Test write buffering
func TestWriteBuffering(t *testing.T) {
	bp := NewBufferedPipe()

	// write first chunk of data
	firstChunk := make([]byte, 16)
	write(t, bp, firstChunk, len(firstChunk), nil)

	// fire the reader
	readDone := make(chan int)
	go func() {
		buf := make([]byte, 64)
		for i := 0; i < 2; i++ {
			l := 8 * (i + 2)
			read(t, bp, buf, l, nil)
		}
		readDone <- 0
	}()

	// sleep before the next write
	time.Sleep(time.Second * 5)

	// write second chunk of data
	secondChunk := make([]byte, 24)
	write(t, bp, secondChunk, len(secondChunk), nil)

	<-readDone
	bp.Close()
}

// Test concurrent writers
func writeSequence(t *testing.T, w Writer, l int, c chan int) {
	for i := 0; i < l; i++ {
		write(t, w, []byte(strconv.Itoa(i)), 1, nil)
	}
	time.Sleep(1 * time.Second)
	c <- 0
}

func TestConcurrentWriter(t *testing.T) {
	bp := NewBufferedPipe()
	l := 10
	writersNum := 5
	total := writersNum * l

	// fire 5 concurrent writers
	chans := make([]chan int, writersNum)
	for i := 0; i < writersNum; i++ {
		chans[i] = make(chan int)
		go writeSequence(t, bp, l, chans[i])
	}

	// fire one concurrent reader
	finalLen := 0
	readDone := make(chan int)
	go func() {
		for {
			buf := make([]byte, 16)
			n, err := bp.Read(buf)
			if err == EOF {
				break
			}
			assert.Nil(t, err, "read error: %v", err)
			finalLen += n
		}
		readDone <- 0
	}()

	// wait for writers to finish
	for i := 0; i < writersNum; i++ {
		<-chans[i]
	}
	bp.Close()

	// check if the output has all the bytes
	<-readDone
	assert.Equal(t, total, finalLen,
		"%d concurrent writers wrote %d bytes total, got %d bytes", writersNum, total, finalLen)
}

// Part 3. Edge cases

// Test read on closed pipe during read
func delayClose(t *testing.T, cl Closer, c chan int) {
	time.Sleep(1 * time.Millisecond)
	err := cl.Close()
	assert.Nil(t, err, "close error: %v", err)
	c <- 0
}

func TestPipeReadClose(t *testing.T) {
	bp := NewBufferedPipe()
	c := make(chan int)

	// delay closer
	go delayClose(t, bp, c)

	// read is expected to block until the pipe is closed
	buf := make([]byte, 64)
	n, err := bp.Read(buf)
	<-c

	assert.Equal(t, EOF, err, "read from closed pipe: %v want %v", err, EOF)
	assert.Equal(t, 0, n, "read on closed pipe returned %d bytes", n)
}

// Test write on closed pipe during write
func TestPipeWriteClose(t *testing.T) {
	bp := NewBufferedPipe()

	// close pipe
	bp.Close()

	// write
	buf := make([]byte, 64)
	n, err := bp.Write(buf)

	assert.Equal(t, ErrUnexpectedEOF, err, "write to closed pipe: %v want %v", err, ErrUnexpectedEOF)
	assert.Equal(t, 0, n, "write on closed pipe returned %d bytes", n)
}

// Helper Functions

// write writes the data and report any error
func write(t *testing.T, w Writer, data []byte, expected int, c chan int) {
	n, err := w.Write(data)
	assert.Nil(t, err, "write error: %v", err)
	assert.Equal(t, expected, n, "expected %d bytes, wrote %d", expected, n)

	if c != nil {
		c <- 0
	}
}

// read reads to buffer and report any error
func read(t *testing.T, r Reader, buf []byte, expected int, c chan int) {
	n, err := r.Read(buf)
	assert.Nil(t, err, "read error: %v", err)
	assert.Equal(t, expected, n, "expected %d bytes, read %d", expected, n)

	if c != nil {
		c <- 0
	}
}
