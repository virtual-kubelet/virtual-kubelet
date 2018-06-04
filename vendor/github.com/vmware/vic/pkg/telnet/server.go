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
	"fmt"
	"io"
	"net"

	log "github.com/Sirupsen/logrus"
)

// DataHandlerFunc is the callback function in the event of receiving data from the telnet client
type DataHandlerFunc func(w io.Writer, data []byte, tc *Conn)

// CmdHandlerFunc is the callback function in the event of receiving a command from the telnet client
type CmdHandlerFunc func(w io.Writer, cmd []byte, tc *Conn)

// CloseHandlerFunc is the callback function in the event of receiving EOF from the telnet client
type CloseHandlerFunc func(tc *Conn)

var defaultDataHandlerFunc = func(w io.Writer, data []byte, tc *Conn) {}

var defaultCmdHandlerFunc = func(w io.Writer, cmd []byte, tc *Conn) {}

var defaultCloseHandlerFunc = func(tc *Conn) {}

type Handlers struct {
	DataHandler  DataHandlerFunc
	CmdHandler   CmdHandlerFunc
	CloseHandler CloseHandlerFunc
}

// ServerOpts is the telnet server constructor options
type ServerOpts struct {
	Addr       string
	ServerOpts []byte
	ClientOpts []byte

	Handlers
}

// Server is the struct representing the telnet server
type Server struct {
	ServerOptions map[byte]bool
	ClientOptions map[byte]bool

	Handlers

	ln net.Listener
}

// NewServer is the constructor of the telnet server
func NewServer(opts ServerOpts) *Server {
	ts := new(Server)
	ts.ClientOptions = make(map[byte]bool)
	ts.ServerOptions = make(map[byte]bool)
	for _, v := range opts.ServerOpts {
		ts.ServerOptions[v] = true
	}
	for _, v := range opts.ClientOpts {
		ts.ClientOptions[v] = true
	}
	ts.DataHandler = defaultDataHandlerFunc
	if opts.DataHandler != nil {
		ts.DataHandler = opts.DataHandler
	}

	ts.CmdHandler = defaultCmdHandlerFunc
	if opts.CmdHandler != nil {
		ts.CmdHandler = opts.CmdHandler
	}

	ts.CloseHandler = defaultCloseHandlerFunc
	if opts.CloseHandler != nil {
		ts.CloseHandler = opts.CloseHandler
	}

	ln, err := net.Listen("tcp", opts.Addr)
	if err != nil {
		panic(fmt.Sprintf("cannot start telnet server: %v", err))
	}
	ts.ln = ln
	return ts
}

// Accept accepts a connection and returns the Telnet connection
func (ts *Server) Accept() (*Conn, error) {
	// #nosec: Errors unhandled.
	conn, _ := ts.ln.Accept()
	log.Info("connection received")
	opts := connOpts{
		conn:       conn,
		Handlers:   ts.Handlers,
		clientOpts: ts.ClientOptions,
		fsm:        newFSM(),
	}
	tc := newConn(&opts)
	go tc.writeLoop()
	go tc.dataHandlerWrapper(tc.handlerWriter, tc.dataRW)
	go tc.fsm.start()
	go tc.startNegotiation()
	return tc, nil
}
