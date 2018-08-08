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
	"github.com/vmware/govmomi/object"
	"github.com/vmware/vic/pkg/trace"
)

// HostPlacementPolicy defines the interface for using metrics to decide the appropriate host
// for a VM based on host metrics and VM provisioned resources.
type HostPlacementPolicy interface {
	// TODO(jzt): refactor to consider vm resource needs in the future
	// CheckHost checks whether or not the host a VM was created on is adequate for power-on.
	CheckHost(trace.Operation, *object.VirtualMachine) bool

	// RecommendHost recommends an ordered list of hosts on which a VM
	// may be placed prior to power-on. The list is ordered from more
	// ideal to less ideal based on weighted host metrics.
	RecommendHost(trace.Operation, []*object.HostSystem) ([]*object.HostSystem, error)
}
