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
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestWriterReconnect(t *testing.T) {
	dn := &mockNetDialer{}
	dn.On("dial").Return(nil, assert.AnError)
	w := newWriter(priority, tag, "", dn, nil)

	go w.run()
	<-w.running

	calls := []func(string) error{
		w.Emerg,
		w.Crit,
		w.Err,
		w.Warning,
		w.Info,
		w.Debug,
	}
	for _, f := range calls {
		err := f("test")
		assert.NoError(t, err)
	}

	w.Close()

	dn.AssertNumberOfCalls(t, "dial", 1+len(calls))
}

func TestWriterWrite(t *testing.T) {
	msg := "foo"

	f := &mockFormatter{}
	f.On("Format", priority, mock.Anything, "host", tag, msg).Return("test")

	a := &MockAddr{}
	a.On("String").Return("host:123")

	c := &MockNetConn{}
	c.On("LocalAddr").Return(a)
	c.On("Write", []byte("test\n")).Return(len(msg), nil)
	c.On("Close").Return(nil)

	dn := &mockNetDialer{}
	dn.On("dial").Return(c, nil)

	w := newWriter(priority, tag, "", dn, f)
	n, err := w.Write([]byte(msg))
	assert.NoError(t, err)
	assert.Equal(t, len(msg), n)

	go w.run()
	<-w.running

	w.Close()

	c.AssertExpectations(t)
	dn.AssertNumberOfCalls(t, "dial", 1)
}

func TestMaxLogBuffer(t *testing.T) {
	f := &mockFormatter{}

	dn := &mockNetDialer{}
	c := &MockNetConn{}
	a := &MockAddr{}
	a.On("String").Return("foo")
	c.On("LocalAddr").Return(a)
	c.On("Close").Return(nil)

	dn.On("dial").Return(c, nil)
	w := newWriter(priority, tag, "", dn, f)

	for i := 0; i < maxLogBuffer+1; i++ {
		msg := fmt.Sprintf("%d", i)
		f.On("Format", priority, mock.Anything, "", tag, msg).Return(msg)
		c.On("Write", []byte(msg+"\n")).Return(len(msg), nil)
		w.Write([]byte(msg))
	}

	go w.run()

	<-w.running

	w.Close()

	for i := 0; i < maxLogBuffer; i++ {
		if !f.AssertCalled(t, "Format", priority, mock.Anything, "", tag, fmt.Sprintf("%d", i)) ||
			!c.AssertCalled(t, "Write", []byte(fmt.Sprintf("%d\n", i))) {
		}
	}

	f.AssertNumberOfCalls(t, "Format", maxLogBuffer)
	f.AssertNotCalled(t, "Format", priority, mock.Anything, "", tag, fmt.Sprintf("%d", maxLogBuffer))
}

func TestWriterReconnectWrite(t *testing.T) {
	dn := &mockNetDialer{}
	c := &MockNetConn{}
	a := &MockAddr{}
	a.On("String").Return("addr:123")
	c.On("LocalAddr").Return(a)
	c.On("Close").Return(nil)

	dn.On("dial").Return(nil, assert.AnError)
	f := &mockFormatter{}
	w := newWriter(priority, tag, "", dn, f)

	go w.run()
	<-w.running

	dn.AssertNumberOfCalls(t, "dial", 1)

	dn = &mockNetDialer{}
	dn.On("dial").Return(c, nil)
	w.dialer = dn

	f.On("Format", priority, mock.Anything, "addr", tag, "test").Return("test")
	c.On("Write", []byte("test\n")).Return(len("test"), nil)

	w.Write([]byte("test"))
	w.Close()

	dn.AssertNumberOfCalls(t, "dial", 1)
	c.AssertNumberOfCalls(t, "Write", 1) // 1 call to writer.write
	f.AssertNumberOfCalls(t, "Format", 1)
}

func TestWriterReconnectWriteError(t *testing.T) {
	dn := &mockNetDialer{}
	c := &MockNetConn{}
	a := &MockAddr{}
	a.On("String").Return("addr:123")
	c.On("LocalAddr").Return(a)
	c.On("Close").Return(nil)

	dn.On("dial").Return(c, nil)
	f := &mockFormatter{}
	w := newWriter(priority, tag, "", dn, f)

	go w.run()
	<-w.running

	dn.AssertNumberOfCalls(t, "dial", 1)

	f.On("Format", priority, mock.Anything, "addr", tag, "test").Return("test")
	c.On("Write", []byte("test\n")).Return(0, assert.AnError)

	w.Write([]byte("test"))

	w.Close()

	f.AssertExpectations(t)
	c.AssertExpectations(t)
}

func TestWriterWithTag(t *testing.T) {
	f := &mockFormatter{}
	f.On("Format", priority, mock.Anything, "addr", "child", "child").Return("child")
	f.On("Format", priority, mock.Anything, "addr", "gchild", "gchild").Return("gchild")

	dn := &mockNetDialer{}
	c := &MockNetConn{}
	a := &MockAddr{}
	a.On("String").Return("addr:123")
	c.On("LocalAddr").Return(a)
	c.On("Close").Return(nil)
	c.On("Write", []byte("child\n")).Return(len("child"), nil)
	c.On("Write", []byte("gchild\n")).Return(len("gchild"), nil)

	dn.On("dial").Return(c, nil)

	w := newWriter(priority, tag, "", dn, f)

	child := w.WithTag("child")
	child.Write([]byte("child"))

	gchild := child.WithTag("gchild")
	gchild.Write([]byte("gchild"))

	go w.run()
	<-w.running

	child.Close()
	gchild.Close()

	select {
	case <-w.done:
	default:
		assert.FailNow(t, "parent writer is not closed by child Close call")
	}

	f.AssertExpectations(t)
	c.AssertExpectations(t)
}

func TestWriterWithPriority(t *testing.T) {
	f := &mockFormatter{}
	f.On("Format", Err|Daemon, mock.Anything, "addr", tag, "err").Return("err")
	f.On("Format", Debug|Daemon, mock.Anything, "addr", tag, "debug").Return("debug")

	dn := &mockNetDialer{}
	c := &MockNetConn{}
	a := &MockAddr{}
	a.On("String").Return("addr:123")
	c.On("LocalAddr").Return(a)
	c.On("Close").Return(nil)
	c.On("Write", []byte("err\n")).Return(len("err"), nil)
	c.On("Write", []byte("debug\n")).Return(len("debug"), nil)

	dn.On("dial").Return(c, nil)

	w := newWriter(priority, tag, "", dn, f)

	errw := w.WithPriority(Err | Daemon)
	errw.Write([]byte("err"))

	debugw := errw.WithPriority(Debug | Daemon)
	debugw.Write([]byte("debug"))

	go w.run()
	<-w.running

	errw.Close()

	select {
	case <-w.done:
	default:
		assert.FailNow(t, "parent writer is not closed by child Close call")
	}

	f.AssertExpectations(t)
	c.AssertExpectations(t)
}

func TestWriterInitialConnectError(t *testing.T) {

	var tests = []error{
		&net.ParseError{},
		&net.AddrError{},
	}

	for _, e := range tests {
		dn := &mockNetDialer{}
		dn.On("dial").Return(nil, e)

		w := newWriter(priority, tag, "", dn, &mockFormatter{})
		w.run()

		select {
		case <-w.running:
			assert.FailNow(t, "writer should not run when connect() fails initially")
		default:
		}
	}
}
