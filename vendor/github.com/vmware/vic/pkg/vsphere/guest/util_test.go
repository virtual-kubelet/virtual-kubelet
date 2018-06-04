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

package guest

import (
	"os/user"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/vmware/vmw-guestinfo/vmcheck"
)

func TestUUID(t *testing.T) {
	if isVM, err := vmcheck.IsVirtualWorld(); !isVM || err != nil {
		t.Skip("can get uuid if not running on a vm")
	}
	// need to be root and on esx to run this test
	u, err := user.Current()
	if !assert.NoError(t, err) {
		return
	}

	if u.Uid != "0" {
		t.SkipNow()
		return
	}

	s, err := UUID()
	if !assert.NoError(t, err) {
		return
	}

	if !assert.NotNil(t, s) {
		return
	}
}
