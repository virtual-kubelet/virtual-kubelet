// Copyright 2016-2017 VMware, Inc. All Rights Reserved.
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

package extraconfig

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSecretFields(t *testing.T) {
	type tell struct {
		Who string `vic:"0.1" scope:"secret" key:"who"`
	}

	type stuff struct {
		Username string `vic:"0.1" scope:"read-only" key:"username"`
		Password string `vic:"0.1" scope:"secret" key:"password"`
		Tell     tell
	}

	config := stuff{
		Username: "root",
		Password: "super-s@fe-passw0rd",
		Tell:     tell{"noone"},
	}

	out, err := NewSecretKey()
	if err != nil {
		t.Fatal(err)
	}

	encoded := map[string]string{}
	Encode(out.Sink(MapSink(encoded)), config)

	password := encoded["guestinfo.vice./password"+suffixSeparator+secretSuffix]
	assert.NotEmpty(t, password, "encrypted password")
	assert.NotEqual(t, password, config.Password, "encrypted password")

	for _, expectEq := range []bool{true, false} {
		var in SecretKey

		var decoded stuff
		Decode(in.Source(MapSource(encoded)), &decoded)

		if expectEq {
			assert.Equal(t, config, decoded, "Encoded and decoded does not match")
		} else {
			assert.NotEqual(t, config, decoded, "Encoded and decoded should not not match")
		}

		// second time should fail to decrypt w/o GuestInfoSecretKey
		delete(encoded, GuestInfoSecretKey)
	}
}
