// Copyright © 2017 The virtual-kubelet authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package providers

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/virtual-kubelet/virtual-kubelet/providers/register"
)

// NewCommand creates a new providers subcommand
// This subcommand is used to determine which providers are registered.
func NewCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "providers",
		Short: "Show the list of supported providers",
		Long:  "Show the list of supported providers",
		Args:  cobra.MaximumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			switch len(args) {
			case 0:
				ls := register.List()
				for _, p := range ls {
					fmt.Fprintln(cmd.OutOrStdout(), p)
				}
			case 1:
				if !register.Exists(args[0]) {
					fmt.Fprintln(cmd.OutOrStderr(), "no such provider", args[0])

					// TODO(@cpuuy83): would be nice to not short-circuit the exit here
					// But at the momemt this seems to be the only way to exit non-zero and
					// handle our own error output
					os.Exit(1)
				}
				fmt.Fprintln(cmd.OutOrStdout(), args[0])
			}
			return
		},
	}
}
