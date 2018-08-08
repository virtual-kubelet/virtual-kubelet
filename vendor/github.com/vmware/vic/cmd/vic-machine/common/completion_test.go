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

import (
	"bufio"
	"bytes"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gopkg.in/urfave/cli.v1"
)

type command struct {
	t       *testing.T
	flip    bool
	format  string
	things  cli.StringSlice
	timeout time.Duration
}

func (c *command) run(clic *cli.Context) (err error) {
	assert.Fail(c.t, "We shouldn't be running the command during completion")

	return nil
}

func (c *command) flags() []cli.Flag {
	flags := []cli.Flag{
		cli.BoolFlag{
			Name:        "flip, f",
			Destination: &c.flip,
		},
		cli.StringFlag{
			Name:        "format",
			Value:       "verbose",
			Destination: &c.format,
		},
		cli.DurationFlag{
			Name:        "timeout, t",
			Value:       3 * time.Minute,
			Destination: &c.timeout,
		},
	}

	return flags
}

func (c *command) configFlags() []cli.Flag {
	flags := []cli.Flag{
		cli.BoolTFlag{
			Name:        "switch, s",
			Destination: &c.flip,
		},
		cli.StringSliceFlag{
			Name:  "things",
			Value: &c.things,
		},
		cli.DurationFlag{
			Name:        "timeout, t",
			Value:       3 * time.Minute,
			Destination: &c.timeout,
		},
	}

	return flags
}

var tests = []struct {
	in  []string
	out []string
}{
	{
		in:  []string{""},
		out: []string{"command", "h", "help"},
	},
	{
		in:  []string{"command"},
		out: []string{"config", "-f", "--flip", "--format", "-t", "--timeout"},
	},
	{
		in:  []string{"command", "--flip"},
		out: []string{"--format", "-t", "--timeout"},
	},
	{
		in:  []string{"command", "--format"},
		out: []string{""},
	},
	{
		in:  []string{"command", "--format", "foo"},
		out: []string{"-f", "--flip", "-t", "--timeout"},
	},
	{
		in:  []string{"command", "--format", "foo", "-f"},
		out: []string{"-t", "--timeout"},
	},
	{
		in:  []string{"command", "--format", "foo", "-f", "false"},
		out: []string{"-t", "--timeout"},
	},
	{
		in:  []string{"command", "--format", "foo", "-t"},
		out: []string{""},
	},
	{
		in:  []string{"command", "--format", "foo", "--timeout", "3m"},
		out: []string{"-f", "--flip"},
	},
	{
		in:  []string{"command", "--format", "foo", "-t", "3m", "-f"},
		out: []string{""},
	},
	{
		in:  []string{"command", "config"},
		out: []string{"-s", "--switch", "--things", "-t", "--timeout"},
	},
	{
		in:  []string{"command", "config", "-t"},
		out: []string{""},
	},
	{
		in:  []string{"command", "config", "--things", "foo"},
		out: []string{"-s", "--switch", "--things", "-t", "--timeout"},
	},
	{
		in:  []string{"command", "config", "--things", "foo", "--things", "bar", "--things", "baz"},
		out: []string{"-s", "--switch", "--things", "-t", "--timeout"},
	},
	{
		in:  []string{"command", "--bad"},
		out: []string{"config", "-f", "--flip", "--format", "-t", "--timeout"},
	},
}

func TestBashComplete(t *testing.T) {
	app := cli.NewApp()
	app.Name = "vic-machine"
	app.EnableBashCompletion = true

	buf := &bytes.Buffer{}
	w := bufio.NewWriter(buf)
	app.Writer = w

	c := command{}
	app.Commands = []cli.Command{
		{
			Name:         "command",
			Action:       c.run,
			Flags:        c.flags(),
			BashComplete: BashComplete(c.flags, "config"),
			Subcommands: []cli.Command{
				{
					Name:         "config",
					Action:       c.run,
					Flags:        c.configFlags(),
					BashComplete: BashComplete(c.configFlags),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(strings.Join(tt.in, " "), func(t *testing.T) {
			defer func(oldArgs []string) { os.Args = oldArgs }(os.Args)
			os.Args = append(append([]string{app.Name}, tt.in...), "--"+cli.BashCompletionFlag.Name)

			app.Run(os.Args)

			w.Flush()
			s := buf.String()
			buf.Reset()

			ss := strings.Split(strings.Trim(s, "\n"), "\n")
			sort.Strings(ss)
			sort.Strings(tt.out)

			assert.Equal(t, tt.out, ss)
		})
	}
}
