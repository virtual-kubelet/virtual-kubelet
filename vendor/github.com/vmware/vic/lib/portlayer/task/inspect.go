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

package task

import (
	"fmt"

	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/vic/lib/config/executor"
	"github.com/vmware/vic/lib/portlayer/exec"
	"github.com/vmware/vic/pkg/trace"
)

// Inspect the task configuration from the containerVM config
func Inspect(op *trace.Operation, h interface{}, id string) (*executor.SessionConfig, error) {
	defer trace.End(trace.Begin(id))

	handle, ok := h.(*exec.Handle)
	if !ok {
		return nil, fmt.Errorf("Type assertion failed for %#+v", handle)
	}

	stasks := handle.ExecConfig.Sessions
	etasks := handle.ExecConfig.Execs

	_, okS := stasks[id]
	_, okE := etasks[id]

	op.Debugf("target task ID: %s", id)
	op.Debugf("session tasks during inspect: %s", stasks)
	op.Debugf("exec tasks during inspect: %s", etasks)

	if !okS && !okE {
		if handle.Runtime.PowerState == types.VirtualMachinePowerStatePoweredOff {
			powerStateError := TaskPowerStateError{
				msg: fmt.Sprintf("the operation cannot be completed, container(%s) has been shut down during the operations execution.", handle.ExecConfig.ID),
			}
			return nil, powerStateError
		}

		return nil, fmt.Errorf("unknown task ID: %s", id)
	}

	tasks := stasks
	if handle.Runtime != nil && handle.Runtime.PowerState != types.VirtualMachinePowerStatePoweredOff {
		op.Debugf("Task configuration applies to ephemeral set")
		tasks = etasks
	}

	if _, ok := tasks[id]; !ok {
		return nil, fmt.Errorf("Cannot find task %s", id)
	}

	return tasks[id], nil
}

// Special Error types for a task inspect
type TaskPowerStateError struct {
	msg string
}

func (e TaskPowerStateError) Error() string {
	return e.msg
}
