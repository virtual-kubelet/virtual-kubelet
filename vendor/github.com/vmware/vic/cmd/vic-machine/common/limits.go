// Copyright 2017 VMware, Inc. All Rights Reserved.
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

package common

import (
	"gopkg.in/urfave/cli.v1"

	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/vic/pkg/flags"
)

type ResourceLimits struct {
	VCHCPULimitsMHz       *int              `cmd:"cpu"`
	VCHCPUReservationsMHz *int              `cmd:"cpu-reservation"`
	VCHCPUShares          *types.SharesInfo `cmd:"cpu-shares"`

	VCHMemoryLimitsMB       *int              `cmd:"memory"`
	VCHMemoryReservationsMB *int              `cmd:"memory-reservation"`
	VCHMemoryShares         *types.SharesInfo `cmd:"memory-shares"`

	IsSet bool
}

func (r *ResourceLimits) VCHMemoryLimitFlags() []cli.Flag {
	return []cli.Flag{
		cli.GenericFlag{
			Name:  "memory, mem",
			Value: flags.NewOptionalInt(&r.VCHMemoryLimitsMB),
			Usage: "VCH resource pool memory limit in MB (unlimited=0)",
		},
		cli.GenericFlag{
			Name:   "memory-reservation, memr",
			Value:  flags.NewOptionalInt(&r.VCHMemoryReservationsMB),
			Usage:  "VCH resource pool memory reservation in MB",
			Hidden: true,
		},
		cli.GenericFlag{
			Name:   "memory-shares, mems",
			Value:  flags.NewSharesFlag(&r.VCHMemoryShares),
			Usage:  "VCH resource pool memory shares in level or share number, e.g. high, normal, low, or 163840",
			Hidden: true,
		},
	}
}

func (r *ResourceLimits) VCHCPULimitFlags() []cli.Flag {
	return []cli.Flag{
		cli.GenericFlag{
			Name:  "cpu",
			Value: flags.NewOptionalInt(&r.VCHCPULimitsMHz),
			Usage: "VCH resource pool vCPUs limit in MHz (unlimited=0)",
		},
		cli.GenericFlag{
			Name:   "cpu-reservation, cpur",
			Value:  flags.NewOptionalInt(&r.VCHCPUReservationsMHz),
			Usage:  "VCH resource pool reservation in MHz",
			Hidden: true,
		},
		cli.GenericFlag{
			Name:   "cpu-shares, cpus",
			Value:  flags.NewSharesFlag(&r.VCHCPUShares),
			Usage:  "VCH resource pool vCPUs shares, in level or share number, e.g. high, normal, low, or 4000",
			Hidden: true,
		},
	}
}
