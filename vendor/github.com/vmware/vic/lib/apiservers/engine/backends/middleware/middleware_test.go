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

package middleware

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateHostname(t *testing.T) {
	r := &http.Request{}
	hostname, err := validateHostname(r)
	assert.Error(t, err)
	assert.EqualValues(t, "", hostname)

	r.Host = ""
	hostname, err = validateHostname(r)
	assert.Error(t, err)
	assert.EqualValues(t, "", hostname)

	r.Host = "localname"
	hostname, err = validateHostname(r)
	assert.NoError(t, err)
	assert.EqualValues(t, "localname", hostname)

	r.Host = "localname:4567"
	hostname, err = validateHostname(r)
	assert.NoError(t, err)
	assert.EqualValues(t, "localname", hostname)

	r.Host = "[2605:a601:1119:6800:c69b:b2ec:eefa:ef4b]"
	hostname, err = validateHostname(r)
	assert.NoError(t, err)
	assert.EqualValues(t, "[2605:a601:1119:6800:c69b:b2ec:eefa:ef4b]", hostname)

	r.Host = "[2605:a601:1119:6800:c69b:b2ec:eefa:ef4b]:8080"
	hostname, err = validateHostname(r)
	assert.NoError(t, err)
	assert.EqualValues(t, "[2605:a601:1119:6800:c69b:b2ec:eefa:ef4b]", hostname)

	r.Host = "127.0.0.1:8080"
	hostname, err = validateHostname(r)
	assert.NoError(t, err)
	assert.EqualValues(t, "127.0.0.1", hostname)

	r.Host = "127.0.0.1"
	hostname, err = validateHostname(r)
	assert.NoError(t, err)
	assert.EqualValues(t, "127.0.0.1", hostname)

	r.Host = "foo.com:80"
	hostname, err = validateHostname(r)
	assert.NoError(t, err)
	assert.EqualValues(t, "foo.com", hostname)

	r.Host = "foo.com"
	hostname, err = validateHostname(r)
	assert.NoError(t, err)
	assert.EqualValues(t, "foo.com", hostname)

}
