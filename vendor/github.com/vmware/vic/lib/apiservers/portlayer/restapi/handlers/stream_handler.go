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

package handlers

import (
	"bytes"
	"fmt"
	"io"
	"net/http"

	log "github.com/Sirupsen/logrus"
	"github.com/go-openapi/runtime"

	"github.com/vmware/vic/pkg/trace"
)

// StreamOutputHandler is a custom return handler that provides common
// stream handling across the API.
type StreamOutputHandler struct {
	outputStream *FlushingReader
	id           string
	outputName   string
	onHTTPClose  func() // clean up func called when transport closed
}

// NewStreamOutputHandler creates StreamOutputHandler with default headers values
func NewStreamOutputHandler(name string) *StreamOutputHandler {
	return &StreamOutputHandler{outputName: name}
}

// WithPayload adds the payload to the container set stdin internal server error response
func (s *StreamOutputHandler) WithPayload(payload *FlushingReader, id string, cleanup func()) *StreamOutputHandler {
	s.outputStream = payload
	s.id = id
	s.onHTTPClose = cleanup
	return s
}

// WriteResponse to the client
func (s *StreamOutputHandler) WriteResponse(rw http.ResponseWriter, producer runtime.Producer) {
	defer trace.End(trace.Begin(fmt.Sprintf("Stream of %s:%s", s.outputName, s.id)))

	rw.WriteHeader(http.StatusOK)
	if f, ok := rw.(http.Flusher); ok {
		f.Flush()
		s.outputStream.AddFlusher(f)
	}

	// prevent us from being dependent on client closing the connection once we've copied
	// all available data.
	done := make(chan bool)
	defer close(done)

	if s.onHTTPClose != nil {
		notify := rw.(http.CloseNotifier).CloseNotify()
		go func() {
			// continue on either
			select {
			case <-notify:
			case <-done:
			}

			// execute cleanup function
			s.onHTTPClose()

			log.Debugf("Completed stream cleanup for %s:%s", s.outputName, s.id)
		}()
	}

	_, err := io.Copy(rw, s.outputStream)
	if err != nil {
		log.Errorf("Error streaming %s for %s, total unwrapped bytes: %d, %s", s.outputName, s.id, s.outputStream.totalBytes, err)
	} else {
		log.Debugf("Finished streaming %s for %s (unwrapped bytes: %d)", s.outputName, s.id, s.outputStream.totalBytes)
	}
}

// closePipe is a convenience function for closing the event stream pipe
func closePipe(pipeReader *io.PipeReader, pipeWriter *io.PipeWriter) {
	if pipeReader != nil {
		pipeReader.Close()
	}
	if pipeWriter != nil {
		pipeWriter.Close()
	}
}

// GenericFlusher is a custom reader to allow us to detach cleanly during an io.Copy
type GenericFlusher interface {
	Flush()
}

type FlushingReader struct {
	io.Reader
	io.WriterTo

	flusher   GenericFlusher
	initBytes []byte

	totalBytes uint64
}

func NewFlushingReader(rdr io.Reader) *FlushingReader {
	return &FlushingReader{Reader: rdr, flusher: nil, initBytes: nil}
}

func NewFlushingReaderWithInitBytes(rdr io.Reader, initBytes []byte) *FlushingReader {
	return &FlushingReader{Reader: rdr, flusher: nil, initBytes: initBytes}
}

func (d *FlushingReader) AddFlusher(flusher GenericFlusher) {
	d.flusher = flusher
}

// readDetectInit() is used by WriteTo() which is used by io.Copy.  It attempts
// to detect a init byte buffer.  If it finds that init byte sequence, it is
// ignored.  This reader does not care about the init sequeunce.  The init sequence
// maybe used by the higher level interaction, which in this case is the Swagger
// establishing initial connection for stdin.
//
// Panics if the buf is smaller than the initBytes
func (d *FlushingReader) readDetectInit(buf []byte) (int, error) {
	initLen := len(d.initBytes)

	// fast path - len(nil) return 0
	if initLen == 0 {
		return d.Read(buf)
	}

	// make sure we have enough room
	if len(buf) < initLen {
		panic("Read buffer is smaller than the initialization byte sequence")
	}

	total := 0
	upto := 0
	for total < initLen {
		nr, err := d.Read(buf[total:])
		if nr > 0 {
			total += nr
			// we are only interested with the first initLen bytes
			upto = total
			if upto > initLen {
				upto = initLen
			}
			if bytes.Compare(d.initBytes[0:upto], buf[0:upto]) != 0 {
				// First bytes aren't part of init bytes so client must not be
				// the docker personality so break and ignore looking for the
				// init bytes.
				log.Debugf("Did not find primer bytes, stopping watch")
				return total, err
			}
		}
		if err != nil && total < initLen {
			log.Debugf("Primer bytes read %d bytes, err %s, stopping watch", nr, err)
			return 0, err
		}
	}

	// would have returned in the compare clause if not matching init bytes
	copy(buf[0:], buf[initLen:])
	log.Debugf("Found primer bytes, port layer client might be personality server")

	// no risk of returning <0
	return total - initLen, nil
}

// WriteTo is derived from go's io.Copy.  We use a smaller buffer so as to not hold up
// writing out data.  Go's version allocates 32k, and the Read will wait till
// buffer is filled (unless EOF is encountered).  Also, we force a flush if
// a flusher is added.  We've seen cases where the last bit of data for a
// screen doesn't reach the docker engine api server.  The flush solves that
// issue.
func (d *FlushingReader) WriteTo(w io.Writer) (written int64, err error) {
	buf := make([]byte, ioCopyBufferSize)

	defer func() {
		total := d.totalBytes + uint64(written)
		if total >= d.totalBytes {
			d.totalBytes = total
			return
		}

		log.Debug("Restarting total byte record for %p from zero, current total: %d", d, d.totalBytes)
		d.totalBytes = uint64(written)
	}()

	nr, er := d.readDetectInit(buf)
	for {
		if nr > 0 {
			nw, ew := w.Write(buf[0:nr])
			if d.flusher != nil {
				d.flusher.Flush()
			}
			if nw > 0 {
				written += int64(nw)
			}
			if ew != nil {
				err = ew
				break
			}
			if nr != nw {
				err = io.ErrShortWrite
				break
			}
		}
		// it's safe to ignore ErrClosedPipe -- encountered when
		// you close the pipe that is feeding the flushingReader
		if er == io.EOF || er == io.ErrClosedPipe {
			break
		}
		if er != nil {
			err = er
			break
		}
		nr, er = d.Read(buf)
	}
	return written, err
}
