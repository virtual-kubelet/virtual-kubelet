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
	"fmt"
	"io"
	"os"
	"path/filepath"
	runtime "runtime/debug"

	log "github.com/Sirupsen/logrus"
	"gopkg.in/urfave/cli.v1"

	"github.com/vmware/vic/cmd/vic-machine/common"
	"github.com/vmware/vic/cmd/vic-machine/configure"
	"github.com/vmware/vic/cmd/vic-machine/create"
	"github.com/vmware/vic/cmd/vic-machine/debug"
	uninstall "github.com/vmware/vic/cmd/vic-machine/delete"
	"github.com/vmware/vic/cmd/vic-machine/inspect"
	"github.com/vmware/vic/cmd/vic-machine/list"
	"github.com/vmware/vic/cmd/vic-machine/update"
	"github.com/vmware/vic/cmd/vic-machine/upgrade"
	viclog "github.com/vmware/vic/pkg/log"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/version"
)

const (
	LogFile = "vic-machine.log"
)

func main() {
	app := cli.NewApp()

	app.Name = filepath.Base(os.Args[0])
	app.Usage = "Create and manage Virtual Container Hosts"
	app.EnableBashCompletion = true

	create := create.NewCreate()
	uninstall := uninstall.NewUninstall()
	inspect := inspect.NewInspect()
	list := list.NewList()
	upgrade := upgrade.NewUpgrade()
	debug := debug.NewDebug()
	updateFw := update.NewUpdateFw()
	configure := configure.NewConfigure()
	app.Commands = []cli.Command{
		{
			Name:         "create",
			Usage:        "Deploy VCH",
			Action:       create.Run,
			Flags:        create.Flags(),
			BashComplete: common.BashComplete(create.Flags),
		},
		{
			Name:         "configure",
			Usage:        "Update VCH configuration",
			Action:       configure.Run,
			Flags:        configure.Flags(),
			BashComplete: common.BashComplete(configure.Flags),
		},
		{
			Name:         "delete",
			Usage:        "Delete VCH and associated resources",
			Action:       uninstall.Run,
			Flags:        uninstall.Flags(),
			BashComplete: common.BashComplete(uninstall.Flags),
		},
		{
			Name:         "ls",
			Usage:        "List VCHs",
			Action:       list.Run,
			Flags:        list.Flags(),
			BashComplete: common.BashComplete(list.Flags),
		},
		{
			Name:         "inspect",
			Usage:        "Inspect VCH",
			Action:       inspect.Run,
			Flags:        inspect.Flags(),
			BashComplete: common.BashComplete(inspect.Flags, "help", "config"),
			Subcommands: []cli.Command{
				{
					Name:         "config",
					Usage:        "Show VCH configuration options",
					Action:       inspect.RunConfig,
					Flags:        inspect.ConfigFlags(),
					BashComplete: common.BashComplete(inspect.ConfigFlags),
				},
			},
		},
		{
			Name:         "upgrade",
			Usage:        "Upgrade VCH to latest version",
			Action:       upgrade.Run,
			Flags:        upgrade.Flags(),
			BashComplete: common.BashComplete(upgrade.Flags),
		},
		{
			Name:   "version",
			Usage:  "Show VIC version information",
			Action: showVersion,
		},
		{
			Name:         "debug",
			Usage:        "Debug VCH",
			Action:       debug.Run,
			Flags:        debug.Flags(),
			BashComplete: common.BashComplete(debug.Flags),
		},
		{
			Name:         "update",
			Usage:        "Modify infrastructure configuration",
			BashComplete: common.BashComplete(nil, "help", "firewall"),
			Subcommands: []cli.Command{
				{
					Name:         "firewall",
					Usage:        "Modify host firewall",
					Action:       updateFw.Run,
					Flags:        updateFw.Flags(),
					BashComplete: common.BashComplete(updateFw.Flags),
				},
			},
		},
	}

	app.Version = version.GetBuild().ShortVersion()

	logs := []io.Writer{app.Writer}
	// Open log file
	// #nosec: Expect file permissions to be 0600 or less
	f, err := os.OpenFile(LogFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		// #nosec: Errors unhandled.
		fmt.Fprintf(os.Stderr, "Error opening logfile %s: %v\n", LogFile, err)
	} else {
		defer f.Close()
		logs = append(logs, f)
	}

	// Initiliaze logger with default TextFormatter
	log.SetFormatter(viclog.NewTextFormatter())
	// SetOutput to io.MultiWriter so that we can log to stdout and a file
	log.SetOutput(io.MultiWriter(logs...))
	// Set up the global trace logger to write to the file
	trace.Logger.Out = f

	defer func() {
		if r := recover(); r != nil {
			// Output instructions to check log file for details to the user, but not the log file
			logger := log.New()
			logger.Out = app.Writer
			logger.Errorf("--------------------")
			logger.Errorf("%s failed, please check log file %s for details", app.Name, LogFile)

			// Output the stack to the log file, but not the user
			// #nosec: Errors unhandled.
			fmt.Fprintf(f, "%s", runtime.Stack())
		}
	}()

	// When writing log files (vic-machine.log and the datastore log), always log at debug.
	defer func(old log.Level) {
		trace.Logger.Level = old
	}(trace.Logger.Level)
	trace.Logger.Level = log.DebugLevel

	// #nosec: Errors unhandled.
	app.Run(os.Args)
}

func showVersion(cli *cli.Context) error {
	// #nosec: Errors unhandled.
	fmt.Fprintf(cli.App.Writer, "%v version %v\n", cli.App.Name, cli.App.Version)
	return nil
}
