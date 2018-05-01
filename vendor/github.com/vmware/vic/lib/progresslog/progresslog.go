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
	"sync"
	"time"

	"github.com/vmware/govmomi/vim25/progress"
	"github.com/vmware/govmomi/vim25/soap"
)

// UploadParams uses default upload settings as initial input and set UploadLogger as a logger.
func UploadParams(ul *UploadLogger) *soap.Upload {
	params := soap.DefaultUpload
	params.Progress = ul
	return &params
}

// UploadLogger is used to track upload progress to ESXi/VC of a specific file.
type UploadLogger struct {
	wg       sync.WaitGroup
	filename string
	interval time.Duration
	logTo    func(format string, args ...interface{})
}

// NewUploadLogger returns a new instance of UploadLogger. User can provide a logger interface
// that will be used to dump output of the current upload status.
func NewUploadLogger(logTo func(format string, args ...interface{}),
	filename string, progressInterval time.Duration) *UploadLogger {

	return &UploadLogger{
		logTo:    logTo,
		filename: filename,
		interval: progressInterval,
	}
}

// Sink returns a channel that receives current upload progress status.
// Channel has to be closed by the caller.
func (ul *UploadLogger) Sink() chan<- progress.Report {
	ul.wg.Add(1)
	ch := make(chan progress.Report)
	fmtStr := "Uploading %s. Progress: %.2f%%"

	go func() {
		var curProgress float32
		var lastProgress float32
		ul.logTo(fmtStr, ul.filename, curProgress)

		mu := sync.Mutex{}
		ticker := time.NewTicker(ul.interval)

		// Print progress every ul.interval seconds.
		go func() {
			for range ticker.C {
				mu.Lock()
				lastProgress = curProgress
				mu.Unlock()
				ul.logTo(fmtStr, ul.filename, lastProgress)
			}
		}()

		for v := range ch {
			mu.Lock()
			curProgress = v.Percentage()
			mu.Unlock()
		}

		ticker.Stop()

		if lastProgress != curProgress {
			ul.logTo(fmtStr, ul.filename, curProgress)
		}

		if curProgress == 100.0 {
			ul.logTo("Uploading of %s has been completed", ul.filename)
		}
		ul.wg.Done()
	}()
	return ch
}

// Wait waits for Sink to complete its work, due to its async nature logging messages may be not sequential.
func (ul *UploadLogger) Wait() {
	ul.wg.Wait()
}
