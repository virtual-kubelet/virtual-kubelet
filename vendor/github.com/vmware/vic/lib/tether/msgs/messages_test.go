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

package msgs

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/vmware/vic/lib/migration/feature"
)

func TestWindowChange(t *testing.T) {
	s := &WindowChangeMsg{1, 2, 3, 4}

	assert.Equal(t, s.RequestType(), WindowChangeReq)

	tmp := s.Marshal()
	out := &WindowChangeMsg{}
	out.Unmarshal(tmp)

	assert.Equal(t, s, out)
}

func TestSignal(t *testing.T) {
	s := &SignalMsg{"HUP"}

	assert.Equal(t, s.RequestType(), SignalReq)

	tmp := s.Marshal()
	out := &SignalMsg{}
	out.Unmarshal(tmp)

	assert.Equal(t, s, out)

	for _, name := range []string{"SIGQUIT", "QUIT", "quit", "3"} {
		err := out.FromString(name)
		if err != nil {
			t.Fatal(err)
		}
	}

	for _, name := range []string{"SIGNOPE", "nope", "0", "-1"} {
		err := out.FromString(name)
		if err == nil {
			t.Errorf("expected error parsing %q", name)
		}
	}
}

func TestContainers(t *testing.T) {
	s := &ContainersMsg{IDs: []string{"foo", "bar", "baz"}}

	assert.Equal(t, s.RequestType(), ContainersReq)

	tmp := s.Marshal()
	out := &ContainersMsg{}
	out.Unmarshal(tmp)

	assert.Equal(t, s, out)
}

func TestVersion(t *testing.T) {
	s := &VersionMsg{Version: feature.MaxPluginVersion - 1}

	assert.Equal(t, s.RequestType(), VersionReq)

	tmp := s.Marshal()
	out := &VersionMsg{}
	out.Unmarshal(tmp)

	assert.Equal(t, s, out)
}
