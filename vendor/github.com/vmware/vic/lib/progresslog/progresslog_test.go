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

package progresslog

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/vmware/govmomi/vim25/progress"
)

type ProgressResults struct {
	percentage float32
}

func (pr *ProgressResults) Percentage() float32 {
	return pr.percentage
}

func (pr *ProgressResults) Detail() string {
	return ""
}

func (pr *ProgressResults) Error() error {
	return nil
}

var _ progress.Report = &ProgressResults{}

func TestNewUploadLoggerComplete(t *testing.T) {
	var logs []string
	logTo := func(format string, args ...interface{}) {
		logs = append(logs, fmt.Sprintf(format, args...))
	}
	pl := NewUploadLogger(logTo, "unittest", time.Millisecond*10)
	progressChan := pl.Sink()
	for i := 0; i <= 10; i++ {
		res := &ProgressResults{percentage: float32(i * 10)}
		progressChan <- res
		time.Sleep(time.Duration(time.Millisecond * 5))
	}
	close(progressChan)
	pl.Wait()

	if assert.True(t, len(logs) > 3) {
		last := len(logs) - 1
		assert.Contains(t, logs[0], "unittest")
		assert.Contains(t, logs[0], "0.00%")
		assert.Contains(t, logs[1], ".00%")
		assert.Contains(t, logs[last-1], "100.00%")
		assert.Contains(t, logs[last], "complete")
	}
}

func TestNewUploadLoggerNotComplete(t *testing.T) {
	var logs []string
	logTo := func(format string, args ...interface{}) {
		logs = append(logs, fmt.Sprintf(format, args...))
	}
	pl := NewUploadLogger(logTo, "unittest", time.Millisecond*10)
	progressChan := pl.Sink()
	for i := 0; i < 10; i++ {
		res := &ProgressResults{percentage: float32(i * 10)}
		progressChan <- res
		time.Sleep(time.Duration(time.Millisecond * 5))
	}
	close(progressChan)
	pl.Wait()

	if assert.True(t, len(logs) > 3) {
		last := len(logs) - 1
		assert.Contains(t, logs[0], "unittest")
		assert.Contains(t, logs[0], "0.00%")
		assert.Contains(t, logs[1], ".00%")
		assert.NotContains(t, logs[last], "100.00%")
		assert.NotContains(t, logs[last], "complete")
	}
}
