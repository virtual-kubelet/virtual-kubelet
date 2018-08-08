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

package encode

import (
	"github.com/vmware/govmomi/vim25/types"

	"github.com/vmware/vic/lib/apiservers/service/models"
)

func AsBytes(value *int, units string) *models.ValueBytes {
	if value == nil || *value == 0 {
		return nil
	}

	return &models.ValueBytes{
		Value: models.Value{
			Value: int64(*value),
			Units: units,
		},
	}
}

func AsMiB(value *int) *models.ValueBytes {
	return AsBytes(value, models.ValueBytesUnitsMiB)
}

func AsBytesMetric(value *int, units string) *models.ValueBytesMetric {
	if value == nil || *value == 0 {
		return nil
	}

	return &models.ValueBytesMetric{
		Value: models.Value{
			Value: int64(*value),
			Units: units,
		},
	}
}

func AsKB(value *int) *models.ValueBytesMetric {
	return AsBytesMetric(value, models.ValueBytesMetricUnitsKB)
}

func AsMHz(value *int) *models.ValueHertz {
	if value == nil || *value == 0 {
		return nil
	}

	return &models.ValueHertz{
		Value: models.Value{
			Value: int64(*value),
			Units: models.ValueHertzUnitsMHz,
		},
	}
}

func AsShares(shares *types.SharesInfo) *models.Shares {
	if shares == nil {
		return nil
	}

	return &models.Shares{
		Level:  string(shares.Level),
		Number: int64(shares.Shares),
	}
}
