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

	log "github.com/Sirupsen/logrus"

	"github.com/vmware/vic/pkg/errors"
	"github.com/vmware/vic/pkg/trace"
)

// Server waits for TCP client connections on serialOverLANPort, then
// once connected, attempts to negotiate an SSH connection to the attached
// client.  The client is the ssh server.
type Server struct {
	port int
	ip   string

	m sync.RWMutex
	l *net.TCPListener
	c *Connector
}

// NewServer returns a Server instance
func NewServer(ip string, port int) *Server {
	defer trace.End(trace.Begin(""))

	return &Server{
		ip:   ip,
		port: port,
	}
}

// Start starts the connector with given listener
func (n *Server) Start() error {
	defer trace.End(trace.Begin(""))

	n.m.Lock()
	defer n.m.Unlock()

	addr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:%d", n.ip, n.port))
	if err != nil {
		return fmt.Errorf("Attach server error %s:%d: %s", n.ip, n.port, errors.ErrorStack(err))
	}

	n.l, err = net.ListenTCP("tcp", addr)
	if err != nil {
		return fmt.Errorf("Attach server error %s: %s", addr, errors.ErrorStack(err))
	}

	log.Infof("Attach server listening on %s:%d", n.ip, n.port)

	// starts serving requests immediately
	n.c = NewConnector(n.l)
	n.c.Start()

	return nil
}

// Stop stops the connector
func (n *Server) Stop() error {
	defer trace.End(trace.Begin(""))

	n.m.Lock()
	defer n.m.Unlock()

	err := n.l.Close()
	n.c.Stop()

	return err
}

// Addr returns the address of the underlying listener
func (n *Server) Addr() string {
	defer trace.End(trace.Begin(""))

	n.m.RLock()
	defer n.m.RUnlock()

	return n.l.Addr().String()
}

// Interaction returns the session interface for the given container.  If the container
// cannot be found, this call will wait for the given timeout.
// id is ID of the container.
func (n *Server) Interaction(ctx context.Context, id string) (SessionInteractor, error) {
	defer trace.End(trace.Begin(id))

	n.m.RLock()
	defer n.m.RUnlock()

	return n.c.Interaction(ctx, id)
}

// RemoveInteraction removes the session interface from underlying connector
func (n *Server) RemoveInteraction(id string) error {
	defer trace.End(trace.Begin(id))

	n.m.Lock()
	defer n.m.Unlock()

	return n.c.RemoveInteraction(id)
}
