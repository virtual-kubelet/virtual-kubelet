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

// Join adds the task configuration to the containerVM config
func Join(op *trace.Operation, h interface{}, task *executor.SessionConfig) (interface{}, error) {
	defer trace.End(trace.Begin(task.ID))

	handle, ok := h.(*exec.Handle)
	if !ok {
		return nil, fmt.Errorf("Type assertion failed for %#+v", handle)
	}

	// if the container isn't running then this is a persistent change
	var tasks map[string]*executor.SessionConfig
	if handle.Runtime == nil || handle.Runtime.PowerState == types.VirtualMachinePowerStatePoweredOff {
		if handle.ExecConfig.Sessions == nil {
			handle.ExecConfig.Sessions = make(map[string]*executor.SessionConfig)
		}
		tasks = handle.ExecConfig.Sessions
		task.Diagnostics.SysLogConfig = handle.ExecConfig.Diagnostics.SysLogConfig
	} else {
		op.Debugf("Task join configuration applies to ephemeral set")
		if handle.ExecConfig.Execs == nil {
			handle.ExecConfig.Execs = make(map[string]*executor.SessionConfig)
		}
		tasks = handle.ExecConfig.Execs

		if err := compatible(h); err != nil {
			return nil, err
		}
	}

	_, ok = tasks[task.ID]
	if ok {
		return nil, fmt.Errorf("task ID collides: %s", task.ID)
	}

	op.Debugf("Adding task (%s): %s", task.Cmd.Path, task.ID)

	tasks[task.ID] = task

	return handle, nil
}
