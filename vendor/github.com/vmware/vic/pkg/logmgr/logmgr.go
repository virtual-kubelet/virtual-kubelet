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

package logmgr

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/vmware/vic/lib/tether"
	"github.com/vmware/vic/pkg/filelock"
	"github.com/vmware/vic/pkg/trace"
)

// RotateInterval defines a type for a log rotate frequency.
type RotateInterval uint32

// LogRotateBinary points to a logrotate path in the system. For testing purposes it should be overwritten to be empty.
var LogRotateBinary = "/usr/sbin/logrotate"

const (
	// Daily to trim logs daily.
	Daily RotateInterval = iota
	// Hourly to trim logs hourly.
	Hourly
	// Weekly to trim logs weekly.
	Weekly
	// Monthly to trim logs monthly.
	Monthly
)

type logRotateConfig struct {
	rotateInterval  RotateInterval
	logFilePath     string
	logFileName     string
	maxLogSizeBytes int64
	maxLogFiles     int64
	compress        bool
}

// ConfigFileContent formats log configuration according to logrotate requirements.
func (lrc *logRotateConfig) ConfigFileContent() string {
	b := make([]string, 0, 32)
	if lrc.compress {
		b = append(b, "compress")
	}

	switch lrc.rotateInterval {
	case Hourly:
		b = append(b, string("hourly"))
	case Daily:
		b = append(b, string("daily"))
	case Weekly:
		b = append(b, string("weekly"))
	case Monthly:
		fallthrough
	default:
		b = append(b, string("monthly"))
	}

	b = append(b, fmt.Sprintf("rotate %d", lrc.maxLogFiles))
	if lrc.maxLogSizeBytes > 0 {
		b = append(b, fmt.Sprintf("size %d", lrc.maxLogSizeBytes))
	}

	if lrc.maxLogSizeBytes > 2 {
		b = append(b, fmt.Sprintf("minsize %d", lrc.maxLogSizeBytes-1))
	}

	// VIC doesn't support HUP yet, thus we are using logrotate copytruncate option that
	// will copy and truncate log file.
	b = append(b, "copytruncate")
	b = append(b, "dateext")
	b = append(b, "dateformat -%Y%m%d-%s")

	for i, v := range b {
		b[i] = "    " + v
	}

	return fmt.Sprintf("%s {\n%s\n}\n", lrc.logFilePath, strings.Join(b, "\n"))
}

// LogManager runs logrotate for specified log files.
// TODO: Upload compressed logs into vSphere storage.
// TODO: Upload all logs into vSphere storage during graceful shutdown.
type LogManager struct {
	// Frequency of running log rotate.
	runInterval time.Duration

	// list of log files and their log rotate parameters to rotate.
	logFiles []*logRotateConfig

	// channel gets closed on stop.
	closed chan struct{}
	op     trace.Operation

	// used to wait until logrotate goroutine stops.
	wg sync.WaitGroup
	// just to make sure start is not called twice accidentally.
	once sync.Once

	logConfig string

	// Mostly for debug purposes to insure log rotate loop is running.
	// It also will log on debug level periodically that it runs.
	loopsCount uint64
}

// NewLogManager creates a new log manager instance.
func NewLogManager(runInterval time.Duration) (*LogManager, error) {
	lm := &LogManager{
		runInterval: runInterval,
		op:          trace.NewOperation(context.Background(), "logrotate"),
		closed:      make(chan struct{}),
	}

	// LogRotateBinary is set to empty during unit testing.
	if s, err := os.Stat(LogRotateBinary); (err != nil || s.IsDir()) && LogRotateBinary != "" {
		return nil, fmt.Errorf("logrotate is not available at %s, without it logs will not be rotated", LogRotateBinary)
	}
	return lm, nil
}

// AddLogRotate adds a log to rotate.
func (lm *LogManager) AddLogRotate(logFilePath string, ri RotateInterval, maxSize, maxLogFiles int64, compress bool) {
	lm.logFiles = append(lm.logFiles, &logRotateConfig{
		rotateInterval:  ri,
		logFilePath:     logFilePath,
		logFileName:     filepath.Base(logFilePath),
		maxLogSizeBytes: maxSize,
		maxLogFiles:     maxLogFiles,
		compress:        compress,
	})
}

// Reload - just to satisfy Tether interface.
func (lm *LogManager) Reload(*tether.ExecutorConfig) error { return nil }

// Start log rotate loop.
func (lm *LogManager) Start(system tether.System) error {
	if len(lm.logFiles) == 0 {
		lm.op.Errorf("Attempt to start logrotate with no log files configured.")
		return nil
	}
	lm.once.Do(func() {
		lm.wg.Add(1)
		lm.logConfig = lm.buildConfig()
		lm.op.Debugf("logrotate config: %s", lm.logConfig)

		go func() {
			for {
				lm.loopsCount++
				if lm.loopsCount%10 == 0 {
					lm.op.Debugf("logrotate has been run %d times", lm.loopsCount)
				}
				select {
				case <-time.After(lm.runInterval):
				case <-lm.closed:
					lm.rotateLogs()
					lm.wg.Done()
					return
				}
				lm.rotateLogs()
			}
		}()
	})
	return nil
}

// Stop loop.
func (lm *LogManager) Stop() error {
	select {
	case <-lm.closed:
	default:
		close(lm.closed)
	}
	lm.wg.Wait()
	return nil
}

func (lm *LogManager) saveConfig(logConf string) string {
	tf, err := ioutil.TempFile("", "vic-logrotate-conf-")
	if err != nil {
		lm.op.Errorf("Failed to create temp file for logrotate: %v", err)
		return ""
	}

	tempFilePath := tf.Name()
	if _, err = tf.Write([]byte(logConf)); err != nil {
		lm.op.Errorf("Failed to store logrotate config %s: %v", tempFilePath, err)
		if err = tf.Close(); err != nil {
			lm.op.Errorf("Failed to close temp file %s: %v", tempFilePath, err)
		}
		if err = os.Remove(tempFilePath); err != nil {
			lm.op.Errorf("Failed to remove temp file %s: %v", tempFilePath, err)
		}
		return ""
	}

	if err = tf.Close(); err != nil {
		lm.op.Errorf("Failed to close logrotate config file %s: %v", tempFilePath, err)
		return ""
	}
	return tempFilePath
}

func (lm *LogManager) buildConfig() string {
	c := make([]string, 0, len(lm.logFiles))
	for _, v := range lm.logFiles {
		c = append(c, v.ConfigFileContent())
	}
	return strings.Join(c, "\n")
}

func (lm *LogManager) rotateLogs() {
	// Check if logrotate config exists, create one
	configFile := lm.saveConfig(lm.logConfig)
	logrotateLock := filelock.NewFileLock(filelock.LogRotateLockName)

	// This lock is necessary to avoid race condition when user requests log bundle.
	if err := logrotateLock.Acquire(); err != nil {
		lm.op.Errorf("Failed to acquire logrotate lock: %v", err)
	} else {
		defer func() { logrotateLock.Release() }()
	}

	if configFile == "" {
		lm.op.Errorf("Can not run logrotate dues to missing logrotate config")
		return
	}
	// remove config file as soon as logrotate finishes its work.
	defer os.Remove(configFile)

	lm.op.Debugf("Running logrotate: %s %s", LogRotateBinary, configFile)

	if LogRotateBinary == "" {
		lm.op.Debugf("logrotate is not defined. Skipping.")
		return
	}
	// #nosec: Subprocess launching with variable
	output, err := exec.Command(LogRotateBinary, configFile).CombinedOutput()
	if err == nil {
		if len(output) > 0 {
			lm.op.Debugf("Logrotate output: %s", output)
		}
		lm.op.Debugf("logrotate finished successfully")
	} else {
		lm.op.Errorf("logrotate exited with non 0 status: %v", err)
		if len(output) > 0 {
			lm.op.Errorf("Logrotate output: %s", output)
		}
	}
}
