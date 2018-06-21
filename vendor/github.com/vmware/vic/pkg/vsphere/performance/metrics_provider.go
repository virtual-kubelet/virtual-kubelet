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

package performance

import (
	"github.com/vmware/govmomi/object"
	"github.com/vmware/vic/pkg/trace"
)

// MetricsProvider defines the interface for providing metrics.
type MetricsProvider interface {
	// GetMetricsForComputeResource returns metrics for a particular compute resource. The metrics are
	// returned in a map of HostMetricsInfo keyed on host ManagedObjectReferences.
	GetMetricsForComputeResource(trace.Operation, *object.ComputeResource) (map[string]*HostMetricsInfo, error)

	// GetMetricsForHosts returns metrics pertaining to supplied ESX hosts.
	GetMetricsForHosts(trace.Operation, []*object.HostSystem) (map[string]*HostMetricsInfo, error)
}
