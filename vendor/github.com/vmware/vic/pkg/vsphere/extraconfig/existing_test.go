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

package extraconfig

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSkipDecode(t *testing.T) {
	type Type struct {
		Time     time.Time `vic:"0.1" scope:"read-write" key:"time"`
		TimeSink time.Time `vic:"0.1" scope:"read-write" key:"timesink" recurse:"skip-decode"`
	}

	Time := Type{
		Time:     time.Date(2009, 11, 10, 23, 00, 00, 0, time.UTC),
		TimeSink: time.Date(2009, 11, 10, 23, 00, 00, 0, time.UTC),
	}

	encoded := map[string]string{}
	Encode(MapSink(encoded), Time)

	expected := map[string]string{
		visibleRW("time"):     "2009-11-10 23:00:00 +0000 UTC",
		visibleRW("timesink"): "2009-11-10 23:00:00 +0000 UTC",
	}

	assert.Equal(t, encoded, expected, "Encoded and expected does not match")

	// update the time values
	Time.Time = time.Date(2010, 11, 10, 23, 00, 00, 0, time.UTC)
	Time.TimeSink = time.Date(2010, 11, 10, 23, 00, 00, 0, time.UTC)

	// Decode into the existing structure
	Decode(MapSource(encoded), &Time)

	encoded2 := map[string]string{}
	Encode(MapSink(encoded2), Time)

	// Encode again - change in TimeSink structure should have been preserved over decode but
	// Time should have been reset to encoded value
	expected2 := map[string]string{
		visibleRW("time"):     "2009-11-10 23:00:00 +0000 UTC",
		visibleRW("timesink"): "2010-11-10 23:00:00 +0000 UTC",
	}

	assert.Equal(t, encoded2, expected2, "Encoded and expected does not match")
}
