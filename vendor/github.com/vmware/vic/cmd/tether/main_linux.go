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
	"os"
	"os/signal"
	"path"
	"runtime/debug"
	"strings"
	"syscall"

	log "github.com/Sirupsen/logrus"

	"github.com/vmware/vic/lib/tether"
	viclog "github.com/vmware/vic/pkg/log"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/vsphere/extraconfig"
)

var tthr tether.Tether

func init() {
	// Initiliaze logger with default TextFormatter
	log.SetFormatter(viclog.NewTextFormatter())

	// use the same logger for trace and other logging
	trace.Logger.Level = log.DebugLevel
	log.SetLevel(log.DebugLevel)

	// init and start the HUP handler
	startSignalHandler()

	pathPrefix = "/dev"
}

func main() {
	if strings.HasSuffix(os.Args[0], "-debug") {
		// very, very verbose
		extraconfig.SetLogLevel(log.DebugLevel)
	}

	defer func() {
		if r := recover(); r != nil {
			log.Errorf("run time panic: %s : %s", r, debug.Stack())
		}
		halt()
	}()

	logFile, err := os.OpenFile(path.Join(pathPrefix, "ttyS1"), os.O_WRONLY|os.O_SYNC, 0)
	if err != nil {
		log.Errorf("Could not open serial port for debugging info. Some debug info may be lost! Error reported was %s", err)
	}

	if err = syscall.Dup3(int(logFile.Fd()), int(os.Stderr.Fd()), 0); err != nil {
		log.Errorf("Could not pipe logfile to standard error due to error %s", err)
	}

	if _, err = os.Stderr.WriteString("all stderr redirected to debug log"); err != nil {
		log.Errorf("Could not write to Stderr due to error %s", err)
	}

	// where to look for the various devices and files related to tether

	// TODO: hard code executor initialization status reporting via guestinfo here
	sshserver := NewAttachServerSSH()
	src, err := extraconfig.GuestInfoSource()
	if err != nil {
		log.Error(err)
		return
	}

	sink, err := extraconfig.GuestInfoSink()
	if err != nil {
		log.Error(err)
		return
	}

	// create the tether
	tthr = tether.New(src, sink, &operations{})

	// register the attach extension
	tthr.Register("Attach", sshserver)

	// register the toolbox extension
	tthr.Register("Toolbox", tether.NewToolbox().InContainer())

	// register the executor extension
	tthr.Register("Haveged", NewHaveged())

	err = tthr.Start()
	if err != nil {
		log.Error(err)
		return
	}

	log.Info("Clean exit from tether")
}

// exit cleanly shuts down the system
func halt() {
	log.Infof("Powering off the system")
	if strings.HasSuffix(os.Args[0], "-debug") {
		log.Info("Squashing power off for debug tether")
		return
	}

	syscall.Sync()
	syscall.Reboot(syscall.LINUX_REBOOT_CMD_POWER_OFF)
}

func startSignalHandler() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGHUP)

	go func() {
		for s := range sigs {
			switch s {
			case syscall.SIGHUP:
				log.Infof("Reloading tether configuration")
				tthr.Reload()
			default:
				log.Infof("%s signal not defined", s.String())
			}
		}
	}()
}
