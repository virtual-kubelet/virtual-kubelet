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
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type dummyConn struct {
	dataBuf bytes.Buffer
}

func (c *dummyConn) Read(b []byte) (n int, err error) {
	return 3, nil
}

func (c *dummyConn) Write(b []byte) (n int, err error) {
	return c.dataBuf.Write(b)
}

func (c *dummyConn) Close() error {
	return nil
}

func (c *dummyConn) LocalAddr() net.Addr {
	return nil
}

func (c *dummyConn) RemoteAddr() net.Addr {
	return nil
}

func (c *dummyConn) SetDeadline(t time.Time) error {
	return nil
}

func (c *dummyConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (c *dummyConn) SetWriteDeadline(t time.Time) error {
	return nil
}

func newTestItem() *Conn {
	opts := connOpts{
		conn: &dummyConn{},
		serverOpts: map[byte]bool{
			Binary: true,
			Echo:   true,
		},
		clientOpts: map[byte]bool{
			Naocrd: true,
			Naohts: true,
		},
	}
	return &Conn{
		connOpts:        opts,
		writeCh:         make(chan []byte),
		connWriteDoneCh: make(chan chan struct{}),
	}
}

func TestWriteData(t *testing.T) {
	conn := newTestItem()
	data := [][]byte{{10, 15, 23, 210}, {10, Iac, 30, 40}, {10, Iac, Iac, 30, 40}}
	expected := [][]byte{{10, 15, 23, 210}, {10, Iac, Iac, 30, 40}, {10, Iac, Iac, Iac, Iac, 30, 40}}
	for i, d := range data {
		go conn.WriteData(d)
		received := <-conn.writeCh
		assert.Equal(t, expected[i], received)
	}
}

func TestSendCmd(t *testing.T) {
	conn := newTestItem()
	go conn.sendCmd(Do, Binary)
	received := <-conn.writeCh
	assert.Equal(t, []byte{Iac, Do, Binary}, received)
}

func TestNegotiation(t *testing.T) {
	conn := newTestItem()
	go conn.startNegotiation()
	expected := map[byte][]byte{
		Binary: {Iac, Will, Binary},
		Echo:   {Iac, Will, Echo},
		Naocrd: {Iac, Do, Naocrd},
		Naohts: {Iac, Do, Naohts},
	}
	for i := 0; i < 4; i++ {
		r := <-conn.writeCh
		assert.Equal(t, expected[r[2]], r)
	}
}

func TestWriteLoop(t *testing.T) {
	conn := newTestItem()
	go conn.writeLoop()
	conn.writeCh <- []byte{1, 2, 3, 4}
	conn.writeCh <- []byte{5, 6}
	conn.writeCh <- []byte{7}
	ch := make(chan struct{})
	conn.connWriteDoneCh <- ch
	<-ch
	assert.Equal(t, []byte{1, 2, 3, 4, 5, 6, 7}, conn.connOpts.conn.(*dummyConn).dataBuf.Bytes())
}
