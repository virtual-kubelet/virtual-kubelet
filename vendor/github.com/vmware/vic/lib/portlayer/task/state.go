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

package task

import (
	"github.com/vmware/vic/lib/config/executor"
	"github.com/vmware/vic/lib/constants"
	"github.com/vmware/vic/pkg/trace"
)

// State takes the given snapshot of a Task and determines the state of the Task.
// NOTE: this does not take into account the powerstate of the task owner at the moment.
//       callers should act on power state information before calling this.
func State(op trace.Operation, e *executor.SessionConfig) (string, error) {

	// DO NOT ASSUME THAT WE CANNOT GET STATE TEARING - THE ORDERING OF SERIALIZATION IS GO REFLECT PACKAGE DEPENDENT
	switch {
	case e.Started == "" && e.Detail.StartTime == 0 && e.Detail.StopTime == 0:
		return constants.TaskCreatedState, nil
	case e.Started == "true" && e.Detail.StartTime > e.StopTime:
		return constants.TaskRunningState, nil
	case e.Started == "true" && e.Detail.StartTime <= e.Detail.StopTime:
		return constants.TaskStoppedState, nil
	case e.Started != "" && e.Started != "true" && e.StartTime >= e.Detail.StopTime:
		// NOTE: this assumes that StopTime does not get set. We really need to investigate this further as it does not look like it will be the case based on the way the child reaper attempts to write things.
		return constants.TaskFailedState, nil
	default:
		op.Debugf("task state cannot be determined (start=%s, starttime: %s, stoptime: %s)", e.Started, e.StartTime, e.StopTime)
		return constants.TaskUnknownState, nil
	}
}
