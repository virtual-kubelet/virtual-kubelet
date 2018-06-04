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

package network

import (
	"context"
	"fmt"
	"sync"

	log "github.com/Sirupsen/logrus"

	"github.com/vmware/vic/lib/portlayer/exec"
	"github.com/vmware/vic/pkg/uid"
)

type Container struct {
	sync.Mutex

	id        uid.UID
	name      string
	endpoints []*Endpoint
}

func (c *Container) Endpoints() []*Endpoint {
	c.Lock()
	defer c.Unlock()

	ret := make([]*Endpoint, len(c.endpoints))
	copy(ret, c.endpoints)
	return ret
}

func (c *Container) ID() uid.UID {
	return c.id
}

func (c *Container) Name() string {
	return c.name
}

func (c *Container) endpoint(s *Scope) *Endpoint {
	for _, e := range c.endpoints {
		if e.Scope() == s {
			return e
		}
	}

	return nil
}

func (c *Container) Endpoint(s *Scope) *Endpoint {
	c.Lock()
	defer c.Unlock()

	return c.endpoint(s)
}

func (c *Container) Scopes() []*Scope {
	c.Lock()
	defer c.Unlock()

	scopes := make([]*Scope, len(c.endpoints))
	i := 0
	for _, e := range c.endpoints {
		scopes[i] = e.Scope()
		i++
	}

	return scopes
}

func (c *Container) addEndpoint(e *Endpoint) {
	c.Lock()
	defer c.Unlock()

	c.endpoints = append(c.endpoints, e)
}

func (c *Container) removeEndpoint(e *Endpoint) {
	c.Lock()
	defer c.Unlock()

	c.endpoints = removeEndpointHelper(e, c.endpoints)
}

func (c *Container) Refresh(ctx context.Context) error {
	c.Lock()
	defer c.Unlock()

	// this will "refresh" the container executor config that contains
	// the current ip addresses
	h := exec.GetContainer(ctx, c.ID())
	if h == nil {
		return fmt.Errorf("could not find container %s", c.ID())
	}
	defer h.Close()

	for _, e := range c.endpoints {
		if err := e.refresh(h); err != nil {
			log.Warnf("could not refresh endpoint for container %s: %s", h.ExecConfig.ID, err)
		}
	}

	return nil
}
