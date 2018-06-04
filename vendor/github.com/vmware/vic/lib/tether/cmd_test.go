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

package tether

import (
	"fmt"
	"os/exec"
	"testing"

	log "github.com/Sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/vmware/vic/lib/config/executor"
	"github.com/vmware/vic/pkg/vsphere/extraconfig"
)

/////////////////////////////////////////////////////////////////////////////////////
// TestPathLookup constructs the spec for a Session where the binary path must be
// resolved from the PATH environment variable - this is a variation from normal
// Cmd handling where that is done during creation of Cmd
//

func TestPathLookup(t *testing.T) {
	_, mocker := testSetup(t)
	defer testTeardown(t, mocker)

	cfg := executor.ExecutorConfig{
		ExecutorConfigCommon: executor.ExecutorConfigCommon{
			ID:   "pathlookup",
			Name: "tether_test_executor",
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
			},
		},
	}

	_, src, err := RunTether(t, &cfg, mocker)
	assert.NoError(t, err, "Didn't expected error from runTether")

	result := ExecutorConfig{}
	extraconfig.Decode(src, &result)

	assert.Equal(t, "true", result.Sessions["pathlookup"].Started, "Expected command to have been started successfully")
	assert.Equal(t, 0, result.Sessions["pathlookup"].ExitStatus, "Expected command to have exited cleanly")
}

func TestRelativePath(t *testing.T) {
	_, mocker := testSetup(t)
	defer testTeardown(t, mocker)

	cfg := executor.ExecutorConfig{
		ExecutorConfigCommon: executor.ExecutorConfigCommon{
			ID:   "relpath",
			Name: "tether_test_executor",
		},

		Sessions: map[string]*executor.SessionConfig{
			"relpath": {
				Common: executor.Common{
					ID:   "relpath",
					Name: "tether_test_session",
				},
				Tty:    false,
				Active: true,

				Cmd: executor.Cmd{
					// test relative path
					Path: "./date",
					Args: []string{"./date", "--reference=/"},
					Env:  []string{"PATH="},
					Dir:  "/bin",
				},
			},
		},
	}

	_, src, err := RunTether(t, &cfg, mocker)
	assert.NoError(t, err, "Didn't expected error from RunTether")

	result := ExecutorConfig{}
	extraconfig.Decode(src, &result)

	assert.Equal(t, "true", result.Sessions["relpath"].Started, "Expected command to have been started successfully")
	assert.Equal(t, 0, result.Sessions["relpath"].ExitStatus, "Expected command to have exited cleanly")
}

func TestAbsPath(t *testing.T) {
	_, mocker := testSetup(t)
	defer testTeardown(t, mocker)

	cfg := executor.ExecutorConfig{
		ExecutorConfigCommon: executor.ExecutorConfigCommon{
			ID:   "abspath",
			Name: "tether_test_executor",
		},

		Sessions: map[string]*executor.SessionConfig{
			"abspath": {
				Common: executor.Common{
					ID:   "abspath",
					Name: "tether_test_session",
				},
				Tty:    false,
				Active: true,

				Cmd: executor.Cmd{
					// test abs path
					Path: "/bin/date",
					Args: []string{"date", "--reference=/"},
					Env:  []string{},
					Dir:  "/",
				},
			},
		},
	}

	_, src, err := RunTether(t, &cfg, mocker)
	assert.NoError(t, err, "Didn't expected error from RunTether")

	result := ExecutorConfig{}
	extraconfig.Decode(src, &result)

	assert.Equal(t, "true", result.Sessions["abspath"].Started, "Expected command to have been started successfully")
	assert.Equal(t, 0, result.Sessions["abspath"].ExitStatus, "Expected command to have exited cleanly")

	// read the output from the session
	log := mocker.SessionLogBuffer.Bytes()

	// block until tether exits
	<-mocker.Cleaned

	// run the command directly
	out, err := exec.Command("/bin/date", "--reference=/").Output()
	if err != nil {
		fmt.Printf("Failed to run date for comparison data: %s", err)
		t.Error(err)
		return
	}

	if !assert.Equal(t, out, log) {
		return
	}
}

func TestHalt(t *testing.T) {
	_, mocker := testSetup(t)
	defer testTeardown(t, mocker)

	cfg := executor.ExecutorConfig{
		ExecutorConfigCommon: executor.ExecutorConfigCommon{
			ID:   "abspath",
			Name: "tether_test_executor",
		},

		Sessions: map[string]*executor.SessionConfig{
			"abspath": {
				Common: executor.Common{
					ID:   "abspath",
					Name: "tether_test_session",
				},
				Tty:    false,
				Active: true,

				Cmd: executor.Cmd{
					// test abs path
					Path: "/bin/date",
					Args: []string{"date", "--reference=/"},
					Env:  []string{},
					Dir:  "/",
				},
			},
		},
	}

	_, src, err := RunTether(t, &cfg, mocker)
	assert.NoError(t, err, "Didn't expected error from RunTether")

	// block until tether exits
	<-mocker.Cleaned

	result := ExecutorConfig{}
	extraconfig.Decode(src, &result)

	assert.Equal(t, "true", result.Sessions["abspath"].Started, "Expected command to have been started successfully")
	assert.Equal(t, 0, result.Sessions["abspath"].ExitStatus, "Expected command to have exited cleanly")

	// read the output from the session
	log := mocker.SessionLogBuffer.Bytes()

	// run the command directly
	out, err := exec.Command("/bin/date", "--reference=/").Output()
	if err != nil {
		fmt.Printf("Failed to run date for comparison data: %s", err)
		t.Error(err)
		return
	}

	if !assert.Equal(t, out, log) {
		return
	}
}

func TestAbsPathRepeat(t *testing.T) {
	log.SetLevel(log.WarnLevel)

	for i := 0; i < 2000 && !t.Failed(); i++ {
		TestAbsPath(t)
	}

	defer log.SetLevel(log.DebugLevel)

}

//
/////////////////////////////////////////////////////////////////////////////////////

/////////////////////////////////////////////////////////////////////////////////////
// TestMissingBinaryConfig constructs the spec for a Session with invalid binary path
//

func TestMissingBinary(t *testing.T) {
	_, mocker := testSetup(t)
	defer testTeardown(t, mocker)

	cfg := executor.ExecutorConfig{
		ExecutorConfigCommon: executor.ExecutorConfigCommon{
			ID:   "missing",
			Name: "tether_test_executor",
		},

		Sessions: map[string]*executor.SessionConfig{
			"missing": {
				Common: executor.Common{
					ID:   "missing",
					Name: "tether_test_session",
				},
				Tty:    false,
				Active: true,

				Cmd: executor.Cmd{
					// test relative path
					Path: "/not/there",
					Args: []string{"/not/there"},
					Env:  []string{"PATH=/not"},
					Dir:  "/",
				},
			},
		},
	}

	_, src, err := RunTether(t, &cfg, mocker)
	assert.Error(t, err, "Expected error from RunTether")

	// refresh the cfg with current data
	extraconfig.Decode(src, &cfg)

	// check the launch status was failed
	status := cfg.Sessions["missing"].Started

	assert.Equal(t, "stat /not/there: no such file or directory", status, "Expected status to have a command not found error message")
}

//
/////////////////////////////////////////////////////////////////////////////////////

/////////////////////////////////////////////////////////////////////////////////////
// TestMissingRelativeBinaryConfig constructs the spec for a Session with invalid binary path
//

func TestMissingRelativeBinary(t *testing.T) {
	_, mocker := testSetup(t)
	defer testTeardown(t, mocker)

	cfg := executor.ExecutorConfig{
		ExecutorConfigCommon: executor.ExecutorConfigCommon{
			ID:   "missing",
			Name: "tether_test_executor",
		},

		Sessions: map[string]*executor.SessionConfig{
			"missing": {
				Common: executor.Common{
					ID:   "missing",
					Name: "tether_test_session",
				},
				Tty:    false,
				Active: true,

				Cmd: executor.Cmd{
					// test relative path
					Path: "notthere",
					Args: []string{"notthere"},
					Env:  []string{"PATH=/not"},
					Dir:  "/",
				},
			},
		},
	}

	_, src, err := RunTether(t, &cfg, mocker)
	assert.Error(t, err, "Expected error from RunTether")

	// refresh the cfg with current data
	extraconfig.Decode(src, &cfg)

	// check the launch status was failed
	status := cfg.Sessions["missing"].Started

	assert.Equal(t, "notthere: no such executable in PATH", status, "Expected status to have a command not found error message")
}

//
/////////////////////////////////////////////////////////////////////////////////////
