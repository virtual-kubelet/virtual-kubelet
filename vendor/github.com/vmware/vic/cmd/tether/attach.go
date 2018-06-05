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
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	log "github.com/Sirupsen/logrus"
	"golang.org/x/crypto/ssh"

	"github.com/vmware/vic/lib/migration/feature"
	"github.com/vmware/vic/lib/tether"
	"github.com/vmware/vic/lib/tether/msgs"
	"github.com/vmware/vic/pkg/serial"
	"github.com/vmware/vic/pkg/trace"
)

const (
	attachChannelType = "attach"
)

// server is the singleton attachServer for the tether - there can be only one
// as the backchannel line protocol may not provide multiplexing of connections
var server AttachServer
var once sync.Once

type AttachServer interface {
	tether.Extension

	start() error
	stop() error
}

// config is a struct that holds Sessions and Execs
type config struct {
	Key []byte

	Sessions map[string]*tether.SessionConfig
	Execs    map[string]*tether.SessionConfig
}

type attachServerSSH struct {
	// serializes data access for exported functions
	m sync.Mutex

	// conn is the underlying net.Conn which carries SSH
	// held directly as it is how we stop the attach server
	conn struct {
		sync.Mutex
		conn net.Conn
	}

	// we pass serverConn to the channelMux goroutine so we need to lock it
	serverConn struct {
		sync.Mutex
		*ssh.ServerConn
	}

	// extension local copy of the bits of config important to attach
	config    config
	sshConfig *ssh.ServerConfig

	enabled int32

	// Cancelable context and its cancel func. Used for resolving the deadlock
	// between run() and stop()
	ctx    context.Context
	cancel context.CancelFunc

	// INTERNAL: must set by testAttachServer only
	testing bool
}

// NewAttachServerSSH either creates a new instance or returns the initialized one
func NewAttachServerSSH() AttachServer {
	once.Do(func() {
		// create a cancelable context and assign it to the CancelFunc
		// it isused for resolving the deadlock between run() and stop()
		// it has a Background parent as we don't want timeouts here,
		// otherwise we may start leaking goroutines in the handshake code
		ctx, cancel := context.WithCancel(context.Background())
		server = &attachServerSSH{
			ctx:    ctx,
			cancel: cancel,
		}
	})
	return server
}

// Reload - tether.Extension implementation
func (t *attachServerSSH) Reload(tconfig *tether.ExecutorConfig) error {
	defer trace.End(trace.Begin("attach reload"))

	t.m.Lock()
	defer t.m.Unlock()

	// We copy this stuff so that we're not referencing the direct config
	// structure if/while it's being updated.
	// The subelements generally have locks or updated in single assignment
	t.config.Key = tconfig.Key
	t.config.Sessions = make(map[string]*tether.SessionConfig)
	for k, v := range tconfig.Sessions {
		t.config.Sessions[k] = v
	}

	t.config.Execs = make(map[string]*tether.SessionConfig)
	for k, v := range tconfig.Execs {
		t.config.Execs[k] = v
	}

	err := server.start()
	if err != nil {
		detail := fmt.Sprintf("unable to start attach server: %s", err)
		log.Error(detail)
		return errors.New(detail)
	}
	return nil
}

// Enable sets the enabled to true
func (t *attachServerSSH) Enable() {
	atomic.StoreInt32(&t.enabled, 1)
}

// Disable sets the enabled to false
func (t *attachServerSSH) Disable() {
	atomic.StoreInt32(&t.enabled, 0)
}

// Enabled returns whether the enabled is true
func (t *attachServerSSH) Enabled() bool {
	return atomic.LoadInt32(&t.enabled) == 1
}

func (t *attachServerSSH) Start(system tether.System) error {
	defer trace.End(trace.Begin(""))

	return nil
}

// Stop needed for tether.Extensions interface
func (t *attachServerSSH) Stop() error {
	defer trace.End(trace.Begin("stop attach server"))

	t.m.Lock()
	defer t.m.Unlock()

	// calling server.start not t.start so that test impl gets invoked
	return server.stop()
}

func (t *attachServerSSH) reload() error {
	t.serverConn.Lock()
	defer t.serverConn.Unlock()

	// push the exec'ed session ids to the portlayer
	if t.serverConn.ServerConn != nil {
		msg := msgs.ContainersMsg{
			IDs: t.sessions(false),
		}
		payload := msg.Marshal()

		ok, _, err := t.serverConn.SendRequest(msgs.ContainersReq, true, payload)
		if !ok || err != nil {
			return fmt.Errorf("failed to send container ids: %s, %t", err, ok)
		}
	}
	return nil
}

func (t *attachServerSSH) start() error {
	defer trace.End(trace.Begin("start attach server"))

	// if we come here while enabled, reload
	if t.Enabled() {
		log.Debugf("Start called while enabled, reloading")
		if err := t.reload(); err != nil {
			log.Warn(err)
		}
		return nil
	}

	// don't assume that the key hasn't changed
	pkey, err := ssh.ParsePrivateKey([]byte(t.config.Key))
	if err != nil {
		detail := fmt.Sprintf("failed to load key for attach: %s", err)
		log.Error(detail)
		return errors.New(detail)
	}

	// An SSH server is represented by a ServerConfig, which holds
	// certificate details and handles authentication of ServerConns.
	// TODO: update this with generated credentials for the appliance
	t.sshConfig = &ssh.ServerConfig{
		PublicKeyCallback: func(c ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			if c.User() == "daemon" {
				return &ssh.Permissions{}, nil
			}
			return nil, fmt.Errorf("expected daemon user")
		},
		PasswordCallback: func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			if c.User() == "daemon" {
				return &ssh.Permissions{}, nil
			}
			return nil, fmt.Errorf("expected daemon user")
		},
		NoClientAuth: true,
	}
	t.sshConfig.AddHostKey(pkey)

	// enable the server and start it
	t.Enable()
	go t.run()

	return nil
}

// stop is not thread safe with start
func (t *attachServerSSH) stop() error {
	defer trace.End(trace.Begin("stop attach server"))

	if t == nil {
		err := fmt.Errorf("attach server is not configured")
		log.Error(err)
		return err
	}

	if !t.Enabled() {
		err := fmt.Errorf("attach server is not enabled")
		log.Error(err)
		return err
	}

	// disable the server
	t.Disable()

	// This context is used by backchannel only. We need to cancel it before
	// trying to obtain the following lock so that backchannel interrupts the
	// underlying Read call by calling Close on it.
	// The lock is held by backchannel's caller and not released until it returns
	log.Debugf("Canceling AttachServer's context")
	t.cancel()

	t.conn.Lock()
	if t.conn.conn != nil {
		log.Debugf("Close called again on rawconn - squashing")
		// #nosec: Errors unhandled.
		t.conn.conn.Close()
		t.conn.conn = nil
	}
	t.conn.Unlock()

	return nil
}

func backchannel(ctx context.Context, conn net.Conn) error {
	defer trace.End(trace.Begin("establish tether backchannel"))

	// used for shutting down the goroutine cleanly otherwise we leak a goroutine for every successful return from this function
	done := make(chan struct{})

	// HACK: currently RawConn dosn't implement timeout so throttle the spinning
	// it does implement the Timeout methods so the intermediary code can be written
	// to support it, but they are stub implementation in rawconn impl.

	// This needs to tick *faster* than the ticker in connection.go on the
	// portlayer side.  The PL sends the first syn and if this isn't waiting,
	// alignment will take a few rounds (or it may never happen).
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	// We run this in a separate goroutine because HandshakeServer
	// calls a Read on rawconn which is a blocking call which causes
	// the caller to block as well so this is the only way to cancel.
	// Calling Close() will unblock us and on the next tick we will
	// return ctx.Err()
	go func() {
		select {
		case <-ctx.Done():
			conn.Close()
		case <-done:
			return
		}
	}()

	for {
		select {
		case <-ticker.C:
			if ctx.Err() != nil {
				return ctx.Err()
			}
			deadline, ok := ctx.Deadline()
			if ok {
				conn.SetReadDeadline(deadline)
			}

			err := serial.HandshakeServer(conn)
			if err == nil {
				conn.SetReadDeadline(time.Time{})
				close(done)
				return nil
			}

			switch et := err.(type) {
			case *serial.HandshakeError:
				log.Debugf("HandshakeServer: %v", et)
			default:
				log.Errorf("HandshakeServer: %v", err)
			}
		}
	}
}

func (t *attachServerSSH) establish() error {
	var err error

	// we hold the t.conn.Lock during the scope of this function
	t.conn.Lock()
	defer t.conn.Unlock()

	// tests are passing their own connections so do not create connections when testing is set
	if !t.testing {
		// close the connection if required
		if t.conn.conn != nil {
			// #nosec: Errors unhandled.
			t.conn.conn.Close()
			t.conn.conn = nil
		}
		t.conn.conn, err = rawConnectionFromSerial()
		if err != nil {
			detail := fmt.Errorf("failed to create raw connection: %s", err)
			log.Error(detail)
			return detail
		}
	} else {
		// A series of unfortunate events can lead calling backchannel with nil when we run unit tests.
		// https://github.com/vmware/vic/pull/5327#issuecomment-305619860
		// This check is here to handle that
		if t.conn.conn == nil {
			return fmt.Errorf("nil connection")
		}
	}

	// wait for backchannel to establish
	err = backchannel(t.ctx, t.conn.conn)
	if err != nil {
		detail := fmt.Errorf("failed to establish backchannel: %s", err)
		log.Error(detail)
		return detail
	}

	return nil
}

func (t *attachServerSSH) cleanup() {
	t.serverConn.Lock()
	defer t.serverConn.Unlock()

	log.Debugf("cleanup on connection")

	if t.serverConn.ServerConn != nil {
		log.Debugf("closing underlying connection")
		t.serverConn.Close()
		t.serverConn.ServerConn = nil
	}
}

// run should not be called directly, but via start
// run will establish an ssh server listening on the backchannel
func (t *attachServerSSH) run() error {
	defer trace.End(trace.Begin("main attach server loop"))

	var established bool

	var chans <-chan ssh.NewChannel
	var reqs <-chan *ssh.Request
	var err error

	// main run loop
	for t.Enabled() {
		t.serverConn.Lock()
		established = t.serverConn.ServerConn != nil
		t.serverConn.Unlock()

		// keep waiting for the connection to establish
		for !established && t.Enabled() {
			log.Infof("Trying to establish a connection")

			if err := t.establish(); err != nil {
				log.Error(err)
				continue
			}

			// create the SSH server using underlying t.conn
			t.serverConn.Lock()

			t.serverConn.ServerConn, chans, reqs, err = ssh.NewServerConn(t.conn.conn, t.sshConfig)
			if err != nil {
				detail := fmt.Errorf("failed to establish ssh handshake: %s", err)
				log.Error(detail)
			}
			established = t.serverConn.ServerConn != nil

			t.serverConn.Unlock()
		}

		// Global requests
		go t.globalMux(reqs, t.cleanup)

		log.Infof("Ready to service attach requests")
		// Service the incoming channels
		for attachchan := range chans {
			// The only channel type we'll support is attach
			if attachchan.ChannelType() != attachChannelType {
				detail := fmt.Sprintf("unknown channel type %s", attachchan.ChannelType())
				attachchan.Reject(ssh.UnknownChannelType, detail)
				log.Error(detail)
				continue
			}

			// check we have a Session matching the requested ID
			bytes := attachchan.ExtraData()
			if bytes == nil {
				detail := "attach channel requires ID in ExtraData"
				attachchan.Reject(ssh.Prohibited, detail)
				log.Error(detail)
				continue
			}

			sessionid := string(bytes)

			s, oks := t.config.Sessions[sessionid]
			e, oke := t.config.Execs[sessionid]
			if !oks && !oke {
				detail := fmt.Sprintf("session %s is invalid", sessionid)
				attachchan.Reject(ssh.Prohibited, detail)
				log.Error(detail)
				continue
			}

			// we have sessionid
			session := s
			if oke {
				session = e
			}

			// session is potentially blocked in launch until we've got the unblock message, so we cannot lock it.
			// check that session is valid
			// The detail remains concise as it'll eventually make its way to the user
			if session.Started != "" && session.Started != "true" {
				detail := fmt.Sprintf("launch failed with: %s", session.Started)
				attachchan.Reject(ssh.Prohibited, detail)
				log.Error(detail)
				continue
			}

			if session.StopTime != 0 {
				detail := fmt.Sprintf("process finished with exit code: %d", session.ExitStatus)
				attachchan.Reject(ssh.Prohibited, detail)
				log.Error(detail)
				continue
			}

			channel, requests, err := attachchan.Accept()
			if err != nil {
				detail := fmt.Sprintf("could not accept channel: %s", err)
				log.Errorf(detail)
				continue
			}

			// bind the channel to the Session
			log.Debugf("binding reader/writers for channel for %s", sessionid)

			log.Debugf("Adding [%p] to Outwriter", channel)
			session.Outwriter.Add(channel)
			log.Debugf("Adding [%p] to Reader", channel)
			session.Reader.Add(channel)

			// cleanup on detach from the session
			cleanup := func() {
				log.Debugf("Cleanup on detach from the session")

				log.Debugf("Removing [%p] from Outwriter", channel)
				session.Outwriter.Remove(channel)

				log.Debugf("Removing [%p] from Reader", channel)
				session.Reader.Remove(channel)

				channel.Close()
			}

			detach := cleanup
			// tty's merge stdout and stderr so we don't bind an additional reader in that case but we need to do so for non-tty
			if !session.Tty {
				// persist the value as we end up with different values each time we access it
				stderr := channel.Stderr()

				log.Debugf("Adding [%p] to Errwriter", stderr)
				session.Errwriter.Add(stderr)

				detach = func() {
					log.Debugf("Cleanup on detach from the session (non-tty)")

					log.Debugf("Removing [%p] from Errwriter", stderr)
					session.Errwriter.Remove(stderr)

					cleanup()
				}
			}
			log.Debugf("reader/writers bound for channel for %s", sessionid)

			go t.channelMux(requests, session, detach)
		}
		log.Info("Incoming attach channel closed")
	}
	return nil
}

func (t *attachServerSSH) sessions(all bool) []string {
	defer trace.End(trace.Begin(""))

	var keys []string

	// this iterates the local copies of the sessions maps
	// so we don't need to care whether they're initialized or not
	// as extension reload comes after that point

	// whether include sessions or not
	if all {
		for k, v := range t.config.Sessions {
			if v.Active && v.StopTime == 0 {
				keys = append(keys, k)
			}
		}
	}

	for k, v := range t.config.Execs {
		// skip those that have had launch errors
		if v.Active && v.StopTime == 0 && (v.Started == "" || v.Started == "true") {
			keys = append(keys, k)
		}
	}

	log.Debugf("Returning %d keys", len(keys))
	return keys
}

func (t *attachServerSSH) globalMux(in <-chan *ssh.Request, cleanup func()) {
	defer trace.End(trace.Begin("attach server global request handler"))

	// cleanup function passed by the caller
	defer cleanup()

	// for the actions after we process the request
	var pendingFn func()
	for req := range in {
		var payload []byte
		ok := true

		log.Infof("received global request type %v", req.Type)

		switch req.Type {
		case msgs.ContainersReq:
			msg := msgs.ContainersMsg{
				IDs: t.sessions(true),
			}
			payload = msg.Marshal()
		case msgs.VersionReq:
			msg := msgs.VersionMsg{
				Version: feature.MaxPluginVersion - 1,
			}
			payload = msg.Marshal()
		default:
			ok = false
			payload = []byte("unknown global request type: " + req.Type)
		}

		log.Debugf("Returning payload: %s", string(payload))

		// make sure that errors get send back if we failed
		if req.WantReply {
			log.Debugf("Sending global request reply %t back with %#v", ok, payload)
			if err := req.Reply(ok, payload); err != nil {
				log.Warnf("Failed to reply a global request back")
			}
		}

		// run any pending work now that a reply has been sent
		if pendingFn != nil {
			log.Debug("Invoking pending work for global mux")
			go pendingFn()
			pendingFn = nil
		}
	}
}

func (t *attachServerSSH) channelMux(in <-chan *ssh.Request, session *tether.SessionConfig, cleanup func()) {
	defer trace.End(trace.Begin("attach server channel request handler"))

	// cleanup function passed by the caller
	defer cleanup()

	// for the actions after we process the request
	var pendingFn func()
	for req := range in {
		ok := true
		abort := false

		log.Infof("received channel mux type %v", req.Type)

		switch req.Type {
		case msgs.PingReq:
			log.Infof("Received PingReq for %s", session.ID)

			if string(req.Payload) != msgs.PingMsg {
				log.Infof("Received corrupted PingReq for %s", session.ID)
				ok = false
			}
		case msgs.UnblockReq:
			log.Infof("Received UnblockReq for %s", session.ID)

			if string(req.Payload) != msgs.UnblockMsg {
				log.Infof("Received corrupted UnblockReq for %s", session.ID)
				ok = false
				break
			}

			// if the process has exited, or couldn't launch
			if session.Started != "" && session.Started != "true" {
				// we need to force the session closed so that error handling occurs on the callers
				// side
				ok = false
				abort = true
			} else {
				// unblock ^ (above)
				pendingFn = session.Unblock()
			}

		case msgs.WindowChangeReq:
			session.Lock()
			pty := session.Pty
			session.Unlock()

			msg := msgs.WindowChangeMsg{}
			if pty == nil {
				ok = false
				log.Errorf("illegal window-change request for non-tty")
			} else if err := msg.Unmarshal(req.Payload); err != nil {
				ok = false
				log.Errorf(err.Error())
			} else if err := resizePty(pty.Fd(), &msg); err != nil {
				ok = false
				log.Errorf(err.Error())
			}
		case msgs.CloseStdinReq:
			log.Infof("Received CloseStdinReq for %s", session.ID)

			log.Debugf("Configuring reader to propagate EOF for %s", session.ID)
			session.Reader.PropagateEOF(true)
		default:
			ok = false
			log.Error(fmt.Sprintf("ssh request type %s is not supported", req.Type))
		}

		// payload is ignored on channel specific replies.  The ok is passed, however.
		if req.WantReply {
			log.Debugf("Sending channel request reply %t back", ok)
			if err := req.Reply(ok, nil); err != nil {
				log.Warnf("Failed replying to a channel request: %s", err)
			}
		}

		// run any pending work now that a reply has been sent
		if pendingFn != nil {
			log.Debug("Invoking pending work for channel mux")
			go pendingFn()
			pendingFn = nil
		}

		if abort {
			break
		}
	}
}

// The syscall struct
type winsize struct {
	wsRow    uint16
	wsCol    uint16
	wsXpixel uint16
	wsYpixel uint16
}
