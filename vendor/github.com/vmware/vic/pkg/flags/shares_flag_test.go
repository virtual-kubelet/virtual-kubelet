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
	"flag"
	"strings"
	"testing"

	"github.com/vmware/govmomi/vim25/types"
)

func TestShareFlag(t *testing.T) {
	fs := flag.NewFlagSet("", flag.ContinueOnError)
	var val *types.SharesInfo

	fs.Var(NewSharesFlag(&val), "shares", "memory shares")

	u := fs.Lookup("shares")

	if u.DefValue != "<nil>" {
		t.Errorf("DefValue: %s", u.DefValue)
	}

	if u.Value.String() != "<nil>" {
		t.Errorf("Value: %s", u.Value)
	}

	ref := "2000"
	u.Value.Set(ref)

	if u.Value.String() != strings.ToLower(ref) {
		t.Errorf("Value after set: %q", u.Value)
	}

	if val == nil {
		t.Errorf("val is not set")
	}
	if val.Level != types.SharesLevelCustom {
		t.Errorf("shares level is not set correctly: %s", val.Level)
	}
	if val.Shares != 2000 {
		t.Errorf("shares Share is not set correctly: %d", val.Shares)
	}

	ref = "HIGH"
	u.Value.Set(ref)

	if u.Value.String() != strings.ToLower(ref) {
		t.Errorf("Value after set: %q", u.Value)
	}

	if val == nil {
		t.Errorf("val is not set")
	}
	if val.Level != types.SharesLevelHigh {
		t.Errorf("shares level is not set correctly: %s", val.Level)
	}
	if val.Shares != 0 {
		t.Errorf("shares Share is not set correctly: %d", val.Shares)
	}

}
