// Copyright © 2018 The virtual-kubelet authors
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

package options

import (
	"fmt"
	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/pflag"
	"os"
	"path/filepath"
	"strings"

	"github.com/virtual-kubelet/virtual-kubelet/providers"
	corev1 "k8s.io/api/core/v1"
)

type VKubeletRunOptions struct {
	KubeletConfig   string
	KubeConfig      string
	KubeNamespace   string
	NodeName        string
	OperatingSystem string
	Provider        string
	ProviderConfig  string
	Taint           string
}

func (v *VKubeletRunOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&v.KubeConfig, "kubeconfig", "", "config file (default is $HOME/.kube/config)")
	fs.StringVar(&v.KubeNamespace, "namespace", "", "kubernetes namespace (default is 'all')")
	fs.StringVar(&v.NodeName, "nodename", "virtual-kubelet", "kubernetes node name")
	fs.StringVar(&v.OperatingSystem, "os", "Linux", "Operating System (Linux/Windows)")
	fs.StringVar(&v.Provider, "provider", "", "cloud provider")
	fs.StringVar(&v.Taint, "taint", "", "apply taint to node, making scheduling explicit")
	fs.StringVar(&v.ProviderConfig, "provider-config", "", "cloud provider configuration file")
}

func NewVKubeletRunOptions() *VKubeletRunOptions {
	// Find home directory.
	home, err := homedir.Dir()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	v := &VKubeletRunOptions{
		KubeConfig:    filepath.Join(home, ".kube", "config"),
		KubeNamespace: corev1.NamespaceAll,
		//TODO: add default value to other fields
	}
	return v
}

func Validate(v *VKubeletRunOptions) {
	if v.Provider == "" {
		fmt.Println("You must supply a cloud provider option: use --provider")
		os.Exit(1)
	}
	// Validate operating system.
	ok, _ := providers.ValidOperatingSystems[v.OperatingSystem]
	if !ok {
		fmt.Printf("Operating system '%s' not supported. Valid options are %s\n", v.OperatingSystem, strings.Join(providers.ValidOperatingSystems.Names(), " | "))
		os.Exit(1)
	}
}