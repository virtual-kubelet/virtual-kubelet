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

package validate

import (
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/vmware/vic/pkg/errors"
	"github.com/vmware/vic/pkg/trace"
)

func VMGroupExists(op trace.Operation, cluster *object.ComputeResource, group string) (bool, error) {
	op.Debugf("Checking for existence of DRS VM Group %q on %s", group, cluster)

	var clusterConfig mo.ClusterComputeResource
	err := cluster.Properties(op, cluster.Reference(), []string{"configurationEx"}, &clusterConfig)
	if err != nil {
		return false, errors.Errorf("Unable to obtain cluster config: %s", err)
	}

	clusterConfigEx := clusterConfig.ConfigurationEx.(*types.ClusterConfigInfoEx)
	for _, g := range clusterConfigEx.Group {
		info := g.GetClusterGroupInfo()
		if info.Name == group {
			op.Debugf("DRS VM Group named %q exists", group)
			return true, nil
		}
	}

	op.Debugf("DRS VM Group named %q does not exist", group)
	return false, nil
}
