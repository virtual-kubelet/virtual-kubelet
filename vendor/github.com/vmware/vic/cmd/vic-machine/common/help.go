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

package common

import "gopkg.in/urfave/cli.v1"

const entireOptionHelpTemplate = `NAME:
   {{.HelpName}} - {{.Usage}}

USAGE:
   {{.HelpName}}{{if .VisibleFlags}} [command options]{{end}} {{if .ArgsUsage}}{{.ArgsUsage}}{{else}}[arguments...]{{end}}{{if .Category}}

CATEGORY:
   {{.Category}}{{end}}{{if .Description}}

DESCRIPTION:
   {{.Description}}{{end}}{{if .VisibleFlags}}

OPTIONS:
   {{range .Flags}}{{.}}
   {{end}}{{end}}
`

type Help struct {
	advancedOptions bool
}

func (h *Help) HelpFlags() []cli.Flag {
	return []cli.Flag{
		cli.BoolFlag{
			Name:        "extended-help, x",
			Usage:       "Show all options - this must be specified instead of --help",
			Destination: &h.advancedOptions,
		},
	}
}

func (h *Help) Print(clic *cli.Context) bool {
	if h.advancedOptions {
		cli.HelpPrinter(clic.App.Writer, entireOptionHelpTemplate, clic.Command)

		return true
	}

	return false
}
