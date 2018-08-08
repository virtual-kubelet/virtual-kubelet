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

package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"sync"
	"syscall"
	"testing"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/ssh"

	"github.com/vmware/vic/lib/config/executor"

	"github.com/vmware/vic/lib/portlayer/attach/communication"

	"github.com/vmware/vic/lib/migration/feature"
	"github.com/vmware/vic/lib/tether"
	"github.com/vmware/vic/pkg/serial"
)

type testAttachServer struct {
	attachServerSSH
	enabled bool
	updated chan bool
}

func (t *testAttachServer) start() error {
	t.testing = true
	err := t.attachServerSSH.start()
	if err == nil {
		t.updated <- true
		t.enabled = true
	}

	log.Info("Started test attach server")
	return err
}

func (t *testAttachServer) stop() error {
	if t.enabled {
		err := t.attachServerSSH.stop()
		if err == nil {
			log.Info("Stopped test attach server")
			t.updated <- true
			t.enabled = false
		}
		return err
	}
	return nil
}

func (t *testAttachServer) Reload(config *tether.ExecutorConfig) error {
	log.Info("Parsing config in test attach server")
	return t.attachServerSSH.Reload(config)
}

func (t *testAttachServer) Start(sys tether.System) error {
	log.Info("opening ttyS0 pipe pair for backchannel (server)")
	c, err := os.OpenFile(path.Join(pathPrefix, "ttyS0c"), os.O_WRONLY|syscall.O_NOCTTY, 0777)
	if err != nil {
		detail := fmt.Sprintf("failed to open cpipe for backchannel: %s", err)
		log.Error(detail)
		return errors.New(detail)
	}

	s, err := os.OpenFile(path.Join(pathPrefix, "ttyS0s"), os.O_RDONLY|syscall.O_NOCTTY, 0777)
	if err != nil {
		detail := fmt.Sprintf("failed to open spipe for backchannel: %s", err)
		log.Error(detail)
		return errors.New(detail)
	}

	log.Infof("creating raw connection from ttyS0 pipe pair for server (c=%d, s=%d) %s\n", c.Fd(), s.Fd(), pathPrefix)
	conn, err := serial.NewHalfDuplexFileConn(s, c, path.Join(pathPrefix, "ttyS0"), "file")
	if err != nil {
		detail := fmt.Sprintf("failed to create raw connection from ttyS0 pipe pair: %s", err)
		log.Error(detail)
		return errors.New(detail)
	}

	t.conn.Lock()
	defer t.conn.Unlock()

	t.conn.conn = conn
	return nil
}

func (t *testAttachServer) Stop() error {
	return t.attachServerSSH.Stop()
}

// create client on the mock pipe
func mockBackChannel(ctx context.Context) (net.Conn, error) {
	log.Info("opening ttyS0 pipe pair for backchannel (client)")
	c, err := os.OpenFile(path.Join(pathPrefix, "ttyS0c"), os.O_RDONLY|syscall.O_NOCTTY, 0777)
	if err != nil {
		detail := fmt.Sprintf("failed to open cpipe for backchannel: %s", err)
		log.Error(detail)
		return nil, errors.New(detail)
	}

	s, err := os.OpenFile(path.Join(pathPrefix, "ttyS0s"), os.O_WRONLY|syscall.O_NOCTTY, 0777)
	if err != nil {
		detail := fmt.Sprintf("failed to open spipe for backchannel: %s", err)
		log.Error(detail)
		return nil, errors.New(detail)
	}

	log.Infof("creating raw connection from ttyS0 pipe pair for backchannel (c=%d, s=%d) %s\n", c.Fd(), s.Fd(), pathPrefix)
	conn, err := serial.NewHalfDuplexFileConn(c, s, path.Join(pathPrefix, "ttyS0"), "file")

	if err != nil {
		detail := fmt.Sprintf("failed to create raw connection from ttyS0 pipe pair: %s", err)
		log.Error(detail)
		return nil, errors.New(detail)
	}

	// HACK: currently RawConn dosn't implement timeout so throttle the spinning
	ticker := time.NewTicker(1000 * time.Millisecond)
	for {
		select {
		case <-ticker.C:
			err := serial.HandshakeClient(conn)
			if err != nil {
				if err == io.EOF {
					// with unix pipes the open will block until both ends are open, therefore
					// EOF means the other end has been intentionally closed
					return nil, err
				}
				log.Error(err)
			} else {
				return conn, nil
			}
		case <-ctx.Done():
			conn.Close()
			ticker.Stop()
			return nil, ctx.Err()
		}
	}
}

// create client on the mock pipe and dial the given host:port
func mockNetworkToSerialConnection(host string) (*sync.WaitGroup, error) {
	log.Info("opening ttyS0 pipe pair for backchannel")
	c, err := os.OpenFile(path.Join(pathPrefix, "ttyS0c"), os.O_RDONLY|syscall.O_NOCTTY, 0777)
	if err != nil {
		return nil, fmt.Errorf("failed to open cpipe for backchannel: %s", err)
	}

	s, err := os.OpenFile(path.Join(pathPrefix, "ttyS0s"), os.O_WRONLY|syscall.O_NOCTTY, 0777)
	if err != nil {
		return nil, fmt.Errorf("failed to open spipe for backchannel: %s", err)
	}

	log.Infof("creating raw connection from ttyS0 pipe pair (c=%d, s=%d)\n", c.Fd(), s.Fd())
	conn, err := serial.NewHalfDuplexFileConn(c, s, path.Join(pathPrefix, "/ttyS0"), "file")
	if err != nil {
		return nil, fmt.Errorf("failed to create raw connection from ttyS0 pipe pair: %s", err)
	}

	// Dial the attach server.  This is a TCP client
	networkClientCon, err := net.Dial("tcp", host)
	if err != nil {
		return nil, err
	}
	log.Debugf("dialed %s", host)

	wg := sync.WaitGroup{}
	wg.Add(2)

	go func() {
		io.Copy(networkClientCon, conn)
		wg.Done()
	}()

	go func() {
		io.Copy(conn, networkClientCon)
		wg.Done()
	}()

	return &wg, nil
}

func genKey() []byte {
	// generate a host key for the tether
	privateKey, err := rsa.GenerateKey(rand.Reader, 512)
	if err != nil {
		panic("unable to generate private key during test")
	}

	privateKeyDer := x509.MarshalPKCS1PrivateKey(privateKey)
	privateKeyBlock := pem.Block{
		Type:    "RSA PRIVATE KEY",
		Headers: nil,
		Bytes:   privateKeyDer,
	}

	return pem.EncodeToMemory(&privateKeyBlock)
}

func attachCase(t *testing.T, runblock bool) {
	mocker := testSetup(t)
	defer testTeardown(t, mocker)

	testServer, _ := server.(*testAttachServer)

	cfg := executor.ExecutorConfig{
		Diagnostics: executor.Diagnostics{
			DebugLevel: 1,
		},
		ExecutorConfigCommon: executor.ExecutorConfigCommon{
			ID:   "attach",
			Name: "tether_test_executor",
		},

		Sessions: map[string]*executor.SessionConfig{
			"attach": {
				Common: executor.Common{
					ID:   "attach",
					Name: "tether_test_session",
				},
				Tty:    false,
				Attach: true,
				Active: true,

				OpenStdin: true,
				RunBlock:  runblock,
				Cmd: executor.Cmd{
					Path: "/usr/bin/tee",
					// grep, matching everything, reading from stdin
					Args: []string{"/usr/bin/tee", pathPrefix + "/tee.out"},
					Env:  []string{},
					Dir:  "/",
				},
			},
		},
		Key: genKey(),
	}

	_, _, conn := StartAttachTether(t, &cfg, mocker)
	defer conn.Close()

	// wait for updates to occur
	<-testServer.updated

	if !testServer.enabled {
		t.Errorf("attach server was not enabled")
	}

	containerConfig := &ssh.ClientConfig{
		User: "daemon",
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		},
	}

	// create the SSH client from the mocked connection
	sshConn, chans, reqs, err := ssh.NewClientConn(conn, "notappliable", containerConfig)
	assert.NoError(t, err)
	defer sshConn.Close()

	ssh.NewClient(sshConn, chans, reqs)
	_, err = communication.ContainerIDs(sshConn)
	version, err := communication.ContainerVersion(sshConn)
	assert.NoError(t, err)
	sshSession, err := communication.NewSSHInteraction(sshConn, cfg.ID, version)
	if runblock {
		sshSession.Unblock()
	}
	assert.NoError(t, err)

	stdout := sshSession.Stdout()

	// FIXME: the pipe pair are line buffered - how do I disable that so we
	// don't have odd hangs to diagnose when the trailing \n is missed

	testBytes := []byte("\x1b[32mhello world\x1b[39m!\n")
	// read from session into buffer
	buf := &bytes.Buffer{}
	done := make(chan bool)
	go func() { io.Copy(buf, stdout); done <- true }()

	// write something to echo
	log.Debug("sending test data")
	sshSession.Stdin().Write(testBytes)
	log.Debug("sent test data")

	// wait for the close to propagate
	sshSession.CloseStdin()
	<-done
	// sshSession.Close()
}

func TestAttach(t *testing.T) {
	attachCase(t, false)
}

func TestAttachBlock(t *testing.T) {
	attachCase(t, true)
}

//
/////////////////////////////////////////////////////////////////////////////////////

/////////////////////////////////////////////////////////////////////////////////////
// TestAttachTTYConfig sets up the config for attach testing
//
func TestAttachTTY(t *testing.T) {
	t.Skip("TTY test skipped - not sure how to test this correctly")

	mocker := testSetup(t)
	defer testTeardown(t, mocker)

	testServer, _ := server.(*testAttachServer)

	cfg := executor.ExecutorConfig{
		ExecutorConfigCommon: executor.ExecutorConfigCommon{
			ID:   "attach",
			Name: "tether_test_executor",
		},

		Sessions: map[string]*executor.SessionConfig{
			"attach": {
				Common: executor.Common{
					ID:   "attach",
					Name: "tether_test_session",
				},
				Tty:    true,
				Attach: true,
				Active: true,

				OpenStdin: true,
				Cmd: executor.Cmd{
					Path: "/usr/bin/tee",
					// grep, matching everything, reading from stdin
					Args: []string{"/usr/bin/tee", pathPrefix + "/tee.out"},
					Env:  []string{},
					Dir:  "/",
				},
			},
		},
		Key: genKey(),
	}

	_, _, conn := StartAttachTether(t, &cfg, mocker)
	defer conn.Close()

	// wait for updates to occur
	<-testServer.updated

	if !testServer.enabled {
		t.Errorf("attach server was not enabled")
	}

	cconfig := &ssh.ClientConfig{
		User: "daemon",
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		},
	}

	// create the SSH client
	sConn, chans, reqs, err := ssh.NewClientConn(conn, "notappliable", cconfig)
	assert.NoError(t, err)

	defer sConn.Close()
	client := ssh.NewClient(sConn, chans, reqs)

	session, err := communication.NewSSHInteraction(client, cfg.ID, feature.MaxPluginVersion-1)
	assert.NoError(t, err)

	stdout := session.Stdout()

	// FIXME: this is line buffered - how do I disable that so we don't have odd hangs to diagnose
	// when the trailing \n is missed
	testBytes := []byte("\x1b[32mhello world\x1b[39m!\n")
	// after tty translation the above string should result in the following
	refBytes := []byte("\x5e[[32mhello world\x5e[[39m!\n")

	// read from session into buffer
	buf := &bytes.Buffer{}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		io.CopyN(buf, stdout, int64(len(refBytes)))
		wg.Done()
	}()

	// write something to echo
	log.Debug("sending test data")
	session.Stdin().Write(testBytes)
	log.Debug("sent test data")

	// wait for the close to propagate
	wg.Wait()
	session.CloseStdin()

	assert.Equal(t, refBytes, buf.Bytes())
}

//
/////////////////////////////////////////////////////////////////////////////////////

/////////////////////////////////////////////////////////////////////////////////////
// TestAttachTTYStdinClose sets up the config for attach testing
//
func TestAttachTTYStdinClose(t *testing.T) {
	mocker := testSetup(t)
	defer testTeardown(t, mocker)

	testServer, _ := server.(*testAttachServer)

	cfg := executor.ExecutorConfig{
		ExecutorConfigCommon: executor.ExecutorConfigCommon{
			ID:   "sort",
			Name: "tether_test_executor",
		},

		Diagnostics: executor.Diagnostics{
			DebugLevel: 1,
		},

		Sessions: map[string]*executor.SessionConfig{
			"sort": {
				Common: executor.Common{
					ID:   "sort",
					Name: "tether_test_session",
				},
				Tty:    true,
				Attach: true,
				Active: true,

				OpenStdin: true,
				RunBlock:  true,

				Cmd: executor.Cmd{
					Path: "/usr/bin/sort",
					// reading from stdin
					Args: []string{"/usr/bin/sort"},
					Env:  []string{},
					Dir:  "/",
				},
			},
		},
		Key: genKey(),
	}

	_, _, conn := StartAttachTether(t, &cfg, mocker)
	defer conn.Close()

	// wait for updates to occur
	<-testServer.updated

	if !testServer.enabled {
		t.Errorf("attach server was not enabled")
	}

	containerConfig := &ssh.ClientConfig{
		User: "daemon",
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		},
	}

	// create the SSH client from the mocked connection
	sshConn, chans, reqs, err := ssh.NewClientConn(conn, "notappliable", containerConfig)
	assert.NoError(t, err)
	defer sshConn.Close()

	ssh.NewClient(sshConn, chans, reqs)
	_, err = communication.ContainerIDs(sshConn)
	assert.NoError(t, err)
	sshSession, err := communication.NewSSHInteraction(sshConn, cfg.ID, feature.MaxPluginVersion-1)
	assert.NoError(t, err)

	// unblock before grabbing stdout - this should buffer in ssh
	sshSession.Unblock()
	stdout := sshSession.Stdout()

	// FIXME: the pipe pair are line buffered - how do I disable that so we
	// don't have odd hangs to diagnose when the trailing \n is missed

	testBytes := []byte("one\ntwo\nthree\n")
	// after tty translation by sort the above string should result in the following
	// - we have echo turned on so we get a repeat of the initial string
	// - all \n bytes are translated to \r\n
	refBytes := []byte("one\r\ntwo\r\nthree\r\none\r\nthree\r\ntwo\r\n")

	// read from session into buffer
	buf := &bytes.Buffer{}
	done := make(chan bool)
	go func() {
		io.Copy(buf, stdout)
		log.Debug("stdout copy complete")
		done <- true
	}()

	// write something to echo
	log.Debug("sending test data")
	sshSession.Stdin().Write(testBytes)
	log.Debug("sent test data")

	// wait for the close to propagate
	sshSession.CloseStdin()
	<-done

	assert.Equal(t, refBytes, buf.Bytes())
}

//
/////////////////////////////////////////////////////////////////////////////////////

/////////////////////////////////////////////////////////////////////////////////////
// TestEcho ensures we get back data without a tty and without any stdin interaction
//
func TestEcho(t *testing.T) {
	mocker := testSetup(t)
	defer testTeardown(t, mocker)

	testServer, _ := server.(*testAttachServer)

	cfg := executor.ExecutorConfig{
		ExecutorConfigCommon: executor.ExecutorConfigCommon{
			ID:   "echo",
			Name: "tether_test_executor",
		},

		Diagnostics: executor.Diagnostics{
			DebugLevel: 1,
		},

		Sessions: map[string]*executor.SessionConfig{
			"echo": {
				Common: executor.Common{
					ID:   "echo",
					Name: "tether_test_session",
				},
				Tty:    false,
				Attach: true,
				Active: true,

				OpenStdin: true,
				RunBlock:  true,

				Cmd: executor.Cmd{
					Path: "/bin/echo",
					// reading from stdin
					Args: []string{"/bin/echo", "hello"},
					Env:  []string{},
					Dir:  "/",
				},
			},
		},
		Key: genKey(),
	}

	_, _, conn := StartAttachTether(t, &cfg, mocker)
	defer conn.Close()

	// wait for updates to occur
	<-testServer.updated

	if !testServer.enabled {
		t.Errorf("attach server was not enabled")
	}

	containerConfig := &ssh.ClientConfig{
		User: "daemon",
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		},
	}

	// create the SSH client from the mocked connection
	sshConn, chans, reqs, err := ssh.NewClientConn(conn, "notappliable", containerConfig)
	assert.NoError(t, err)
	defer sshConn.Close()

	ssh.NewClient(sshConn, chans, reqs)
	_, err = communication.ContainerIDs(sshConn)
	assert.NoError(t, err)
	version, err := communication.ContainerVersion(sshConn)
	assert.NoError(t, err)

	sshSession, err := communication.NewSSHInteraction(sshConn, cfg.ID, version)
	assert.NoError(t, err)

	// unblock before grabbing stdout - this should buffer in ssh
	sshSession.Unblock()
	stdout := sshSession.Stdout()
	stderr := sshSession.Stderr()

	doneStdout := make(chan bool)
	doneStderr := make(chan bool)

	// read from session into buffer
	bufout := &bytes.Buffer{}
	go func() {
		io.Copy(bufout, stdout)
		log.Debug("stdout copy complete")
		doneStdout <- true
	}()

	// read from session into buffer
	buferr := &bytes.Buffer{}
	go func() {
		io.Copy(buferr, stderr)
		log.Debug("stderr copy complete")
		doneStderr <- true
	}()

	// wait for the close to propagate
	<-doneStdout
	assert.Equal(t, "hello\n", string(bufout.Bytes()))

	<-doneStderr
	assert.Equal(t, "", string(buferr.Bytes()))
}

func TestEchoRepeat(t *testing.T) {
	log.SetLevel(log.WarnLevel)

	for i := 0; i < 10 && !t.Failed(); i++ {
		TestEcho(t)
	}

	defer log.SetLevel(log.DebugLevel)
}

//
/////////////////////////////////////////////////////////////////////////////////////

/////////////////////////////////////////////////////////////////////////////////////
// TestAttachMultiple sets up the config for attach testing - tests launching and
// attaching to multiple processes simultaneously
//
func TestAttachMultiple(t *testing.T) {
	mocker := testSetup(t)
	defer testTeardown(t, mocker)

	testServer, _ := server.(*testAttachServer)

	cfg := executor.ExecutorConfig{
		ExecutorConfigCommon: executor.ExecutorConfigCommon{
			ID:   "tee1",
			Name: "tether_test_executor",
		},

		Sessions: map[string]*executor.SessionConfig{
			"tee1": {
				Common: executor.Common{
					ID:   "tee1",
					Name: "tether_test_session",
				},
				Tty:    false,
				Attach: true,
				Active: true,

				OpenStdin: true,
				Cmd: executor.Cmd{
					Path: "/usr/bin/tee",
					// grep, matching everything, reading from stdin
					Args: []string{"/usr/bin/tee", pathPrefix + "/tee1.out"},
					Env:  []string{},
					Dir:  "/",
				},
			},
			"tee2": {
				Common: executor.Common{
					ID:   "tee2",
					Name: "tether_test_session2",
				},
				Tty:    false,
				Attach: true,
				Active: true,

				OpenStdin: true,
				Cmd: executor.Cmd{
					Path: "/usr/bin/tee",
					// grep, matching everything, reading from stdin
					Args: []string{"/usr/bin/tee", pathPrefix + "/tee2.out"},
					Env:  []string{},
					Dir:  "/",
				},
			},
			"tee3": {
				Common: executor.Common{
					ID:   "tee3",
					Name: "tether_test_session2",
				},
				Tty:    false,
				Attach: false,
				Active: true,

				OpenStdin: true,
				Cmd: executor.Cmd{
					Path: "/usr/bin/tee",
					// grep, matching everything, reading from stdin
					Args: []string{"/usr/bin/tee", pathPrefix + "/tee3.out"},
					Env:  []string{},
					Dir:  "/",
				},
			},
		},
		Key: genKey(),
		Diagnostics: executor.Diagnostics{
			DebugLevel: 1,
		},
	}

	_, _, conn := StartAttachTether(t, &cfg, mocker)
	defer conn.Close()

	// wait for updates to occur
	<-mocker.Started
	<-testServer.updated

	if !testServer.enabled {
		t.Errorf("attach server was not enabled")
	}

	cconfig := &ssh.ClientConfig{
		User: "daemon",
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		},
	}

	// create the SSH client
	sConn, chans, reqs, err := ssh.NewClientConn(conn, "notappliable", cconfig)
	assert.NoError(t, err)

	defer sConn.Close()
	client := ssh.NewClient(sConn, chans, reqs)

	ids, err := communication.ContainerIDs(client)
	assert.NoError(t, err)
	version, err := communication.ContainerVersion(client)
	assert.NoError(t, err)

	// there's no ordering guarantee in the returned ids
	if len(ids) != len(cfg.Sessions) {
		t.Errorf("ID list - expected %d, got %d", len(cfg.Sessions), len(ids))
	}

	// check the ids we got correspond to those in the config
	for _, id := range ids {
		if _, ok := cfg.Sessions[id]; !ok {
			t.Errorf("Expected sessions to have an entry for %s", id)
		}
	}

	sessionA, err := communication.NewSSHInteraction(client, "tee1", version)
	assert.NoError(t, err)

	sessionB, err := communication.NewSSHInteraction(client, "tee2", version)
	assert.NoError(t, err)

	stdoutA := sessionA.Stdout()
	stdoutB := sessionB.Stdout()

	// FIXME: this is line buffered - how do I disable that so we don't have odd hangs to diagnose
	// when the trailing \n is missed
	testBytesA := []byte("hello world!\n")
	testBytesB := []byte("goodbye world!\n")
	// read from session into buffer
	bufA := &bytes.Buffer{}
	bufB := &bytes.Buffer{}

	var wg sync.WaitGroup
	// wg.Add cannot go inside the go routines as the Add may not have happened by the time we call Wait
	wg.Add(2)
	go func() {
		io.CopyN(bufA, stdoutA, int64(len(testBytesA)))
		wg.Done()
	}()
	go func() {
		io.CopyN(bufB, stdoutB, int64(len(testBytesB)))
		wg.Done()
	}()

	// write something to echo
	log.Debug("sending test data")
	sessionA.Stdin().Write(testBytesA)
	sessionB.Stdin().Write(testBytesB)
	log.Debug("sent test data")

	// wait for the close to propagate
	wg.Wait()

	sessionA.CloseStdin()
	sessionB.CloseStdin()

	<-mocker.Cleaned

	assert.Equal(t, bufA.Bytes(), testBytesA)
	assert.Equal(t, bufB.Bytes(), testBytesB)
}

//
/////////////////////////////////////////////////////////////////////////////////////

/////////////////////////////////////////////////////////////////////////////////////
// TestAttachInvalid sets up the config for attach testing - launches a process but
// tries to attach to an invalid session id
//
func TestAttachInvalid(t *testing.T) {
	mocker := testSetup(t)
	defer testTeardown(t, mocker)

	testServer, _ := server.(*testAttachServer)

	cfg := executor.ExecutorConfig{
		ExecutorConfigCommon: executor.ExecutorConfigCommon{
			ID:   "attachinvalid",
			Name: "tether_test_executor",
		},

		Sessions: map[string]*executor.SessionConfig{
			"valid": {
				Common: executor.Common{
					ID:   "valid",
					Name: "tether_test_session",
				},
				Tty:       false,
				Attach:    true,
				Active:    true,
				OpenStdin: true,
				Cmd: executor.Cmd{
					Path: "/usr/bin/tee",
					// grep, matching everything, reading from stdin
					Args: []string{"/usr/bin/tee", pathPrefix + "/tee.out"},
					Env:  []string{},
					Dir:  "/",
				},
			},
		},
		Key: genKey(),
	}

	tthr, _, conn := StartAttachTether(t, &cfg, mocker)
	defer conn.Close()

	// wait for updates to occur
	<-testServer.updated

	if !testServer.enabled {
		t.Errorf("attach server was not enabled")
	}

	cconfig := &ssh.ClientConfig{
		User: "daemon",
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		},
	}

	// create the SSH client
	sConn, chans, reqs, err := ssh.NewClientConn(conn, "notappliable", cconfig)
	assert.NoError(t, err)
	defer sConn.Close()

	client := ssh.NewClient(sConn, chans, reqs)

	version, err := communication.ContainerVersion(client)
	assert.NoError(t, err)

	_, err = communication.NewSSHInteraction(client, "invalid", version)
	tthr.Stop()
	if err == nil {
		t.Errorf("Expected to fail on attempt to attach to invalid session")
	}
}

//
/////////////////////////////////////////////////////////////////////////////////////

// Start the tether, start a mock esx serial to tcp connection, start the
// attach server, try to Get() the tether's attached session.
/*
func TestMockAttachTetherToPL(t *testing.T) {
	testSetup(t)
	defer testTeardown(t)

	// Start the PL attach server
	testServer := communication.NewAttachServer("", 8080)
	assert.NoError(t, testServer.Start())
	defer testServer.Stop()

	cfg := executor.ExecutorConfig{
		Common: executor.Common{
			ID:   "attach",
			Name: "tether_test_executor",
		},

		Sessions: map[string]*executor.SessionConfig{
			"attach": executor.SessionConfig{
				Common: executor.Common{
					ID:   "attach",
					Name: "tether_test_session",
				},
				Tty:    true,
				Attach: true,
				Cmd: executor.Cmd{
					Path: "/usr/bin/tee",
					// grep, matching everything, reading from stdin
					Args: []string{"/usr/bin/tee", pathPrefix + "/tee.out"},
					Env:  []string{},
					Dir:  "/",
				},
			},
		},
		Key: genKey(),
	}

	StartTether(t, &cfg)

	// create a conn on the mock pipe.  Reads from pipe, echos to network.
	_, err := mockNetworkToSerialConnection(testServer.Addr())
	if !assert.NoError(t, err) {
		return
	}

	var pty communication.SessionInteractor
	pty, err = testServer.Get(context.Background(), "attach", 600*time.Second)
	if !assert.NoError(t, err) {
		return
	}

	err = pty.Resize(1, 2, 3, 4)
	if !assert.NoError(t, err) {
		return
	}
	if !assert.Equal(t, Mocked.WindowCol, uint32(1)) || !assert.Equal(t, Mocked.WindowRow, uint32(2)) {
		return
	}

	if err = pty.Signal("HUP"); !assert.NoError(t, err) {
		return
	}
	if !assert.Equal(t, Mocked.Signal, ssh.Signal("HUP")) {
		return
	}
}
*/

func TestReattach(t *testing.T) {
	mocker := testSetup(t)
	defer testTeardown(t, mocker)

	testServer, _ := server.(*testAttachServer)

	cfg := executor.ExecutorConfig{
		ExecutorConfigCommon: executor.ExecutorConfigCommon{
			ID:   "attach",
			Name: "tether_test_executor",
		},
		Diagnostics: executor.Diagnostics{
			DebugLevel: 1,
		},

		Sessions: map[string]*executor.SessionConfig{
			"attach": {
				Common: executor.Common{
					ID:   "attach",
					Name: "tether_test_session",
				},
				Tty:    false,
				Attach: true,
				Active: true,

				RunBlock:  true,
				OpenStdin: true,
				Cmd: executor.Cmd{
					Path: "/usr/bin/tee",
					// grep, matching everything, reading from stdin
					Args: []string{"/usr/bin/tee", pathPrefix + "/tee.out"},
					Env:  []string{},
					Dir:  "/",
				},
			},
		},
		Key: genKey(),
	}

	_, _, conn := StartAttachTether(t, &cfg, mocker)
	defer conn.Close()

	// wait for updates to occur
	<-testServer.updated

	if !testServer.enabled {
		t.Errorf("attach server was not enabled")
	}

	containerConfig := &ssh.ClientConfig{
		User: "daemon",
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		},
	}

	// create the SSH client from the mocked connection
	sshConn, chans, reqs, err := ssh.NewClientConn(conn, "notappliable", containerConfig)
	assert.NoError(t, err)
	defer sshConn.Close()

	var sshSession communication.SessionInteractor
	done := make(chan bool)
	buf := &bytes.Buffer{}
	testBytes := []byte("\x1b[32mhello world\x1b[39m!\n")

	attachFunc := func() {
		attachClient := ssh.NewClient(sshConn, chans, reqs)
		if attachClient == nil {
			t.Errorf("Failed to get ssh.NewClient")
		}

		_, err = communication.ContainerIDs(sshConn)
		assert.NoError(t, err)

		version, err := communication.ContainerVersion(sshConn)
		assert.NoError(t, err)

		sshSession, err = communication.NewSSHInteraction(sshConn, cfg.ID, version)
		assert.NoError(t, err)

		sshSession.Unblock()

		stdout := sshSession.Stdout()

		// read from session into buffer
		go func() {
			io.CopyN(buf, stdout, int64(len(testBytes)))
			done <- true
		}()

		// write something to echo
		log.Debug("sending test data")
		sshSession.Stdin().Write(testBytes)
		log.Debug("sent test data")
	}

	limit := 10
	for i := 0; i <= limit; i++ {
		if i > 0 {
			// truncate the buffer for the retach
			buf.Reset()
			testBytes = []byte(fmt.Sprintf("\x1b[32mhello world - again %dth time \x1b[39m!\n", i))
		}

		// attach
		attachFunc()

		// wait for the close to propagate
		<-done

		// send close-stdin if this is the last iteration
		if i == limit {
			// exit
			sshSession.CloseStdin()
		} else {
			// detach
			sshSession.Stdin().Close()
		}
		assert.Equal(t, buf.Bytes(), testBytes)
	}
}
