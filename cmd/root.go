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

package cmd

import (
	"github.com/spf13/cobra"
	"log"

	"github.com/virtual-kubelet/virtual-kubelet/cmd/options"
	vkubelet "github.com/virtual-kubelet/virtual-kubelet/vkubelet"
)

func NewVKubeletCommand() *cobra.Command {
	v := options.NewVKubeletRunOptions()
	cmd := &cobra.Command{
		Use:   "virtual-kubelet",
		Short: "virtual-kubelet provides a virtual kubelet interface for your kubernetes cluster.",
		Long: `virtual-kubelet implements the Kubelet interface with a pluggable 
backend implementation allowing users to create kubernetes nodes without running the kubelet.
This allows users to schedule kubernetes workloads on nodes that aren't running Kubernetes.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			options.Validate(v)
			f, err := vkubelet.New(v)
			if err != nil {
				log.Fatal(err)
			}
			return f.Run()
		},
	}
	v.AddFlags(cmd.Flags())

	return cmd
}
