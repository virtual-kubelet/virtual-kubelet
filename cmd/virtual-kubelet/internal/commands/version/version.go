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

package version

import (
	"fmt"

	"github.com/spf13/cobra"
)

// NewCommand creates a new version subcommand command
func NewCommand(version, buildTime string) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show the version of the program",
		Long:  `Show the version of the program`,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Version: %s, Built: %s\n", version, buildTime)
		},
	}
}
