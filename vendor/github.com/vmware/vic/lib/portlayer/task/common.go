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

	"github.com/vmware/vic/lib/config/executor"
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

	op.Debugf("target task ID: %s", id)
	op.Debugf("session tasks during inspect: %s", handle.ExecConfig.Sessions)
	// print all of them, otherwise we will have to assemble the id list regardless of
	// the log level at the moment. If there is a way to check the log level we should
	// do that. since the other approach will slow down all calls to toggleActive.
	op.Debugf("exec tasks during inspect: %s", handle.ExecConfig.Execs)

	var task *executor.SessionConfig
	if taskS, okS := handle.ExecConfig.Sessions[id]; okS {
		task = taskS
	} else if taskE, okE := handle.ExecConfig.Execs[id]; okE {
		task = taskE
	}

	// if no task has been joined that can be manipulated in the container's current state
	if task == nil {
		// FIXME return a compatibility style error here. Propagate it back to the user.
		if err := compatible(handle); err != nil {
			return nil, err
		}

		// NOTE: this was the previous error, before merging we need to decide which one to use.
		// return nil, fmt.Errorf("Cannot modify task %s in current state", id)
		return nil, TaskNotFoundError{msg: fmt.Sprintf("Cannot find task %s", id)}
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
