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

package main

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/vmware/vic/lib/config/executor"
	"github.com/vmware/vic/lib/tether"
	"github.com/vmware/vic/pkg/vsphere/extraconfig"
)

/////////////////////////////////////////////////////////////////////////////////////
// TestPathLookup constructs the spec for a Session where the binary path must be
// resolved from the PATH environment variable - this is a variation from normal
// Cmd handling where that is done during creation of Cmd
//

func TestRestart(t *testing.T) {
	testSetup(t)
	defer testTeardown(t)

	cfg := executor.ExecutorConfig{
		ExecutorConfigCommon: executor.ExecutorConfigCommon{
			ID:   "pathlookup",
			Name: "tether_test_executor",
		},
		Diagnostics: executor.Diagnostics{
			DebugLevel: 2,
		},
		Sessions: map[string]*executor.SessionConfig{
			"pathlookup": {
				Common: executor.Common{
					ID:   "pathlookup",
					Name: "tether_test_session",
				},
				Tty:    false,
				Active: true,

				Cmd: executor.Cmd{
					// test relative path
					Path: "date",
					Args: []string{"date", "--reference=/"},
					Env:  []string{"PATH=/bin"},
					Dir:  "/bin",
				},
				Restart: true,
			},
		},
	}

	tthr, src := StartTether(t, &cfg)

	// wait for initialization
	<-Mocked.Started

	result := &tether.ExecutorConfig{}
	extraconfig.Decode(src, result)

	// Started returns when we reload but that doesn't mean that the process is started
	// Try multiple times before giving up
	for i := 0; i < 10; i++ {
		if result.Sessions["pathlookup"].Started != "" {
			break
		}
		time.Sleep(time.Duration(i) * time.Millisecond)
	}

	assert.Equal(t, 0, result.Sessions["pathlookup"].ExitStatus, "Expected command to have exited cleanly")
	assert.True(t, result.Sessions["pathlookup"].Restart, "Expected command to be configured for restart")

	// wait for the resurrection count to max out the channel
	for result.Sessions["pathlookup"].Diagnostics.ResurrectionCount < 10 {
		result = &tether.ExecutorConfig{}
		extraconfig.Decode(src, &result)
		assert.Equal(t, 0, result.Sessions["pathlookup"].ExitStatus, "Expected command to have exited cleanly")
		// proceed to the next reincarnation
		<-Mocked.SessionExit
		tthr.Reload()
	}

	// read the output from the session
	log := Mocked.SessionLogBuffer.Bytes()

	// the tether has to be stopped before comparison on the reaper may swaller exec.Wait
	tthr.Stop()
	<-Mocked.Cleaned

	// run the command directly
	out, err := exec.Command("/bin/date", "--reference=/").Output()
	if err != nil {
		fmt.Printf("Failed to run date for comparison data: %s", err)
		t.Error(err)
		return
	}

	assert.True(t, strings.HasPrefix(string(log), string(out)), "Expected the data to be constant - first invocation doesn't match")
	assert.True(t, strings.HasSuffix(string(log), string(out)), "Expected the data to be constant - last invocation doesn't match")
}
