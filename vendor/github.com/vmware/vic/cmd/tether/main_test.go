// Copyright 2016-2017 VMware, Inc. All Rights Reserved.
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
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"syscall"
	"testing"

	log "github.com/Sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/vmware/vic/lib/tether"
	"github.com/vmware/vic/pkg/trace"
)

func init() {
	trace.Logger.Level = log.DebugLevel
}

// createFakeDevices creates regular files or pipes in place of the char devices used
// in a full VM
func createFakeDevices() error {
	var err error

	// create control channel
	path := fmt.Sprintf("%s/ttyS0", pathPrefix)
	err = syscall.Mkfifo(path+"s", uint32(backchannelMode))
	if err != nil {
		detail := fmt.Sprintf("failed to create fifo pipe %ss for com0: %s", path, err)
		return errors.New(detail)
	}
	err = syscall.Mkfifo(path+"c", uint32(backchannelMode))
	if err != nil {
		detail := fmt.Sprintf("failed to create fifo pipe %sc for com0: %s", path, err)
		return errors.New(detail)
	}
	log.Debugf("created %s/ttyS0{c,s} as raw conn pipes", pathPrefix)

	// others are non-interactive
	for i := 1; i < 3; i++ {
		path = fmt.Sprintf("%s/ttyS%d", pathPrefix, i)
		_, err = os.Create(path)
		if err != nil {
			detail := fmt.Sprintf("failed to create %s for com%d: %s", path, i+1, err)
			return errors.New(detail)
		}
		log.Debugf("created %s as persistent log destinations", path)
	}

	// make an access to urandom
	path = fmt.Sprintf("%s/urandom", pathPrefix)
	err = os.Symlink("/dev/urandom", path)
	if err != nil {
		detail := fmt.Sprintf("failed to create urandom access: %s", err)
		return errors.New(detail)
	}

	return nil
}

func testSetup(t *testing.T) *Mocker {
	var err error

	pathPrefix, err = ioutil.TempDir("", path.Base(t.Name()))
	if err != nil {
		fmt.Println(err)
		t.Error(err)
	}

	mocker := tetherTestSetup(t)

	err = os.MkdirAll(pathPrefix, 0777)
	if err != nil {
		fmt.Println(err)
		t.Error(err)
	}
	log.Infof("Using %s as test prefix", pathPrefix)

	backchannelMode = os.ModeNamedPipe | os.ModePerm
	err = createFakeDevices()
	if err != nil {
		fmt.Println(err)
		t.Error(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	// supply custom attach server so we can inspect its state
	testServer := &testAttachServer{
		updated: make(chan bool, 10),
		attachServerSSH: attachServerSSH{
			ctx:    ctx,
			cancel: cancel,
		},
	}
	server = testServer

	return mocker
}

func testTeardown(t *testing.T, mocker *Mocker) {
	tetherTestTeardown(t, mocker)
}

type mockery struct {
	reloaded chan bool
}

func (m *mockery) Reload() {
	m.reloaded <- true
}

func (m *mockery) Start() error {
	return nil
}

func (m *mockery) Stop() error {
	return nil
}

func (m *mockery) Register(name string, config tether.Extension) {
}

func (m *mockery) LaunchUtility(fn tether.UtilityFn) (<-chan int, error) {
	return nil, nil
}

// Test reloading via signal helper
func TestReload(t *testing.T) {
	m := &mockery{make(chan bool)}
	tthr = m

	startSignalHandler()

	if !assert.NoError(t, tether.ReloadConfig()) {
		return
	}

	// check the started channel is closed (which gets closed on reconfig)
	if !assert.True(t, <-m.reloaded) {
		return
	}
}
