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

package communication

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"

	"github.com/vmware/vic/lib/tether/msgs"
	"github.com/vmware/vic/pkg/errors"
	"github.com/vmware/vic/pkg/serial"
	"github.com/vmware/vic/pkg/trace"

	"golang.org/x/crypto/ssh"
	"golang.org/x/sync/singleflight"
)

const (
	VersionString = "SSH-2.0-VIC"
	ClientTimeout = 10 * time.Second
)

// Connector defines the connection and interactions
type Connector struct {
	mutex        sync.RWMutex
	cond         *sync.Cond
	interactions map[string]*LazySessionInteractor

	listener net.Listener
	// Quit channel for serve
	done chan struct{}

	// deduplication of incoming calls
	fg singleflight.Group

	// graceful shutdown
	wg sync.WaitGroup
}

// NewConnector returns a new Connector
func NewConnector(listener net.Listener) *Connector {
	defer trace.End(trace.Begin(""))

	connector := &Connector{
		interactions: make(map[string]*LazySessionInteractor),
		listener:     listener,
		done:         make(chan struct{}),
	}
	connector.cond = sync.NewCond(connector.mutex.RLocker())

	return connector
}

// SessionIfAlive returns SessionInteractor or error
func (c *Connector) SessionIfAlive(ctx context.Context, id string) (SessionInteractor, error) {
	c.mutex.RLock()
	v, ok := c.interactions[id]
	c.mutex.RUnlock()

	if !ok {
		return nil, fmt.Errorf("attach connector: no such connection in the map")
	}
	// we have an entry in the map, let's check its status
	var conn SessionInteractor
	var err error

	conn, err = v.Initialize()
	if err != nil {
		goto Error
	}

	log.Debugf("attach connector: Pinging for %s", id)
	if err = conn.Ping(); err != nil {
		goto Error
	}

	log.Debugf("attach connector: Unblocking for %s", id)
	if err = conn.Unblock(); err != nil {
		goto Error
	}
	log.Debugf("attach connector: Unblocked %s, returning", id)

	return conn, nil

Error:
	log.Debugf("attach connector: liveness check failed, removing %s from connection map", id)

	c.mutex.Lock()
	delete(c.interactions, id)
	c.mutex.Unlock()

	return nil, err
}

// Interaction returns the interactor corresponding to the specified ID. If the connection doesn't exist
// the method will wait for the specified timeout, returning when the connection is created
// or the timeout expires, whichever occurs first
func (c *Connector) Interaction(ctx context.Context, id string) (SessionInteractor, error) {
	defer trace.End(trace.Begin(id))

	// make sure that we have only one call in-flight for each ID at any given time
	si, err, shared := c.fg.Do(id, func() (interface{}, error) {
		return c.interaction(ctx, id)
	})
	if err != nil {
		c.fg.Forget(id)
		return nil, err
	}
	if shared {
		log.Debugf("Eliminated duplicated calls to Interaction for %s", id)
	}
	return si.(SessionInteractor), nil
}

func (c *Connector) interaction(ctx context.Context, id string) (SessionInteractor, error) {
	defer trace.End(trace.Begin(id))

	conn, err := c.SessionIfAlive(ctx, id)
	if conn != nil && err == nil {
		return conn, nil
	}

	if ctx.Err() == context.DeadlineExceeded {
		return nil, fmt.Errorf("attach connector: no such connection")
	}

	result := make(chan SessionInteractor, 1)
	go func() {
		ok := false
		var v *LazySessionInteractor

		c.mutex.RLock()
		defer c.mutex.RUnlock()

		for !ok && ctx.Err() == nil {
			v, ok = c.interactions[id]
			if ok {
				conn, err := v.Initialize()
				if conn != nil && err == nil {
					// no need to test this connection as we just created it, unblock if needed
					log.Debugf("attach connector: Unblocking for %s", id)
					err = conn.Unblock()
					if err == nil {
						log.Debugf("attach connector: Unblocked %s, returning", id)

						result <- conn
						return
					}
				}
				if err != nil {
					log.Error(err)
				}
				ok = false
			}

			// block until cond is updated
			log.Infof("attach connector:  Connection not found yet for %s", id)
			c.cond.Wait()
		}
		log.Debugf("attach connector:  Giving up on connection for %s", id)
	}()

	select {
	case client := <-result:
		log.Debugf("attach connector: Found connection for %s: %p", id, client)
		return client, nil
	case <-ctx.Done():
		err := fmt.Errorf("attach connector: Connection not found error for id:%s: %s", id, ctx.Err())
		log.Error(err)
		// wake up the result gofunc before returning
		c.mutex.RLock()
		c.cond.Broadcast()
		c.mutex.RUnlock()

		return nil, err
	}
}

// RemoveInteraction removes the session the inteactions map
func (c *Connector) RemoveInteraction(id string) error {
	defer trace.End(trace.Begin(id))

	var err error

	c.mutex.Lock()
	v, ok := c.interactions[id]
	if ok {
		log.Debugf("attach connector: Removing %s from the connection map", id)
		delete(c.interactions, id)
		c.fg.Forget(id)
	}
	c.mutex.Unlock()

	// the !ok case, but let's check the actual condition that impacts us
	if v == nil {
		return nil
	}

	conn := v.SessionInteractor()
	if conn != nil {
		err = conn.Close()
	}
	return err
}

// Start starts the connector
func (c *Connector) Start() {
	defer trace.End(trace.Begin(""))

	c.wg.Add(1)
	go c.serve()
}

// Stop stops the connector
func (c *Connector) Stop() {
	defer trace.End(trace.Begin(""))

	c.listener.Close()
	close(c.done)
	c.wg.Wait()
}

// Starts the connector listening on the specified source
// TODO: should have mechanism for stopping this, and probably handing off the interactions to another
// routine to insert into the map
func (c *Connector) serve() {
	defer c.wg.Done()
	for {
		if c.listener == nil {
			log.Debugf("attach connector: listener closed")
			break
		}

		// check to see whether we should stop accepting new connections and exit
		select {
		case <-c.done:
			log.Debugf("attach connector: done closed")
			return
		default:
		}

		conn, err := c.listener.Accept()
		if err != nil {
			log.Errorf("Error waiting for incoming connection: %s", errors.ErrorStack(err))
			continue
		}
		log.Debugf("attach connector: Received incoming connection")

		go c.processIncoming(conn)
	}
}

// takes the base connection, determines the ID of the source and stashes it in the map
func (c *Connector) processIncoming(conn net.Conn) {
	var err error
	defer func() {
		if err != nil && conn != nil {
			conn.Close()
		}
	}()

	log.Debugf("Initiating ssh handshake with new connection attempt")
	for {
		if conn == nil {
			log.Infof("attach connector: connection closed")
			return
		}

		// TODO needs timeout handling.  This could take 30s.

		// Timeout for client handshake should be reasonably small.
		// Server will try to drain a buffer and if the buffer doesn't contain
		// 2 or more bytes it will just wait, so client should timeout.
		// However, if timeout is too short, client will flood server with Syn requests.
		ctx, cancel := context.WithTimeout(context.TODO(), time.Second)
		defer cancel()

		deadline, ok := ctx.Deadline()
		if ok {
			conn.SetReadDeadline(deadline)
		}
		if err = serial.HandshakeClient(conn); err == nil {
			conn.SetReadDeadline(time.Time{})
			log.Debugf("HandshakeClient: connection handshake established")
			cancel()
			break
		}

		switch e := err.(type) {
		case *serial.HandshakeError:
			log.Debugf("HandshakeClient: %v", e)
			continue
		case *net.OpError:
			if e.Temporary() || e.Timeout() {
				// if it's a passing error or timeout then try again
				continue
			}
			// if it's not a temporary condition, then treat it as a transport error
			log.Errorf("HandshakeClient: transport op-error: %v", e)
			conn.Close()
			return
		default: // includes the io.EOF case
			// treat everything unknown as transport errror
			log.Errorf("HandshakeClient: transport error: %v (%T)", e, e)
			conn.Close()
			return
		}
	}

	callback := func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		return nil
	}

	config := &ssh.ClientConfig{
		User:            "daemon",
		HostKeyCallback: callback,
		ClientVersion:   VersionString,
		Timeout:         ClientTimeout,
	}

	// create the SSH connection
	clientConn, chans, reqs, err := ssh.NewClientConn(conn, "", config)
	if err != nil {
		log.Errorf("SSH connection could not be established: %s", errors.ErrorStack(err))
		return
	}

	// ask the IDs
	ids, err := ContainerIDs(clientConn)
	if err != nil {
		log.Errorf("SSH connection could not be established: %s", errors.ErrorStack(err))
		return
	}

	// Handle global requests
	go c.reqs(reqs, clientConn, ids)
	// Handle channel open messages
	go c.chans(chans)

	// create the connections
	c.ids(clientConn, ids)

	return
}

// ids iterates over the gived ids and
// - calls Ping for existing connections
// - calls NewSSHInteraction for new connections and fills the connection map
func (c *Connector) ids(conn ssh.Conn, ids []string) {
	for _, id := range ids {
		// needed for following closure - https://golang.org/doc/faq#closures_and_goroutines
		id := id

		c.mutex.RLock()
		v, ok := c.interactions[id]
		c.mutex.RUnlock()

		if ok {
			si, err := v.Initialize()
			if si != nil && err == nil {
				if err := si.Ping(); err == nil {
					log.Debugf("Connection %s found and alive", id)

					continue
				}
			}
			log.Warnf("Connection found but it wasn't alive. Creating a new one")
		}

		// this is a new connection so learn the version
		version, err := ContainerVersion(conn)
		if err != nil {
			log.Errorf("SSH version could not be learned (id=%s): %s", id, errors.ErrorStack(err))
			return
		}

		lazy := &LazySessionInteractor{
			fn: func() (SessionInteractor, error) {
				defer trace.End(trace.Begin(id))

				return NewSSHInteraction(conn, id, version)
			},
		}

		log.Infof("Established connection with container VM: %s", id)

		c.mutex.Lock()

		c.interactions[id] = lazy

		c.cond.Broadcast()
		c.mutex.Unlock()
	}
}

// reqs is the global request channel of the portlayer side of the connection
// we keep a list of sessions associated with this connection and drop them from the map when the global mux exits
func (c *Connector) reqs(reqs <-chan *ssh.Request, conn ssh.Conn, ids []string) {
	defer trace.End(trace.Begin(""))

	var pending func()

	// list of session ids mux'ed on this connection
	droplist := make(map[string]struct{})

	// fill the map with the initial ids
	for _, id := range ids {
		droplist[id] = struct{}{}
	}

	for req := range reqs {
		ok := true

		log.Infof("received global request type %v", req.Type)
		switch req.Type {
		case msgs.ContainersReq:
			pending = func() {
				ids := msgs.ContainersMsg{}
				if err := ids.Unmarshal(req.Payload); err != nil {
					log.Errorf("Unmarshal failed with %s", err)
					return
				}
				c.ids(conn, ids.IDs)

				// drop the drop list to clear no longer active sessions from the map
				droplist = make(map[string]struct{})

				// fill the droplist with the latest info
				for _, id := range ids.IDs {
					droplist[id] = struct{}{}
				}
			}
		default:
			ok = false
		}

		// make sure that errors get send back if we failed
		if req.WantReply {
			log.Infof("Sending global request reply %t", ok)
			if err := req.Reply(ok, nil); err != nil {
				log.Warnf("Failed to reply a request back")
			}
		}

		// run any pending work now that a reply has been sent
		if pending != nil {
			log.Debug("Invoking pending work for global mux")
			go pending()
			pending = nil
		}
	}

	// global mux closed so it is time to do cleanup
	for id := range droplist {
		log.Infof("Droping %s from connection map", id)
		c.RemoveInteraction(id)
	}
}

// this is the channel mux for the ssh channel . It is configured to reject everything (required)
func (c *Connector) chans(chans <-chan ssh.NewChannel) {
	defer trace.End(trace.Begin(""))

	for ch := range chans {
		ch.Reject(ssh.UnknownChannelType, fmt.Sprintf("unknown channel type: %v", ch.ChannelType()))
	}
}
