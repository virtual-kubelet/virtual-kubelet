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

package communication

import (
	"net"
	"sync"
	"testing"
	"time"

	"context"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/testdata"

	log "github.com/Sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/vmware/vic/lib/migration/feature"
	"github.com/vmware/vic/lib/tether/msgs"
	"github.com/vmware/vic/pkg/serial"
)

// Start the server, make 200 client connections, test they connect, then Stop.
func TestAttachStartStop(t *testing.T) {
	log.SetLevel(log.InfoLevel)
	if testing.Verbose() {
		log.SetLevel(log.DebugLevel)
	}
	s := NewServer("localhost", 0)

	var wg sync.WaitGroup

	dial := func() {
		defer wg.Done()

		c, err := net.Dial("tcp", s.l.Addr().String())
		if !assert.NoError(t, err) || !assert.NotNil(t, c) {
			return
		}
		defer c.Close()

		buf := make([]byte, 1)
		c.SetReadDeadline(time.Now().Add(time.Second))
		c.Read(buf)

		// This will pass if the client has written a second syn packet by the time it's called. As such we set an
		// unbounded readdeadline on the connection.
		// We can assert behaviours that take a while, but cannot reliably assert behaviours that require fast scheduling
		// of lots of threads on all systems running the CI.
		c.SetReadDeadline(time.Time{})
		if !assert.NoError(t, serial.HandshakeServer(c), "Expected handshake to succeed on 2nd syn packet from client") {
			return
		}
	}

	assert.NoError(t, s.Start())

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go dial()
	}

	done := make(chan bool)
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fail()
	}
	assert.NoError(t, s.Stop())

	_, err := net.Dial("tcp", s.Addr())
	assert.Error(t, err)
}

func TestAttachSshSession(t *testing.T) {
	log.SetLevel(log.InfoLevel)
	if testing.Verbose() {
		log.SetLevel(log.DebugLevel)
	}
	s := NewServer("localhost", 0)
	assert.NoError(t, s.Start())
	defer s.Stop()

	expectedID := "foo"

	// This should block until the ssh server returns its container ID
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err := s.c.Interaction(ctx, expectedID)
		if !assert.NoError(t, err) {
			return
		}
	}()

	// Dial the attach server.  This is a TCP client
	networkClientCon, err := net.Dial("tcp", s.Addr())
	if !assert.NoError(t, err) {
		return
	}

	if !assert.NoError(t, serial.HandshakeServer(networkClientCon)) {
		return
	}

	containerConfig := &ssh.ServerConfig{
		NoClientAuth: true,
	}

	signer, err := ssh.ParsePrivateKey(testdata.PEMBytes["dsa"])
	if !assert.NoError(t, err) {
		return
	}
	containerConfig.AddHostKey(signer)

	// create the SSH server on the client.  The attach server will ssh connect to this.
	sshConn, chans, reqs, err := ssh.NewServerConn(networkClientCon, containerConfig)
	if !assert.NoError(t, err) {
		return
	}
	defer sshConn.Close()

	// Service the incoming Channel channel.
	wg.Add(2)
	go func() {
		defer wg.Done()
		exit := 0
		for req := range reqs {
			if req.Type == msgs.ContainersReq {
				msg := msgs.ContainersMsg{IDs: []string{expectedID}}
				req.Reply(true, msg.Marshal())
				exit++
			}
			if req.Type == msgs.VersionReq {
				msg := msgs.VersionMsg{Version: feature.MaxPluginVersion - 1}
				req.Reply(true, msg.Marshal())
				exit++
			}
			if exit == 2 {
				break
			}
		}
	}()

	go func() {
		defer wg.Done()
		for ch := range chans {
			assert.Equal(t, ch.ChannelType(), attachChannelType)
			_, reqs, _ = ch.Accept()
			for req := range reqs {
				if req.Type == msgs.UnblockReq {
					req.Reply(true, nil)
					break
				}
			}
			break
		}
	}()

	wg.Wait()
}
