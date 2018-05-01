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

package exec

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/vmware/vic/pkg/uid"
)

func TestStateStringer(t *testing.T) {

	c := &Container{
		ContainerInfo: ContainerInfo{
			state: StateRunning,
		},
	}

	assert.Equal(t, "Running", c.state.String())
	c.state = StateStopped
	assert.Equal(t, "Stopped", c.state.String())
	c.state = StateStopping
	assert.Equal(t, "Stopping", c.state.String())
	c.state = StateRemoving
	assert.Equal(t, "Removing", c.state.String())
	c.state = StateStarting
	assert.Equal(t, "Starting", c.state.String())
	c.state = StateCreated
	assert.Equal(t, "Created", c.state.String())
}

func NewContainer(id uid.UID) *Handle {
	con := &Container{
		ContainerInfo: ContainerInfo{
			state: StateCreating,
		},
		newStateEvents: make(map[State]chan struct{}),
	}

	h := newHandle(con)
	h.ExecConfig.ID = id.String()

	return h
}
