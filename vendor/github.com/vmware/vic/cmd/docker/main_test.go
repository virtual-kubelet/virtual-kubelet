// Copyright 2016-2018 VMware, Inc. All Rights Reserved.
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

package main

import (
	"flag"
	"testing"

	"github.com/stretchr/testify/assert"
)

var systemTest *bool

func init() {
	systemTest = flag.Bool("systemTest", false, "Run system test")
}

func TestSystem(t *testing.T) {
	if *systemTest {
		main()
	}
}

func TestHandleFlags(t *testing.T) {
	assert.True(t, handleFlags(), "Flags were not parsed correctly")
}
