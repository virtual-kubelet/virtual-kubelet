// Copyright 2018 VMware, Inc. All Rights Reserved.
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

package placement

import (
	"math/rand"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/vic/pkg/trace"
)

// RandomHostPolicy chooses a random host on which to power-on a VM.
type RandomHostPolicy struct {
	cluster *object.ComputeResource
}

// NewRandomHostPolicy returns a RandomHostPolicy instance.
func NewRandomHostPolicy(op trace.Operation, cls *object.ComputeResource) (*RandomHostPolicy, error) {
	return &RandomHostPolicy{cluster: cls}, nil
}

// CheckHost always returns false in a RandomHostPolicy.
func (p *RandomHostPolicy) CheckHost(op trace.Operation, vm *object.VirtualMachine) bool {
	return false
}

// RecommendHost recommends a random host on which to place a newly created VM. As this
// HostPlacementPolicy implementation does not rely on host metrics in its recommendation
// logic, hosts that are disconnected or in maintenance mode are not filtered from the
// returned list. Subsequent attempts to relocate to one of these hosts should result in
// the host being removed from the list and the resulting subset being used in a new call
// to RecommendHost.
func (p *RandomHostPolicy) RecommendHost(op trace.Operation, hosts []*object.HostSystem) ([]*object.HostSystem, error) {
	var err error
	if hosts == nil {
		hosts, err = p.cluster.Hosts(op)
		if err != nil {
			return nil, err
		}
	}

	// shuffle hosts
	for i := range hosts {
		j := rand.Intn(i + 1)
		hosts[i], hosts[j] = hosts[j], hosts[i]
	}

	return hosts, nil
}
