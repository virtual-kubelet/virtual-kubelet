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

package convert

import (
	"fmt"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/go-units"

	"github.com/vmware/vic/lib/apiservers/portlayer/models"
)

// State will create and return a docker ContainerState object
// from the passed vic ContainerInfo object
func State(info *models.ContainerInfo) *types.ContainerState {
	// ensure we have the data we need
	if info == nil || info.ProcessConfig == nil || info.ContainerConfig == nil {
		return nil
	}

	dockerState := &types.ContainerState{}

	// convert start / stop times
	var started time.Time
	var finished time.Time

	if info.ProcessConfig.StartTime > 0 {
		started = time.Unix(info.ProcessConfig.StartTime, 0)
		dockerState.StartedAt = time.Unix(info.ProcessConfig.StartTime, 0).Format(time.RFC3339Nano)
	}

	if info.ProcessConfig.StopTime > 0 {
		finished = time.Unix(info.ProcessConfig.StopTime, 0)
		dockerState.FinishedAt = time.Unix(info.ProcessConfig.StopTime, 0).Format(time.RFC3339Nano)
	}

	// set docker status to state and we'll change if needed
	dockStatus := info.ContainerConfig.State
	// set exitCode and change if needed
	exitCode := int(info.ProcessConfig.ExitCode)

	switch info.ContainerConfig.State {
	case "Running":
		// if we don't have a start date leave the status as the state
		if !started.IsZero() {
			dockStatus = fmt.Sprintf("Up %s", units.HumanDuration(time.Now().UTC().Sub(started)))
			dockerState.Running = true
		}
	case "Stopped":
		// if we don't have a finished date then don't process exitCode and return "Stopped" for the status
		if !finished.IsZero() {
			// interrogate the process status returned from the portlayer
			// and based on status text and exit codes set the appropriate
			// docker exit code
			if strings.Contains(info.ProcessConfig.Status, "permission denied") {
				exitCode = 126
			} else if strings.Contains(info.ProcessConfig.Status, "no such") {
				exitCode = 127
			} else if info.ProcessConfig.Status == "true" && exitCode == -1 {
				// most likely the process was killed via the cli
				// or received a sigkill
				exitCode = 137
			} else if info.ProcessConfig.Status == "" && exitCode == -1 {
				// the process was stopped via the cli
				// or received a sigterm
				exitCode = 143
			}

			dockStatus = fmt.Sprintf("Exited (%d) %s ago", exitCode, units.HumanDuration(time.Now().UTC().Sub(finished)))
		}
	}
	dockerState.Status = dockStatus
	dockerState.ExitCode = exitCode
	dockerState.Pid = int(info.ProcessConfig.Pid)
	dockerState.Error = info.ProcessConfig.ErrorMsg

	return dockerState
}
