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

package vmomi

import (
	"testing"

	"reflect"

	"github.com/stretchr/testify/assert"

	"github.com/vmware/govmomi/vim25/types"
)

func TestDelta(t *testing.T) {
	new := map[string]string{
		"hello":  "goodbye",
		"cruel":  "world",
		"is":     "not",
		"enough": "already",
	}

	existing := []types.BaseOptionValue{
		&types.OptionValue{Key: "hello", Value: "goodbye"},
		&types.OptionValue{Key: "is", Value: "always"},
		&types.OptionValue{Key: "present", Value: "regardless"},
	}

	updatesSlice := OptionValueUpdatesFromMap(existing, new)

	expected := map[string]string{
		"enough": "already", // added
		"cruel":  "world",   // added
		"is":     "not",     // changed
	}

	// turn them back into maps for equality check
	updates := OptionValueMap(updatesSlice)

	if !assert.True(t, reflect.DeepEqual(expected, updates), "DeepEqual says they do not match") {
		t.Fatalf("Expected: %+q \nActual: %+q\n", expected, updates)
	}
}
