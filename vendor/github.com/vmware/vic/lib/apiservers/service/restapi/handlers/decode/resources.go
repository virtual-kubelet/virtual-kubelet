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

package decode

import (
	"fmt"
	"math"

	"github.com/docker/go-units"

	"github.com/vmware/govmomi/vim25/types"

	"github.com/vmware/vic/lib/apiservers/service/models"
)

func FromValueBytesMetric(m *models.ValueBytesMetric) string {
	v := float64(m.Value.Value)

	var bytes float64
	switch m.Value.Units {
	case models.ValueBytesMetricUnitsB:
		bytes = v
	case models.ValueBytesMetricUnitsKB:
		bytes = v * float64(units.KB)
	case models.ValueBytesMetricUnitsMB:
		bytes = v * float64(units.MB)
	case models.ValueBytesMetricUnitsGB:
		bytes = v * float64(units.GB)
	case models.ValueBytesMetricUnitsTB:
		bytes = v * float64(units.TB)
	case models.ValueBytesMetricUnitsPB:
		bytes = v * float64(units.PB)
	}

	return fmt.Sprintf("%d B", int64(bytes))
}

func MBFromValueBytes(m *models.ValueBytes) *int {
	if m == nil {
		return nil
	}

	v := float64(m.Value.Value)

	var mbs float64
	switch m.Value.Units {
	case models.ValueBytesUnitsB:
		mbs = v / float64(units.MiB)
	case models.ValueBytesUnitsKiB:
		mbs = v / (float64(units.MiB) / float64(units.KiB))
	case models.ValueBytesUnitsMiB:
		mbs = v
	case models.ValueBytesUnitsGiB:
		mbs = v * (float64(units.GiB) / float64(units.MiB))
	case models.ValueBytesUnitsTiB:
		mbs = v * (float64(units.TiB) / float64(units.MiB))
	case models.ValueBytesUnitsPiB:
		mbs = v * (float64(units.PiB) / float64(units.MiB))
	}

	i := int(math.Ceil(mbs))
	return &i
}

func MHzFromValueHertz(m *models.ValueHertz) *int {
	if m == nil {
		return nil
	}

	v := float64(m.Value.Value)

	var mhzs float64
	switch m.Value.Units {
	case models.ValueHertzUnitsHz:
		mhzs = v / float64(units.MB)
	case models.ValueHertzUnitsKHz:
		mhzs = v / (float64(units.MB) / float64(units.KB))
	case models.ValueHertzUnitsMHz:
		mhzs = v
	case models.ValueHertzUnitsGHz:
		mhzs = v * (float64(units.GB) / float64(units.MB))
	}

	i := int(math.Ceil(mhzs))
	return &i
}

func FromShares(m *models.Shares) *types.SharesInfo {
	if m == nil {
		return nil
	}

	var level types.SharesLevel
	switch types.SharesLevel(m.Level) {
	case types.SharesLevelLow:
		level = types.SharesLevelLow
	case types.SharesLevelNormal:
		level = types.SharesLevelNormal
	case types.SharesLevelHigh:
		level = types.SharesLevelHigh
	default:
		level = types.SharesLevelCustom
	}

	return &types.SharesInfo{
		Level:  level,
		Shares: int32(m.Number),
	}
}

func FromValueBits(m *models.ValueBits) int {
	return int(m.Value.Value)
}
