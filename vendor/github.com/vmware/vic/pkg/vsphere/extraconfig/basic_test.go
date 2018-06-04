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
	"encoding/base64"
	"net"
	"net/url"
	"os/exec"
	"testing"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

// [BEGIN] SLIMMED DOWNED and MODIFIED VERSION of github.com/vmware/vic/lib/metadata
type Common struct {
	ExecutionEnvironment string `vic:"0.1" recurse:"depth=0"`

	ID string `vic:"0.1" scope:"read-only" key:"id"`

	Name string `vic:"0.1" scope:"read-only" key:"name"`

	Notes string `vic:"0.1" scope:"read-only" key:"notes"`
}

type ContainerVM struct {
	Common `vic:"0.1" scope:"read-only" key:"common"`

	Version string `vic:"0.1" scope:"hidden" key:"version"`

	Aliases map[string]string `vic:"0.1" recurse:"depth=0"`

	Interaction url.URL `vic:"0.1" recurse:"depth=0"`

	AgentKey []byte `vic:"0.1" recurse:"depth=0"`
}

type ExecutorConfig struct {
	Common `vic:"0.1" scope:"read-only" key:"common"`

	Sessions map[string]SessionConfig `vic:"0.1" scope:"hidden" key:"sessions"`

	Key string `json:"string"`
}

type ExecutorConfigPointers struct {
	Common `vic:"0.1" scope:"read-only" key:"common"`

	Sessions map[string]*SessionConfig `vic:"0.1" scope:"hidden" key:"sessions"`

	Key string `json:"string"` // will inherit parent vic attributes
}

type Cmd struct {
	Path string `vic:"0.1" scope:"hidden" key:"path"`

	Args []string `vic:"0.1" scope:"hidden" key:"args"`

	Env []string `vic:"0.1" scope:"hidden" key:"env"`

	Dir string `vic:"0.1" scope:"hidden" key:"dir"`

	Cmd *exec.Cmd `vic:"0.1" scope:"hidden" key:"cmd" recurse:"depth=0"`
}

type SessionConfig struct {
	Common `vic:"0.1" scope:"hidden" key:"common" json:"page"`

	Cmd Cmd `vic:"0.1" scope:"hidden" key:"cmd"`

	Tty bool `vic:"0.1" scope:"hidden" key:"tty"`
}

type ExecutorConfigPointersVisible struct {
	Sessions map[string]*VisibleSessionConfig `vic:"0.1" scope:"read-only" key:"sessions"`
}

type VisibleSessionConfig struct {
	Cmd Cmd `vic:"0.1" scope:"read-only" key:"cmd"`

	Tty bool `vic:"0.1" scope:"read-only" key:"tty"`
}

// [END] SLIMMED VERSION of github.com/vmware/vic/lib/metadata

// make it verbose during testing
func init() {
	logger.Level = logrus.DebugLevel
}

func TestBasic(t *testing.T) {
	type Type struct {
		Int    int     `vic:"0.1" scope:"read-write" key:"int"`
		Bool   bool    `vic:"0.1" scope:"read-write" key:"bool"`
		Float  float64 `vic:"0.1" scope:"read-write" key:"float"`
		String string  `vic:"0.1" scope:"read-write" key:"string"`
	}

	Struct := Type{
		42,
		true,
		3.14,
		"Grrr",
	}

	encoded := map[string]string{}
	Encode(MapSink(encoded), Struct)

	expected := map[string]string{
		visibleRW("int"):    "42",
		visibleRW("bool"):   "true",
		visibleRW("float"):  "3.14E+00",
		visibleRW("string"): "Grrr",
	}

	assert.Equal(t, expected, encoded, "Encoded and expected does not match")

	var decoded Type
	Decode(MapSource(encoded), &decoded)

	assert.Equal(t, Struct, decoded, "Encoded and decoded does not match")
}

func TestBasicMap(t *testing.T) {
	type Type struct {
		IntMap map[string]int `vic:"0.1" scope:"read-only" key:"intmap"`
	}

	// key is not present
	var decoded Type
	Decode(MapSource(nil), &decoded)
	assert.NotNil(t, decoded.IntMap)
	assert.Empty(t, decoded.IntMap)

	IntMap := Type{
		map[string]int{
			"1st": 12345,
			"2nd": 67890,
		},
	}

	encoded := map[string]string{}
	Encode(MapSink(encoded), IntMap)

	expected := map[string]string{
		visibleRO("intmap" + Separator + "1st"): "12345",
		visibleRO("intmap" + Separator + "2nd"): "67890",
		visibleRO("intmap"):                     "1st" + Separator + "2nd",
	}
	assert.Equal(t, expected, encoded, "Encoded and expected does not match")

	// Decode to new variable
	decoded = Type{}
	Decode(MapSource(encoded), &decoded)

	assert.Equal(t, IntMap, decoded, "Encoded and decoded does not match")

	// Decode to already existing variable
	IntMapOptimusPrime := Type{
		map[string]int{
			"first":  1,
			"second": 2,
			"1st":    0,
		},
	}
	Decode(MapSource(encoded), &IntMapOptimusPrime)

	// We expect a merge and over-write
	expectedOptimusPrime := Type{
		map[string]int{
			"1st":    12345,
			"2nd":    67890,
			"first":  1,
			"second": 2,
		},
	}
	assert.Equal(t, IntMapOptimusPrime, expectedOptimusPrime, "Decoded and expected does not match")

}

func TestBasicSlice(t *testing.T) {
	type Type struct {
		IntSlice []int `vic:"0.1" scope:"read-only" key:"intslice"`
	}

	IntSlice := Type{
		[]int{1, 2, 3, 4, 5},
	}

	encoded := map[string]string{}
	Encode(MapSink(encoded), IntSlice)

	expected := map[string]string{
		visibleRO("intslice~"): "1" + Separator + "2" + Separator + "3" + Separator + "4" + Separator + "5",
		visibleRO("intslice"):  "4",
	}
	assert.Equal(t, expected, encoded, "Encoded and expected does not match")

	var decoded Type
	decoded.IntSlice = make([]int, 1)
	Decode(MapSource(encoded), &decoded)

	assert.Equal(t, IntSlice, decoded, "Encoded and decoded does not match")
}

func TestStruct(t *testing.T) {
	type Type struct {
		Common Common `vic:"0.1" scope:"read-only" key:"common"`
	}

	Struct := Type{
		Common: Common{
			ID:   "0xDEADBEEF",
			Name: "Struct",
		},
	}

	encoded := map[string]string{}
	Encode(MapSink(encoded), Struct)

	expected := map[string]string{
		visibleRO("common/id"):    "0xDEADBEEF",
		visibleRO("common/name"):  "Struct",
		visibleRO("common/notes"): "",
	}
	assert.Equal(t, expected, encoded, "Encoded and expected does not match")

	var decoded Type
	Decode(MapSource(encoded), &decoded)

	assert.Equal(t, Struct, decoded, "Encoded and decoded does not match")
}

func TestTime(t *testing.T) {
	type Type struct {
		Time time.Time `vic:"0.1" scope:"read-only" key:"time"`
	}

	Time := Type{
		Time: time.Date(2009, 11, 10, 23, 00, 00, 0, time.UTC),
	}

	encoded := map[string]string{}
	Encode(MapSink(encoded), Time)

	expected := map[string]string{
		visibleRO("time"): "2009-11-10 23:00:00 +0000 UTC",
	}
	assert.Equal(t, encoded, expected, "Encoded and expected does not match")

	var decoded Type
	Decode(MapSource(encoded), &decoded)

	assert.Equal(t, Time, decoded, "Encoded and decoded does not match")
}

func TestNet(t *testing.T) {
	type Type struct {
		Net net.IPNet `vic:"0.1" scope:"read-only" key:"net"`
	}

	// 127.0.0.1/8
	n := net.IPNet{IP: net.IP{0x7f, 0x0, 0x0, 0x1}, Mask: net.IPMask{0xff, 0x0, 0x0, 0x0}}
	Net := Type{
		Net: n,
	}

	encoded := map[string]string{}
	Encode(MapSink(encoded), Net)

	expected := map[string]string{
		visibleRO("net/IP"):   base64.StdEncoding.EncodeToString(n.IP),
		visibleRO("net/Mask"): base64.StdEncoding.EncodeToString(n.Mask),
	}
	assert.Equal(t, expected, encoded, "Encoded and expected does not match")

	var decoded Type
	Decode(MapSource(encoded), &decoded)

	assert.Equal(t, Net, decoded, "Encoded and decoded does not match")
}

func TestNilNetPointer(t *testing.T) {
	type Type struct {
		Net *net.IPNet `vic:"0.1" scope:"read-only" key:"net"`
	}

	Net := Type{
		Net: nil,
	}

	// Net should be nil - pointers are supposed to be nil if the referenced tree is zero valued
	encoded := map[string]string{}
	Encode(MapSink(encoded), Net)

	expected := map[string]string{}
	assert.Equal(t, expected, encoded, "Encoded and expected does not match")

	var decoded Type
	Decode(MapSource(encoded), &decoded)

	assert.Equal(t, Net, decoded, "Encoded and decoded does not match")
}

func TestPointer(t *testing.T) {
	type Type struct {
		Pointer           *ContainerVM `vic:"0.1" scope:"hidden" key:"pointer"`
		PointerOmitnested *ContainerVM `vic:"0.1" scope:"non-persistent" key:"pointeromitnested" recurse:"depth=0"`
	}

	Pointer := Type{
		Pointer: &ContainerVM{Version: "0.1"},
	}

	encoded := map[string]string{}
	Encode(MapSink(encoded), Pointer)

	expected := map[string]string{
		visibleRO("pointer/common/id"):    "",
		visibleRO("pointer/common/name"):  "",
		visibleRO("pointer/common/notes"): "",
		"pointer/version":                 "0.1",
	}
	assert.Equal(t, expected, encoded, "Encoded and expected does not match")

	var decoded Type
	Decode(MapSource(encoded), &decoded)

	assert.Equal(t, Pointer, decoded, "Encoded and decoded does not match")
}

func TestInheritenceOfNonPersistence(t *testing.T) {
	type CommonPersistence struct {
		ExecutionEnvironment string `vic:"0.1" recurse:"depth=0"`

		ID string `vic:"0.1" scope:"read-only" key:"id"`

		Name string `vic:"0.1" scope:"read-only" key:"name"`

		Notes string `vic:"0.1" scope:"hidden" key:"notes"`
	}

	type Type struct {
		Common CommonPersistence `vic:"0.1" scope:"read-write,non-persistent" key:"common"`
	}

	Struct := Type{
		Common: CommonPersistence{
			ID:   "0xDEADBEEF",
			Name: "Struct",
		},
	}

	encoded := map[string]string{}
	filterSink := ScopeFilterSink(NonPersistent|Hidden, MapSink(encoded))
	Encode(filterSink, Struct)

	expected := map[string]string{
		visibleRONonpersistent("common/id"):   "0xDEADBEEF",
		visibleRONonpersistent("common/name"): "Struct",
	}
	assert.Equal(t, expected, encoded, "Encoded and expected does not match")

	var decoded Type
	Decode(MapSource(encoded), &decoded)

	assert.Equal(t, Struct, decoded, "Encoded and decoded does not match")
}

func TestInheritenceOfNonPersistenceWithPointer(t *testing.T) {

	type Persistence struct {
		ExecutorConfigPointersVisible `vic:"0.1" scope:"read-only,non-persistent" key:"pointers"`
	}

	Struct := Persistence{
		ExecutorConfigPointersVisible: ExecutorConfigPointersVisible{
			Sessions: map[string]*VisibleSessionConfig{
				"primary": {
					Tty: true,
				},
			},
		},
	}

	encoded := map[string]string{}
	filterSink := ScopeFilterSink(NonPersistent|Hidden, MapSink(encoded))
	Encode(filterSink, Struct)

	expected := map[string]string{
		visibleRONonpersistent("pointers/sessions"):                             "primary",
		visibleRONonpersistent("pointers/sessions" + Separator + "primary/tty"): "true",
	}
	assert.Equal(t, expected, encoded, "Encoded and expected does not match")

	var decoded Persistence
	Decode(MapSource(encoded), &decoded)

	assert.Equal(t, Struct, decoded, "Encoded and decoded does not match")
}

func TestFilterSink(t *testing.T) {
	type CommonPersistence struct {
		ExecutionEnvironment string `vic:"0.1" recurse:"depth=0"`

		ID string `vic:"0.1" scope:"read-only" key:"id"`

		Name string `vic:"0.1" scope:"read-only,non-persistent" key:"name"`

		Notes string `vic:"0.1" scope:"read-only" key:"notes"`
	}

	type Type struct {
		Common CommonPersistence `vic:"0.1" scope:"read-write" key:"common"`
	}

	Struct := Type{
		Common: CommonPersistence{
			ID:   "0xDEADBEEF",
			Name: "Struct",
		},
	}

	encoded := map[string]string{}
	filterSink := ScopeFilterSink(NonPersistent|Hidden, MapSink(encoded))
	Encode(filterSink, Struct)

	expected := map[string]string{
		visibleRONonpersistent("common/name"): "Struct",
	}
	assert.Equal(t, expected, encoded, "Encoded and expected does not match")

	// strip ID as that would be filtered out
	Struct.Common.ID = ""

	var decoded Type
	Decode(MapSource(encoded), &decoded)

	assert.Equal(t, Struct, decoded, "Encoded and decoded does not match")
}
