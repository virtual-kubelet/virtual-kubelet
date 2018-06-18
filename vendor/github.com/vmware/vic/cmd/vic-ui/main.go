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

	log "github.com/Sirupsen/logrus"
	"gopkg.in/urfave/cli.v1"

	"github.com/vmware/vic/cmd/vic-ui/ui"
	"github.com/vmware/vic/pkg/errors"
	viclog "github.com/vmware/vic/pkg/log"
)

var (
	Version  string
	BuildID  string
	CommitID string
)

const (
	LogFile = "vic-ui.log"
)

func main() {
	app := cli.NewApp()

	app.Name = filepath.Base(os.Args[0])
	app.Usage = "Install/remove VIC UI plugin"
	app.EnableBashCompletion = true

	ui := ui.NewUI()
	app.Commands = []cli.Command{
		{
			Name:   "install",
			Usage:  "Install UI plugin",
			Action: ui.Install,
			Flags:  ui.Flags(),
		},
		{
			Name:   "remove",
			Usage:  "Remove UI plugin",
			Action: ui.Remove,
			Flags:  ui.Flags(),
		},
		{
			Name:   "info",
			Usage:  "Show UI plugin information",
			Action: ui.Info,
			Flags:  ui.InfoFlags(),
		},
		{
			Name:   "version",
			Usage:  "Show VIC version information",
			Action: showVersion,
		},
	}
	if Version != "" {
		app.Version = fmt.Sprintf("%s-%s-%s", Version, BuildID, CommitID)
	} else {
		app.Version = fmt.Sprintf("%s-%s", BuildID, CommitID)
	}

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

	if err := app.Run(os.Args); err != nil {
		log.Errorf("--------------------")
		log.Errorf("%s failed: %s\n", app.Name, errors.ErrorStack(err))
		os.Exit(1)
	}
}

func showVersion(cli *cli.Context) error {
	// #nosec: Errors unhandled.
	fmt.Fprintf(cli.App.Writer, "%v version %v\n", cli.App.Name, cli.App.Version)
	return nil
}
