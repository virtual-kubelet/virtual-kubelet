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
	"fmt"
	"os"
	"strings"

	"gopkg.in/urfave/cli.v1"
)

// getNames extracts one or more names from a cli.Flag object, trimming whitespace from each.
func getNames(flag cli.Flag) []string {
	names := strings.Split(flag.GetName(), ",")

	trimmedNames := make([]string, 0, len(names))
	for _, name := range names {
		trimmedNames = append(trimmedNames, strings.Trim(name, " "))
	}

	return trimmedNames
}

// findFlag searches a haystack of cli.Flag objects for the first with a name matching the needle.
func findFlag(needle string, haystack []cli.Flag) cli.Flag {
	for _, f := range haystack {
		for _, n := range getNames(f) {
			if needle == n {
				return f
			}
		}
	}

	return nil
}

// BashComplete returns a cli.BashCompleteFunc for a cli.Command given its Flags function and subcommand names.
func BashComplete(flagsFunc func() []cli.Flag, subcommands ...string) cli.BashCompleteFunc {
	return func(c *cli.Context) {
		var flags []cli.Flag
		if flagsFunc != nil {
			flags = flagsFunc()
		}

		// Because cli.Context doesn't provide a good way to see what argument-value pair the user
		// might have "in progress" when asking for a completion, we need to examine the arguments
		// that they've entered.
		//
		// The last argument is going to be `cli.BashCompletionFlag.Name`, as that's what triggers
		// this logic, so the last argument the user supplied will the preceding one.
		//
		// If we recognize that argument, we can adjust our completion recommendations.
		if len(os.Args) > 2 {
			lastArg := os.Args[len(os.Args)-2]

			// If the argument begins with a hypen, we *guess* that it's probably a flag.
			if strings.HasPrefix(lastArg, "-") {
				lastFlag := findFlag(strings.Trim(lastArg, "-"), flags)
				switch lastFlag.(type) {
				case nil:
					// The last value wasn't a flag we recognize.
					break
				case cli.BoolFlag, cli.BoolTFlag:
					// The last flag was a boolean flag; it doesn't need a value.
					break
				default:
					// The last thing the user entered was a flag which takes a value (without the
					// value), so we return without suggesting anything since they need to supply
					// the value on their own.
					return
				}
			}
		}

		// If the user isn't in the middle of entering an argument-value pair, we'll suggest all
		// of the arguments that they haven't yet supplied (unless that argument can be specified
		// multiple times).
		for _, f := range flags {
			names := getNames(f)
			for _, n := range names {
				if c.IsSet(n) {
					switch f.(type) {
					case cli.Int64SliceFlag, cli.IntSliceFlag, cli.StringSliceFlag:
						// The last flag was a slice flag; it can be specified more than once.
						break
					default:
						continue
					}
				}

				if len(n) == 1 {
					fmt.Fprintln(c.App.Writer, "-"+n)
				} else {
					fmt.Fprintln(c.App.Writer, "--"+n)
				}
			}
		}

		// Currently we require any subcommand to be listed first, so we won't suggest a subcommand
		// once the user has specified an argument or flag (a call with arguments between the main
		// command and subcommand, like `inspect --target ... config ...` is unsupported).
		if c.NArg() == 0 && c.NumFlags() == 0 {
			for _, s := range subcommands {
				fmt.Fprintln(c.App.Writer, s)
			}
		}
	}
}
