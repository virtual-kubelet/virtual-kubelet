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

package logmgr

import (
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestContentIsGeneratedAndSaved tests:
// 1. Config file content for log rotate is as expected.
// 2. Config file is saved on disk and can be read back.
func TestContentIsGeneratedAndSaved(t *testing.T) {
	oldLogRotate := LogRotateBinary
	LogRotateBinary = ""
	defer func() { LogRotateBinary = oldLogRotate }()

	m, e := NewLogManager(time.Millisecond)
	if !assert.Nil(t, e) {
		return
	}
	expectedConfig := `/var/log/test.log {
    compress
    hourly
    rotate 3
    size 10000
    minsize 9999
    copytruncate
    dateext
    dateformat -%Y%m%d-%s
}

/var/log/test1.log {
    daily
    rotate 2
    size 20000
    minsize 19999
    copytruncate
    dateext
    dateformat -%Y%m%d-%s
}
`
	m.AddLogRotate("/var/log/test.log", Hourly, 10000, 3, true)
	m.AddLogRotate("/var/log/test1.log", Daily, 20000, 2, false)
	actualConfig := m.buildConfig()
	assert.Equal(t, expectedConfig, actualConfig)

	fn := m.saveConfig(expectedConfig)
	if !assert.NotEqual(t, "", fn) {
		return
	}
	// delete temp file when test is done.
	defer func() {
		assert.Nil(t, os.Remove(fn))
	}()
	data, err := ioutil.ReadFile(fn)
	if !assert.Nil(t, err) {
		return
	}
	assert.Equal(t, expectedConfig, string(data))
}

// TestStartAndStop starts and stop the service checking that start/stop conditions are met.
func TestStartAndStop(t *testing.T) {
	oldLogRotate := LogRotateBinary
	LogRotateBinary = ""
	defer func() { LogRotateBinary = oldLogRotate }()

	m, e := NewLogManager(time.Millisecond)
	if !assert.Nil(t, e) {
		return
	}
	m.AddLogRotate("/var/log/test.log", Hourly, 10000, 3, true)
	m.AddLogRotate("/var/log/test1.log", Daily, 20000, 2, false)

	go func() {
		assert.Nil(t, m.Start(nil))
		time.Sleep(time.Millisecond * 100)
		assert.Nil(t, m.Stop())
	}()

	select {
	case <-time.After(time.Second * 10):
		assert.Fail(t, "timeout to start/stop log manager")
	case _, ok := <-m.closed:
		assert.False(t, ok)
	}
	assert.True(t, m.loopsCount > 0)
}
