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

package exec

import (
	"context"

	log "github.com/Sirupsen/logrus"

	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

type VCHStats struct {
	CPULimit    int64 // resource pool CPU limit
	CPUUsage    int64 // resource pool CPU usage in MhZ
	MemoryLimit int64 // resource pool Memory limit
	MemoryUsage int64 // resource pool Memory Usage
}

func GetVCHstats(ctx context.Context, moref ...types.ManagedObjectReference) VCHStats {
	var p mo.ResourcePool
	var vch VCHStats

	if Config.ResourcePool == nil {
		log.Errorf("Unable to retrieve VCHstats: Config.ResourcePool is nil")
		return vch
	}

	r := Config.ResourcePool.Reference()
	if len(moref) > 0 {
		r = moref[0]
	}

	ps := []string{"config.cpuAllocation", "config.memoryAllocation", "runtime.cpu", "runtime.memory", "parent"}

	if err := Config.ResourcePool.Properties(ctx, r, ps, &p); err != nil {
		log.Errorf("VCH stats error: %s", err)
		return vch
	}

	vch.CPUUsage = p.Runtime.Cpu.OverallUsage
	vch.MemoryUsage = p.Runtime.Memory.OverallUsage

	if p.Config.CpuAllocation.Limit != nil {
		vch.CPULimit = *p.Config.CpuAllocation.Limit
	}

	if p.Config.MemoryAllocation.Limit != nil {
		vch.MemoryLimit = *p.Config.MemoryAllocation.Limit
	}

	stats := []int64{vch.CPULimit,
		vch.MemoryLimit,
		vch.CPUUsage,
		vch.MemoryUsage}

	log.Debugf("The VCH stats are: %+v", stats)

	// If any of the stats is -1, we need to get the vch stats from the parent resource pool
	for _, v := range stats {
		if v == -1 {
			return GetVCHstats(ctx, *p.Parent)
		}
	}

	return vch
}
