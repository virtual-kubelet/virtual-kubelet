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

package network

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/vic/lib/config/executor"
	"github.com/vmware/vic/lib/constants"
	"github.com/vmware/vic/pkg/ip"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/uid"
)

type Scope struct {
	sync.RWMutex

	id          uid.UID
	name        string
	scopeType   string
	subnet      *net.IPNet
	gateway     net.IP
	dns         []net.IP
	trustLevel  executor.TrustLevel
	containers  map[uid.UID]*Container
	endpoints   []*Endpoint
	spaces      []*AddressSpace
	builtin     bool
	network     object.NetworkReference
	annotations map[string]string
	internal    bool
}

func newScope(id uid.UID, scopeType string, network object.NetworkReference, scopeData *ScopeData) *Scope {
	return &Scope{
		id:          id,
		name:        scopeData.Name,
		scopeType:   scopeType,
		subnet:      scopeData.Subnet,
		gateway:     scopeData.Gateway,
		dns:         scopeData.DNS,
		trustLevel:  scopeData.TrustLevel,
		network:     network,
		containers:  make(map[uid.UID]*Container),
		annotations: make(map[string]string),
		internal:    scopeData.Internal,
	}
}

func (s *Scope) Annotations() map[string]string {
	s.RLock()
	defer s.RUnlock()

	return s.annotations
}

func (s *Scope) Name() string {
	s.RLock()
	defer s.RUnlock()

	return s.name
}

func (s *Scope) ID() uid.UID {
	s.RLock()
	defer s.RUnlock()

	return s.id
}

func (s *Scope) Type() string {
	s.RLock()
	defer s.RUnlock()

	return s.scopeType
}

func (s *Scope) Internal() bool {
	s.RLock()
	defer s.RUnlock()

	return s.internal
}

func (s *Scope) Network() object.NetworkReference {
	s.RLock()
	defer s.RUnlock()

	return s.network
}

func (s *Scope) isDynamic() bool {
	return s.scopeType != constants.BridgeScopeType && len(s.spaces) == 0
}

func (s *Scope) Pools() []*ip.Range {
	s.RLock()
	defer s.RUnlock()

	return s.pools()
}

func (s *Scope) TrustLevel() executor.TrustLevel {
	s.RLock()
	defer s.RUnlock()

	return s.trustLevel
}

func (s *Scope) pools() []*ip.Range {
	pools := make([]*ip.Range, len(s.spaces))
	for i := range s.spaces {
		sp := s.spaces[i]
		if sp.Network != nil {
			r := ip.ParseRange(sp.Network.String())
			if r == nil {
				continue
			}
			pools[i] = r
			continue
		}

		pools[i] = sp.Pool
	}

	return pools
}

func (s *Scope) reserveEndpointIP(e *Endpoint) error {
	if s.isDynamic() {
		return nil
	}

	// reserve an ip address
	var err error
	for _, p := range s.spaces {
		if !ip.IsUnspecifiedIP(e.ip) {
			if err = p.ReserveIP4(e.ip); err == nil {
				return nil
			}
		} else {
			var eip net.IP
			if eip, err = p.ReserveNextIP4(); err == nil {
				e.ip = eip
				return nil
			}
		}
	}

	return err
}

func (s *Scope) releaseEndpointIP(e *Endpoint) error {
	if s.isDynamic() {
		return nil
	}

	for _, p := range s.spaces {
		if err := p.ReleaseIP4(e.ip); err == nil {
			if !e.static {
				e.ip = net.IPv4(0, 0, 0, 0)
			}
			return nil
		}
	}

	return fmt.Errorf("could not release IP for endpoint")
}

func (s *Scope) AddContainer(con *Container, e *Endpoint) error {
	op := trace.NewOperation(context.Background(), "Add container to the scope")

	s.Lock()
	defer s.Unlock()

	if con == nil {
		return fmt.Errorf("container is nil")
	}

	_, ok := s.containers[con.id]
	if ok {
		return DuplicateResourceError{resID: con.id.String()}
	}

	op.Debugf("Adding container %s to the scope %s(%s)",
		con.id, s.name, s.id)

	if err := s.reserveEndpointIP(e); err != nil {
		return err
	}

	con.addEndpoint(e)
	s.endpoints = append(s.endpoints, e)
	s.containers[con.id] = con
	return nil
}

func (s *Scope) RemoveContainer(con *Container) error {
	s.Lock()
	defer s.Unlock()

	op := trace.NewOperation(context.Background(), "Removing container from the scope")

	c, ok := s.containers[con.id]
	if !ok || c != con {
		op.Debugf("Container %s not found in the scope %s(%s)", con.id, s.name, s.id)
		return ResourceNotFoundError{}
	}

	e := c.Endpoint(s)
	if e == nil {
		op.Debugf("No scope endpoint for container %s in the scope %s(%s)", con.id, s.name, s.id)
		return ResourceNotFoundError{}
	}

	if err := s.releaseEndpointIP(e); err != nil {
		return err
	}

	delete(s.containers, c.id)
	s.endpoints = removeEndpointHelper(e, s.endpoints)
	c.removeEndpoint(e)

	op.Debugf("Container %s removed from the scope %s(%s)", con.id, s.name, s.id)

	return nil
}

func (s *Scope) Containers() []*Container {
	s.RLock()
	defer s.RUnlock()

	containers := make([]*Container, len(s.containers))
	i := 0
	for _, c := range s.containers {
		containers[i] = c
		i++
	}

	return containers
}

func (s *Scope) Container(id uid.UID) *Container {
	s.RLock()
	defer s.RUnlock()

	if c, ok := s.containers[id]; ok {
		return c
	}

	return nil
}

func (s *Scope) ContainerByAddr(addr net.IP) *Endpoint {
	s.RLock()
	defer s.RUnlock()

	if addr == nil || addr.IsUnspecified() {
		return nil
	}

	for _, e := range s.endpoints {
		if addr.Equal(e.IP()) {
			return e
		}
	}

	return nil
}

func (s *Scope) Endpoints() []*Endpoint {
	s.RLock()
	defer s.RUnlock()

	eps := make([]*Endpoint, len(s.endpoints))
	copy(eps, s.endpoints)
	return eps
}

func (s *Scope) Subnet() *net.IPNet {
	s.RLock()
	defer s.RUnlock()

	return s.subnet
}

func (s *Scope) Gateway() net.IP {
	s.RLock()
	defer s.RUnlock()

	return s.gateway
}

func (s *Scope) DNS() []net.IP {
	s.RLock()
	defer s.RUnlock()

	return s.dns
}

type scopeJSON struct {
	ID          uid.UID
	Name        string
	Type        string
	Subnet      *net.IPNet
	Gateway     net.IP
	DNS         []net.IP
	Builtin     bool
	Pools       []*ip.Range
	Annotations map[string]string
	Internal    bool
}

func (s *Scope) MarshalJSON() ([]byte, error) {
	s.RLock()
	defer s.RUnlock()

	return json.Marshal(&scopeJSON{
		ID:          s.id,
		Name:        s.name,
		Type:        s.scopeType,
		Subnet:      s.subnet,
		Gateway:     s.gateway,
		DNS:         s.dns,
		Builtin:     s.builtin,
		Pools:       s.pools(),
		Annotations: s.annotations,
		Internal:    s.internal,
	})
}

func (s *Scope) UnmarshalJSON(data []byte) error {
	s.Lock()
	defer s.Unlock()

	var sj scopeJSON
	if err := json.Unmarshal(data, &sj); err != nil {
		return err
	}

	ns := Scope{
		containers:  make(map[uid.UID]*Container),
		annotations: make(map[string]string),
	}
	ns.id = sj.ID
	ns.name = sj.Name
	ns.scopeType = sj.Type
	ns.subnet = sj.Subnet
	ns.gateway = sj.Gateway
	ns.dns = sj.DNS
	ns.builtin = sj.Builtin
	ns.spaces = make([]*AddressSpace, len(sj.Pools))
	for i := range sj.Pools {
		sp := NewAddressSpaceFromRange(sj.Pools[i].FirstIP, sj.Pools[i].LastIP)
		if sp == nil {
			return fmt.Errorf("invalid pool %s in scope %s", sj.Pools[i].String(), sj.Name)
		}

		ns.spaces[i] = sp
	}

	for k, v := range sj.Annotations {
		ns.annotations[k] = v
	}

	ns.internal = sj.Internal

	s.swap(&ns)

	return nil
}

func (s *Scope) swap(other *Scope) {
	s.id, other.id = other.id, s.id
	s.name, other.name = other.name, s.name
	s.scopeType, other.scopeType = other.scopeType, s.scopeType
	s.subnet, other.subnet = other.subnet, s.subnet
	s.gateway, other.gateway = other.gateway, s.gateway
	s.dns, other.dns = other.dns, s.dns
	s.builtin, other.builtin = other.builtin, s.builtin
	s.spaces, other.spaces = other.spaces, s.spaces
	s.endpoints, other.endpoints = other.endpoints, s.endpoints
	s.containers, other.containers = other.containers, s.containers
	s.network, other.network = other.network, s.network
	s.annotations, other.annotations = other.annotations, s.annotations
	s.internal, other.internal = other.internal, s.internal
}
