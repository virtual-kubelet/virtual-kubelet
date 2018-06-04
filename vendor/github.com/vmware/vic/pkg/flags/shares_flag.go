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

package flags

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/vmware/govmomi/vim25/types"
)

type ShareFlag struct {
	shares **types.SharesInfo
}

func (s *ShareFlag) Set(val string) error {
	if *s.shares == nil {
		*s.shares = &types.SharesInfo{}
	}
	switch val = strings.ToLower(val); val {
	case string(types.SharesLevelNormal), string(types.SharesLevelLow), string(types.SharesLevelHigh):
		(*s.shares).Level = types.SharesLevel(val)
		(*s.shares).Shares = 0
	default:
		n, err := strconv.Atoi(val)
		if err != nil {
			return err
		}

		(*s.shares).Level = types.SharesLevelCustom
		(*s.shares).Shares = int32(n)
	}

	return nil
}

func (s *ShareFlag) String() string {
	if s.shares == nil || *s.shares == nil {
		return "<nil>"
	}
	switch (*s.shares).Level {
	case types.SharesLevelCustom:
		return fmt.Sprintf("%v", (*s.shares).Shares)
	default:
		return string((*s.shares).Level)
	}
}

func NewSharesFlag(shares **types.SharesInfo) *ShareFlag {
	return &ShareFlag{shares}
}
