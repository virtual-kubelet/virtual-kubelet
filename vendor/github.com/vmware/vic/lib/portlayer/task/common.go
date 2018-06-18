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
	"github.com/vmware/vic/lib/migration/feature"
	"github.com/vmware/vic/lib/portlayer/exec"
	"github.com/vmware/vic/pkg/trace"
)

// Toggle launching of the process in the container
func toggleActive(op *trace.Operation, h interface{}, id string, active bool) (interface{}, error) {
	defer trace.End(trace.Begin(id))

	handle, ok := h.(*exec.Handle)
	if !ok {
		return nil, fmt.Errorf("Type assertion failed for %#+v", handle)
	}

	stasks := handle.ExecConfig.Sessions
	etasks := handle.ExecConfig.Execs

	taskS, okS := stasks[id]
	taskE, okE := etasks[id]

	if !okS && !okE {
		return nil, fmt.Errorf("unknown task ID: %s", id)
	}

	task := taskS
	if handle.Runtime != nil && handle.Runtime.PowerState != types.VirtualMachinePowerStatePoweredOff {
		op.Debugf("Task bind configuration applies to ephemeral set")
		task = taskE

		if err := compatible(handle); err != nil {
			return nil, err
		}
	}

	// if no task has been joined that can be manipulated in the container's current state
	if task == nil {
		return nil, fmt.Errorf("Cannot modify task %s in current state", id)
	}

	op.Debugf("Toggling active state of task %s (%s): %t", id, task.Cmd.Path, active)

	task.Active = active
	handle.Reload()

	return handle, nil
}

func compatible(h interface{}) error {
	if handle, ok := h.(*exec.Handle); ok {
		if handle.DataVersion < feature.TasksSupportedVersion {
			return fmt.Errorf("running tasks not supported for this container")
		}

		return nil
	}

	return fmt.Errorf("Type assertion failed for %#+v", h)
}
