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

package opts

import (
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/mitchellh/go-homedir"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
)

// Defaults for root command options
const (
	DefaultNodeName             = "virtual-kubelet"
	DefaultOperatingSystem      = "Linux"
	DefaultInformerResyncPeriod = 1 * time.Minute
	DefaultMetricsAddr          = ":10255"
	DefaultListenPort           = 10250 // TODO(cpuguy83)(VK1.0): Change this to an addr instead of just a port.. we should not be listening on all interfaces.
	DefaultPodSyncWorkers       = 10
	DefaultKubeNamespace        = corev1.NamespaceAll
	DefaultKubeClusterDomain    = "cluster.local"

	DefaultTaintEffect = string(corev1.TaintEffectNoSchedule)
	DefaultTaintKey    = "virtual-kubelet.io/provider"
)

// Opts stores all the options for configuring the root virtual-kubelet command.
// It is used for setting flag values.
//
// You can set the default options by creating a new `Opts` struct and passing
// it into `SetDefaultOpts`
type Opts struct {
	// Path to the kubeconfig to use to connect to the Kubernetes API server.
	KubeConfigPath string
	// Namespace to watch for pods and other resources
	KubeNamespace string
	// Domain suffix to append to search domains for the pods created by virtual-kubelet
	KubeClusterDomain string

	// Sets the port to listen for requests from the Kubernetes API server
	ListenPort int32

	// Node name to use when creating a node in Kubernetes
	NodeName string

	// Operating system to run pods for
	OperatingSystem string

	Provider           string
	ProviderConfigPath string

	TaintKey     string
	TaintEffect  string
	TaintValue   string
	DisableTaint bool

	MetricsAddr string

	// Number of workers to use to handle pod notifications
	PodSyncWorkers       int
	InformerResyncPeriod time.Duration

	// Use node leases when supported by Kubernetes (instead of node status updates)
	EnableNodeLease bool

	// Startup Timeout is how long to wait for the kubelet to start
	StartupTimeout time.Duration

	Version string
}

// FromEnv sets default options for unset values on the passed in option struct.
// Fields tht are already set will not be modified.
func FromEnv() (*Opts, error) {
	o := &Opts{}
	setDefaults(o)

	o.NodeName = getEnv("DEFAULTNODE_NAME", o.NodeName)

	if kp := os.Getenv("KUBELET_PORT"); kp != "" {
		p, err := strconv.Atoi(kp)
		if err != nil {
			return o, errors.Wrap(err, "error parsing KUBELET_PORT environment variable")
		}
		o.ListenPort = int32(p)
	}

	o.KubeConfigPath = os.Getenv("KUBECONFIG")
	if o.KubeConfigPath == "" {
		home, _ := homedir.Dir()
		if home != "" {
			o.KubeConfigPath = filepath.Join(home, ".kube", "config")
		}
	}

	o.TaintKey = getEnv("VKUBELET_TAINT_KEY", o.TaintKey)
	o.TaintValue = getEnv("VKUBELET_TAINT_VALUE", o.TaintValue)
	o.TaintEffect = getEnv("VKUBELET_TAINT_EFFECT", o.TaintEffect)

	return o, nil
}

func New() *Opts {
	o := &Opts{}
	setDefaults(o)
	return o
}

func setDefaults(o *Opts) {
	o.OperatingSystem = DefaultOperatingSystem
	o.NodeName = DefaultNodeName
	o.TaintKey = DefaultTaintKey
	o.TaintEffect = DefaultTaintEffect
	o.KubeNamespace = DefaultKubeNamespace
	o.PodSyncWorkers = DefaultPodSyncWorkers
	o.ListenPort = DefaultListenPort
	o.MetricsAddr = DefaultMetricsAddr
	o.InformerResyncPeriod = DefaultInformerResyncPeriod
	o.KubeClusterDomain = DefaultKubeClusterDomain
}

func getEnv(key, defaultValue string) string {
	value, found := os.LookupEnv(key)
	if found {
		return value
	}
	return defaultValue
}
