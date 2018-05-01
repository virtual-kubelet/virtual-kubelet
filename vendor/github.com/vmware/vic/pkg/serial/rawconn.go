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

package serial

import (
	"io"
	"net"
	"os"
	"runtime"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"

	"github.com/vmware/vic/pkg/trace"
)

type NamedReadChannel interface {
	io.ReadCloser
	Name() string
	Fd() uintptr
}

type NamedWriteChannel interface {
	io.WriteCloser
	Name() string
	Fd() uintptr
}

type RawConn struct {
	rchannel   NamedReadChannel
	wchannel   NamedWriteChannel
	localAddr  net.Addr
	remoteAddr net.Addr
	err        chan error
	mutex      sync.Mutex
	closed     bool
}

func NewTypedConn(r NamedReadChannel, w NamedWriteChannel, net string) (*RawConn, error) {
	if tracing {
		defer trace.End(trace.Begin(""))
	}
	conn := &RawConn{
		rchannel: r,
		wchannel: w,

		localAddr:  *NewRawAddr(net, r.Name()),
		remoteAddr: *NewRawAddr(net, w.Name()),
		err:        make(chan error, 1),
		closed:     false,
	}

	return conn, nil
}

// NewFileConn creates a connection of the provided file - assumes file is a
// full duplex comm mechanism
func NewFileConn(file *os.File) (*RawConn, error) {
	if tracing {
		defer trace.End(trace.Begin(""))
	}
	return NewTypedConn(file, file, "file")
}

// NewRawConn creates a connection via the provided file descriptor - assumes file is a
// full duplex comm mechanism
func NewRawConn(fd uintptr, name string, net string) (*RawConn, error) {
	if tracing {
		defer trace.End(trace.Begin(""))
	}
	file := os.NewFile(fd, name)
	return NewTypedConn(file, file, net)
}

// NewHalfDuplexFileConn creates a connection via the provided files - this assumes that
// each file is a half-duplex mechanism, such as a linux fifo pipe
func NewHalfDuplexFileConn(read *os.File, write *os.File, name string, net string) (*RawConn, error) {
	if tracing {
		defer trace.End(trace.Begin(""))
	}
	return NewTypedConn(read, write, net)
}

// Read reads data from the connection.
func (conn *RawConn) Read(b []byte) (int, error) {
	if tracing {
		defer trace.End(trace.Begin(""))
	}

	var n int
	var err error

	if verbose {
		defer func() {
			log.Debugf("Returning error and bytes from read (%s:%s): %d, %s", conn.rchannel.Name(), conn.wchannel.Name(), n, err)
		}()
	}

	// TODO: this is horrific from a performance perspective - really need a better
	// way to interrupt that file.Read call
	bytes := make(chan int, 1)

	go func() {
		n, err = conn.rchannel.Read(b)

		// if we've got any bytes we need to pass them back so we cannot return
		// the error via conn.err
		bytes <- n
		close(bytes)
	}()

	conn.mutex.Lock()
	closed := conn.closed
	conn.mutex.Unlock()

	select {
	case n = <-bytes:
		if err != nil && closed {
			err = io.EOF
		}
		return n, err
	case e := <-conn.err:
		log.Debugf("Returning error from read: %s", e)
		// only one close will send an error and we have that, so this won't block
		// we do need to interrupt all reads
		conn.err <- e
		return n, e
	}
}

// Write writes data to the connection
func (conn *RawConn) Write(b []byte) (int, error) {
	if tracing {
		defer trace.End(trace.Begin(""))
	}
	return conn.wchannel.Write(b)
}

// Close closes the connection.
func (conn *RawConn) Close() error {
	if tracing {
		defer trace.End(trace.Begin(""))
	}
	var closed bool

	conn.mutex.Lock()
	closed = conn.closed
	conn.closed = true
	conn.mutex.Unlock()

	if closed {
		log.Debugf("Close called again on RawConn (%s:%s) - dropping", conn.rchannel.Name(), conn.wchannel.Name())
		return nil
	}

	// process the close
	log.Debugf("Closing the RawConn (%s:%s)", conn.rchannel.Name(), conn.wchannel.Name())
	errR := conn.rchannel.Close()
	errW := conn.wchannel.Close()

	if verbose {
		buf := make([]byte, 4096)
		bytes := runtime.Stack(buf, false)
		log.Debugf("Close called on RawConn (%s:%s):\n%s", conn.rchannel.Name(), conn.wchannel.Name(), string(buf[:bytes]))
	}

	log.Debugf("Pushing EOF to any blocked readers on the raw connection (%s:%s)", conn.rchannel.Name(), conn.wchannel.Name())
	conn.err <- io.EOF

	if errR != nil {
		return errR
	}
	return errW
}

// LocalAddr returns the local network address.
func (conn *RawConn) LocalAddr() net.Addr {
	if tracing {
		defer trace.End(trace.Begin(""))
	}
	return conn.localAddr
}

// RemoteAddr returns the remote network address.
func (conn *RawConn) RemoteAddr() net.Addr {
	if tracing {
		defer trace.End(trace.Begin(""))
	}
	return conn.remoteAddr
}

// SetDeadline sets the read and write deadlines associated
// with the connection
func (conn *RawConn) SetDeadline(t time.Time) error {
	if tracing {
		defer trace.End(trace.Begin(t.String()))
	}
	// https://golang.org/src/net/fd_poll_runtime.go#L133
	// consider implementing this by making RawConn a netFD
	// if we can find a way around the lack of export
	return nil
}

// SetReadDeadline sets the deadline for future Read calls.
func (conn *RawConn) SetReadDeadline(t time.Time) error {
	if tracing {
		defer trace.End(trace.Begin(t.String()))
	}
	return nil
}

// SetWriteDeadline sets the deadline for future Write calls.
func (conn *RawConn) SetWriteDeadline(t time.Time) error {
	if tracing {
		defer trace.End(trace.Begin(t.String()))
	}
	return nil
}
