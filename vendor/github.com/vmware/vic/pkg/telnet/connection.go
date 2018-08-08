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

package telnet

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"net"
	"sync"

	log "github.com/Sirupsen/logrus"
)

type optCallBackFunc func(byte, byte)

type connOpts struct {
	// conn is the underlying connection
	conn net.Conn
	fsm  *fsm

	serverOpts  map[byte]bool
	clientOpts  map[byte]bool
	optCallback optCallBackFunc

	Handlers
}

// Conn is the struct representing the telnet connection
type Conn struct {
	connOpts

	// the connection write channel. Everything required to be written to the connection goes to this channel
	writeCh chan []byte
	// dataRW is the data buffer. It is written to by the FSM and read from by the data handler
	dataRW        io.ReadWriter
	cmdBuffer     bytes.Buffer
	handlerWriter io.WriteCloser

	// used in the dataHandlerWrapper to notify that the telnet connection is closed
	dataHandlerCloseCh chan chan struct{}
	// used in the dataHandlerWrapper to notify that data has been writeen to the dataRW buffer
	dataWrittenCh chan bool

	// connWriteDoneCh closes the write loop when the telnet connection is closed
	connWriteDoneCh chan chan struct{}

	closedMutex sync.Mutex
	closed      bool
}

// Safely read/write concurrently to the data Buffer
// databuffer is written to by the FSM and it is read from by the dataHandler
type dataReadWriter struct {
	buf bytes.Buffer
	sync.Mutex
}

func (drw *dataReadWriter) Read(p []byte) (int, error) {
	drw.Lock()
	defer drw.Unlock()
	return drw.buf.Read(p)
}

func (drw *dataReadWriter) Write(p []byte) (int, error) {
	drw.Lock()
	defer drw.Unlock()
	return drw.buf.Write(p)
}

// This is the Writer that is passed to the handlers to write to the telnet connection
type connectionWriter struct {
	ch chan []byte
}

func (cw *connectionWriter) Write(b []byte) (int, error) {
	if cw.ch != nil {
		cw.ch <- b
	}
	return len(b), nil
}

func (cw *connectionWriter) Close() error {
	close(cw.ch)
	cw.ch = nil
	return nil
}

func newConn(opts *connOpts) *Conn {
	tc := &Conn{
		connOpts:           *opts,
		writeCh:            make(chan []byte),
		dataHandlerCloseCh: make(chan chan struct{}),
		dataWrittenCh:      make(chan bool),
		connWriteDoneCh:    make(chan chan struct{}),
		closed:             false,
	}
	if tc.optCallback == nil {
		tc.optCallback = tc.handleOptionCommand
	}
	tc.handlerWriter = &connectionWriter{
		ch: tc.writeCh,
	}
	tc.dataRW = &dataReadWriter{}
	tc.fsm.tc = tc
	return tc
}

//UnderlyingConnection returns the underlying TCP connection
func (c *Conn) UnderlyingConnection() net.Conn {
	return c.conn
}

func (c *Conn) writeLoop() {
	log.Debugf("entered write loop")
	for {
		select {
		case writeBytes := <-c.writeCh:
			c.conn.Write(writeBytes)
		case ch := <-c.connWriteDoneCh:
			ch <- struct{}{}
			return
		}
	}
}

func (c *Conn) startNegotiation() {
	for k := range c.serverOpts {
		log.Infof("sending WILL %d", k)
		c.sendCmd(Will, k)
	}
	for k := range c.clientOpts {
		log.Infof("sending DO %d", k)
		c.sendCmd(Do, k)
	}
}

// close closes the telnet connection
func (c *Conn) close() {
	c.closedMutex.Lock()
	defer c.closedMutex.Unlock()

	c.closed = true
	log.Infof("Closing the connection")
	c.conn.Close()
	c.closeConnLoopWrite()
	c.closeDatahandler()
	c.handlerWriter.Close()
	log.Infof("telnet connection closed")

	// calling the CloseHandler passed by vspc
	c.CloseHandler(c)

}

func (c *Conn) closeConnLoopWrite() {
	connLoopWriteCh := make(chan struct{})
	c.connWriteDoneCh <- connLoopWriteCh
	<-connLoopWriteCh
	log.Debugf("connection loop write-side closed")
}

func (c *Conn) closeDatahandler() {
	dataCh := make(chan struct{})
	c.dataHandlerCloseCh <- dataCh
	<-dataCh
}

func (c *Conn) sendCmd(cmd byte, opt byte) {
	c.writeCh <- []byte{Iac, cmd, opt}
	log.Debugf("Sending command: %v %v", cmd, opt)
}

func (c *Conn) handleOptionCommand(cmd byte, opt byte) {
	if cmd == Will || cmd == Wont {
		if _, ok := c.clientOpts[opt]; !ok {
			c.sendCmd(Dont, opt)
			return
		}
		c.sendCmd(Do, opt)
	}

	if cmd == Do || cmd == Dont {
		if _, ok := c.serverOpts[opt]; !ok {
			c.sendCmd(Wont, opt)
			return
		}
		log.Debugf("Sending WILL command")
		c.sendCmd(Will, opt)

	}
}

func (c *Conn) dataHandlerWrapper(w io.Writer, r io.Reader) {
	defer func() {
		log.Debugf("data handler closed")
	}()
	for {
		select {
		case ch := <-c.dataHandlerCloseCh:
			ch <- struct{}{}
			return
		case <-c.dataWrittenCh:
			// #nosec: Errors unhandled.
			if b, _ := ioutil.ReadAll(r); len(b) > 0 {
				c.DataHandler(w, b, c)
			}
		}
	}
}

func (c *Conn) cmdHandlerWrapper(w io.Writer, r io.Reader) {
	// #nosec: Errors unhandled.
	if cmd, _ := ioutil.ReadAll(r); len(cmd) > 0 {
		c.CmdHandler(w, cmd, c)
	}
}

// IsClosed returns true if the connection is already closed
func (c *Conn) IsClosed() bool {
	c.closedMutex.Lock()
	defer c.closedMutex.Unlock()
	return c.closed
}

// WriteData writes telnet data to the underlying connection doubling every IAC
func (c *Conn) WriteData(b []byte) (int, error) {
	var escaped []byte
	for _, v := range b {
		if v == Iac {
			escaped = append(escaped, 255)
		}
		escaped = append(escaped, v)
	}
	if c.IsClosed() {
		return -1, errors.New("telnet connection is already closed")
	}
	c.writeCh <- escaped
	return len(b), nil
}
