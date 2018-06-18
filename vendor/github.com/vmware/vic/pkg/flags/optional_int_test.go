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

package flags

import (
	"flag"
	"testing"
)

func TestOptionalInt(t *testing.T) {
	fs := flag.NewFlagSet("", flag.ContinueOnError)
	var val *int

	fs.Var(NewOptionalInt(&val), "oint", "optional int")

	b := fs.Lookup("oint")

	if b.DefValue != "<nil>" {
		t.Fail()
	}

	if b.Value.String() != "<nil>" {
		t.Fail()
	}

	if b.Value.(flag.Getter).Get() != nil {
		t.Fail()
	}

	b.Value.Set("1")

	if b.Value.String() != "1" {
		t.Fail()
	}

	if b.Value.(flag.Getter).Get() != 1 {
		t.Fail()
	}
}
