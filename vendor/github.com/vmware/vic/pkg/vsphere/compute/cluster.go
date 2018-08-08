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

package compute

import (
	"github.com/vmware/govmomi/object"
	"github.com/vmware/vic/pkg/trace"
)

type Cluster struct {
	*object.ClusterComputeResource
}

func NewCluster(compute object.ComputeResource) *Cluster {
	// ensure we have a cluster
	if compute.Reference().Type != "ClusterComputeResource" {
		return nil
	}

	return &Cluster{
		&object.ClusterComputeResource{
			ComputeResource: compute,
		},
	}
}

// DRSEnabled returns a bool indicating if DRS is enabled
func (c *Cluster) DRSEnabled(op trace.Operation) (bool, error) {
	config, err := c.Configuration(op)
	if err != nil {
		return false, err
	}
	return *config.DrsConfig.Enabled, nil
}
