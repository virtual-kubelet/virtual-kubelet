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
	"io/ioutil"
	"os"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/vmware/vic/lib/config/executor"
	"github.com/vmware/vic/pkg/vsphere/extraconfig"
)

func TestExec(t *testing.T) {
	_, mocker := testSetup(t)

	defer testTeardown(t, mocker)

	dir, err := ioutil.TempDir("", "tetherTestExec")
	if !assert.Nil(t, err, "Unable to create temp file") {
		t.FailNow()
	}

	flagFile := path.Join(dir, "flagfile")
	defer os.RemoveAll(flagFile)

	cfg := executor.ExecutorConfig{
		ExecutorConfigCommon: executor.ExecutorConfigCommon{
			ID:   "primary",
			Name: "tether_test_executor",
		},

		Sessions: map[string]*executor.SessionConfig{
			"primary": {
				Common: executor.Common{
					ID:   "primary",
					Name: "tether_test_session",
				},
				Tty:       true,
				Active:    true,
				OpenStdin: true,

				Cmd: executor.Cmd{
					// test abs path
					Path: "/bin/bash",
					Args: []string{"/bin/bash", "-c", fmt.Sprintf("until /usr/bin/test -e %s;do /bin/sleep 0.1;done", flagFile)},
					Env:  []string{},
					Dir:  "/",
				},
			},
		},
	}

	tthr, src, sink := StartTether(t, &cfg, mocker)

	<-mocker.Started

	// at this point the primary process should be up and running, so grab the pid
	result := ExecutorConfig{}
	extraconfig.Decode(src, &result)

	for result.Sessions["primary"].Started != "true" {
		time.Sleep(200 * time.Millisecond)
		extraconfig.Decode(src, &result)
	}

	// configure a command to kill the primary
	cfg.Execs = map[string]*executor.SessionConfig{
		"touch": {
			Common: executor.Common{
				ID:   "touch",
				Name: "tether_test_session",
			},
			Tty:    false,
			Active: true,

			Cmd: executor.Cmd{
				// test abs path
				Path: "/bin/touch",
				Args: []string{"touch", flagFile},
				Env:  []string{},
				Dir:  "/",
			},
		},
	}

	// update the active config
	extraconfig.Encode(sink, cfg)

	// trigger tether reload
	tthr.Reload()

	<-mocker.Reloaded

	// block until tether exits - exit within test timeout is a pass given the pairing of processes in the test
	<-mocker.Cleaned
}

func TestMissingBinaryNonFatal(t *testing.T) {
	_, mocker := testSetup(t)

	defer testTeardown(t, mocker)

	dir, err := ioutil.TempDir("", "tetherTestExec")
	if !assert.Nil(t, err, "Unable to create temp file") {
		t.FailNow()
	}

	flagFile := path.Join(dir, "flagfile")
	defer os.RemoveAll(flagFile)

	cfg := executor.ExecutorConfig{
		ExecutorConfigCommon: executor.ExecutorConfigCommon{
			ID:   "primary",
			Name: "tether_test_executor",
		},
		Diagnostics: executor.Diagnostics{
			DebugLevel: 2,
		},

		Sessions: map[string]*executor.SessionConfig{
			"primary": {
				Common: executor.Common{
					ID:   "primary",
					Name: "tether_test_session",
				},
				Tty:       true,
				Active:    true,
				OpenStdin: true,

				Cmd: executor.Cmd{
					// test abs path
					Path: "/bin/bash",
					Args: []string{"/bin/bash", "-c", fmt.Sprintf("until /usr/bin/test -e %s;do /bin/sleep 0.1;done", flagFile)},
					Env:  []string{},
					Dir:  "/",
				},
			},
		},
	}

	tthr, src, sink := StartTether(t, &cfg, mocker)

	<-mocker.Started

	// at this point the primary process should be up and running, so grab the pid
	extraconfig.Decode(src, &cfg)

	for cfg.Sessions["primary"].Started != "true" {
		time.Sleep(time.Second)
		extraconfig.Decode(src, &cfg)
	}

	// configure a command to kill the primary
	cfg.Execs = map[string]*executor.SessionConfig{
		"nonfatal": {
			Common: executor.Common{
				ID:   "nonfatal",
				Name: "nonfatal_exec_session",
			},
			Tty:    false,
			Active: true,

			Cmd: executor.Cmd{
				// test abs path
				Path: "/not/there",
				Args: []string{"/not/there"},
				Env:  []string{},
				Dir:  "/",
			},
		},
	}

	// update the active config
	extraconfig.Encode(sink, cfg)

	// trigger tether reload
	tthr.Reload()

	// wait for the reload to be processed
	<-mocker.Reloaded

	for cfg.Execs["nonfatal"].Started == "" {
		time.Sleep(time.Second)
		extraconfig.Decode(src, &cfg)
	}

	// reconfigure the missing process to make it inactive
	cfg.Execs["nonfatal"].Active = false

	// configure a command to kill the primary
	cfg.Execs["touch"] = &executor.SessionConfig{
		Common: executor.Common{
			ID:   "touch",
			Name: "tether_test_session",
		},
		Tty:    false,
		Active: true,

		Cmd: executor.Cmd{
			// test abs path
			Path: "/bin/touch",
			Args: []string{"touch", flagFile},
			Env:  []string{},
			Dir:  "/",
		},
		ExitStatus: -1,
	}

	// update the active config
	extraconfig.Encode(sink, cfg)

	// trigger tether reload
	tthr.Reload()

	// block until tether exits - exit within test timeout is a pass given the pairing of processes in the test
	<-mocker.Cleaned

	// update config - into existing data
	extraconfig.Decode(src, &cfg)

	// check that nonfatal error was as expected
	status := cfg.Execs["nonfatal"].Started
	assert.Equal(t, "stat /not/there: no such file or directory", status, "Expected nonfatal status to have a command not found error message")
	status = cfg.Execs["touch"].Started
	assert.Equal(t, "true", status, "Expected touch status to be clean")
	exit := cfg.Execs["touch"].ExitStatus
	assert.Equal(t, 0, exit, "Expected touch exit status to be success")

}

func TestExecHalt(t *testing.T) {
	_, mocker := testSetup(t)

	defer testTeardown(t, mocker)

	dir, err := ioutil.TempDir("", "tetherTestExec")
	if !assert.Nil(t, err, "Unable to create temp file") {
		t.FailNow()
	}

	flagFile := path.Join(dir, "flagfile")
	defer os.RemoveAll(flagFile)

	cfg := executor.ExecutorConfig{
		ExecutorConfigCommon: executor.ExecutorConfigCommon{
			ID:   "primary",
			Name: "tether_test_executor",
		},

		Sessions: map[string]*executor.SessionConfig{
			"primary": {
				Common: executor.Common{
					ID:   "primary",
					Name: "tether_test_session",
				},
				Tty:       true,
				Active:    true,
				OpenStdin: true,

				Cmd: executor.Cmd{
					// test abs path
					Path: "/bin/bash",
					Args: []string{"/bin/bash", "-c", fmt.Sprintf("until /usr/bin/test -e %s;do /bin/sleep 0.1;done", flagFile)},
					Env:  []string{},
					Dir:  "/",
				},
			},
		},
	}

	tthr, src, sink := StartTether(t, &cfg, mocker)

	<-mocker.Started

	// at this point the primary process should be up and running, so grab the pid
	result := ExecutorConfig{}
	extraconfig.Decode(src, &result)

	for result.Sessions["primary"].Started != "true" {
		time.Sleep(200 * time.Millisecond)
		extraconfig.Decode(src, &result)
	}

	// reconfigure the primary process to make it inactive
	cfg.Sessions["primary"].Active = false

	// update the active config
	extraconfig.Encode(sink, cfg)

	// trigger tether reload
	tthr.Reload()

	// block until tether exits - exit within test timeout is a pass given we're deactivating the primary
	<-mocker.Cleaned

	// process should have been killed via signal due to deactivation
	extraconfig.Decode(src, &result)
	assert.Equal(t, -1, result.Sessions["primary"].ExitStatus, "Expected exit code for primary to indicate signal death")
}
