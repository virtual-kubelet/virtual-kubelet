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

package management

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/vmware/vic/pkg/trace"
)

func TestFindCertPaths(t *testing.T) {
	op := trace.NewOperation(context.Background(), "TestFindCertPaths")

	vchName := "vch-foo"

	// NOTE: not checking for dockerConfPath since $HOME is dependent on the user
	// running the test
	possiblePaths := map[string]bool{
		vchName: false,
		".":     false,
	}

	// Get paths when an input certPath is not specified
	paths := findCertPaths(op, vchName, "")
	assert.True(t, len(paths) >= 2)
	for i := range paths {
		possiblePaths[paths[i]] = true
	}
	assert.True(t, possiblePaths[vchName])
	assert.True(t, possiblePaths["."])

	// Get paths when an input certPath is specified
	paths = findCertPaths(op, vchName, "foopath")
	for i := range paths {
		possiblePaths[paths[i]] = true
	}
	assert.True(t, len(paths) == 1)
	assert.True(t, possiblePaths["foopath"])
}
