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
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"syscall"
	"time"

	log "github.com/Sirupsen/logrus"

	"github.com/vmware/vic/lib/pprof"
	"github.com/vmware/vic/lib/tether"
	"github.com/vmware/vic/pkg/dio"
	"github.com/vmware/vic/pkg/trace"
)

// pathPrefix is present to allow the various files referenced by tether to be placed
// in specific directories, primarily for testing.
var pathPrefix string

const (
	logDir  = "var/log/vic"
	initLog = "init.log"
)

type operations struct {
	tether.BaseOperations
}

func (t *operations) Setup(sink tether.Config) error {
	if err := t.BaseOperations.Setup(sink); err != nil {
		return err
	}

	return pprof.StartPprof("vch-init", pprof.VCHInitPort)
}

func (t *operations) Cleanup() error {
	return t.BaseOperations.Cleanup()
}

// HandleSessionExit controls the behaviour on session exit - for the tether if the session exiting
// is the primary session (i.e. SessionID matches ExecutorID) then we exit everything.
func (t *operations) HandleSessionExit(config *tether.ExecutorConfig, session *tether.SessionConfig) func() {
	defer trace.End(trace.Begin(""))

	// trigger a reload to force relaunch
	return func() {
		// If executor debug is greater than 1 then suppress the relaunch but leave the executor up
		// for diagnostics
		if config.DebugLevel > 2 {
			log.Warnf("Debug is set to %d so squashing relaunch of exited process", config.DebugLevel)
			return
		}

		// incredibly basic throttle
		time.Sleep(3 * time.Second)

		tthr.Reload()
		log.Info("Triggered reload")
	}
}

func (t *operations) SetHostname(name string, aliases ...string) error {
	// switch the names around so we get the pretty name and not the ID
	return t.BaseOperations.SetHostname(aliases[0])
}

func (t *operations) Apply(endpoint *tether.NetworkEndpoint) error {
	return t.BaseOperations.Apply(endpoint)
}

// Log will redirect logging to both a file on disk and to stdout for the process
func (t *operations) Log() (io.Writer, error) {
	defer trace.End(trace.Begin("operations.Log"))

	// make the logging directory
	// #nosec: Expect directory permissions to be 0700 or less
	os.MkdirAll(path.Join(pathPrefix, logDir), 0755)

	logPath := path.Join(pathPrefix, logDir, initLog)

	log.Infof("opening %s for debug log", logPath)
	// #nosec: Expect file permissions to be 0600 or less
	out, err := os.OpenFile(logPath, os.O_RDWR|os.O_CREATE|os.O_APPEND|os.O_SYNC|syscall.O_NOCTTY, 0644)
	if err != nil {
		detail := fmt.Sprintf("failed to open file port for debug log: %s", err)
		log.Error(detail)
		return nil, errors.New(detail)
	}

	return io.MultiWriter(out, os.Stdout), nil
}

// sessionLogWriter returns a writer that will persist the session output
func (t *operations) SessionLog(session *tether.SessionConfig) (dio.DynamicMultiWriter, dio.DynamicMultiWriter, error) {
	defer trace.End(trace.Begin("configure session log writer"))

	name := session.ID
	if name == "" {
		name = session.Name
	}

	logPath := path.Join(pathPrefix, logDir, name+".log")

	// open SttyS2 for session logging
	log.Infof("opening %s for session logging", logPath)
	// #nosec: Expect file permissions to be 0600 or less
	f, err := os.OpenFile(logPath, os.O_RDWR|os.O_CREATE|os.O_APPEND|os.O_SYNC|syscall.O_NOCTTY, 0644)
	if err != nil {
		detail := fmt.Sprintf("failed to open file for session log: %s", err)
		log.Error(detail)
		return nil, nil, errors.New(detail)
	}

	// use multi-writer so it goes to both screen and session log
	if debugLevel > 0 {
		return dio.MultiWriter(f, os.Stdout), dio.MultiWriter(f, os.Stderr), nil
	}

	// only duplicate stderr
	return dio.MultiWriter(f), dio.MultiWriter(f), nil
}
