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

package backends

import (
	"net"

	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/libnetwork"
	"github.com/docker/libnetwork/types"
)

type sandbox struct {
	id          string
	containerID string
}

func newSandbox(containerID string) *sandbox {
	return &sandbox{
		id:          stringid.GenerateRandomID(),
		containerID: containerID,
	}
}

// ID returns the ID of the sandbox
func (s *sandbox) ID() string {
	return s.id
}

// Key returns the sandbox's key
func (s *sandbox) Key() string {
	return ""
}

// ContainerID returns the container id associated to this sandbox
func (s *sandbox) ContainerID() string {
	return s.containerID
}

// Labels returns the sandbox's labels
func (s *sandbox) Labels() map[string]interface{} {
	return nil
}

// Statistics retrieves the interfaces' statistics for the sandbox
func (s *sandbox) Statistics() (map[string]*types.InterfaceStatistics, error) {
	return nil, notImplementedError
}

// Refresh leaves all the endpoints, resets and re-apply the options,
// re-joins all the endpoints without destroying the osl sandbox
func (s *sandbox) Refresh(options ...libnetwork.SandboxOption) error {
	return notImplementedError
}

// SetKey updates the Sandbox Key
func (s *sandbox) SetKey(key string) error {
	return notImplementedError
}

// Rename changes the name of all attached Endpoints
func (s *sandbox) Rename(name string) error {
	return notImplementedError

}

// Delete destroys this container after detaching it from all connected endpoints.
func (s *sandbox) Delete() error {
	return notImplementedError

}

// ResolveName resolves a service name to an IPv4 or IPv6 address by searching
// the networks the sandbox is connected to. For IPv6 queries, second  return
// value will be true if the name exists in docker domain but doesn't have an
// IPv6 address. Such queries shouldn't be forwarded  to external nameservers.
func (s *sandbox) ResolveName(name string, iplen int) ([]net.IP, bool) {
	return nil, false

}

// ResolveIP returns the service name for the passed in IP. IP is in reverse dotted
// notation; the format used for DNS PTR records
func (s *sandbox) ResolveIP(name string) string {
	return ""

}

// Endpoints returns all the endpoints connected to the sandbox
func (s *sandbox) Endpoints() []libnetwork.Endpoint {
	return nil
}

// ResolveService returns all the backend details about the containers or hosts
// backing a service. Its purpose is to satisfy an SRV query
func (s *sandbox) ResolveService(name string) ([]*net.SRV, []net.IP) {
	return nil, nil
}

// EnableService  makes a managed container's service available by adding the
// endpoint to the service load balancer and service discovery
func (s *sandbox) EnableService() error {
	return notImplementedError
}

// DisableService removes a managed contianer's endpoints from the load balancer
// and service discovery
func (s *sandbox) DisableService() error {
	return notImplementedError
}
