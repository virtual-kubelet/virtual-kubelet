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

package retry

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRetry(t *testing.T) {
	i := 0
	operation := func() error {
		i++
		t.Logf("Retry called: %s", time.Now().UTC())
		return fmt.Errorf("designed error")
	}
	retryOnError := func(err error) bool {
		if i < 4 {
			return true
		}
		return false
	}
	err := Do(operation, retryOnError)
	assert.True(t, i == 4, "should retried 4 times")
	assert.True(t, err != nil, fmt.Sprintf("retry do not depend on error status"))
}
