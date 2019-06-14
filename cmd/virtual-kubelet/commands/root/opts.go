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

package root

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
	DisableTaint bool

	MetricsAddr string

	// Number of workers to use to handle pod notifications
	PodSyncWorkers       int
	InformerResyncPeriod time.Duration

	// Use node leases when supported by Kubernetes (instead of node status updates)
	EnableNodeLease bool

	TraceExporters  []string
	TraceSampleRate string
	TraceConfig     TracingExporterOptions

	// Startup Timeout is how long to wait for the kubelet to start
	StartupTimeout time.Duration

	Version string
}

// SetDefaultOpts sets default options for unset values on the passed in option struct.
// Fields tht are already set will not be modified.
func SetDefaultOpts(c *Opts) error {
	if c.OperatingSystem == "" {
		c.OperatingSystem = DefaultOperatingSystem
	}

	if c.NodeName == "" {
		c.NodeName = getEnv("DEFAULT_NODE_NAME", DefaultNodeName)
	}

	if c.InformerResyncPeriod == 0 {
		c.InformerResyncPeriod = DefaultInformerResyncPeriod
	}

	if c.MetricsAddr == "" {
		c.MetricsAddr = DefaultMetricsAddr
	}

	if c.PodSyncWorkers == 0 {
		c.PodSyncWorkers = DefaultPodSyncWorkers
	}

	if c.TraceConfig.ServiceName == "" {
		c.TraceConfig.ServiceName = DefaultNodeName
	}

	if c.ListenPort == 0 {
		if kp := os.Getenv("KUBELET_PORT"); kp != "" {
			p, err := strconv.Atoi(kp)
			if err != nil {
				return errors.Wrap(err, "error parsing KUBELET_PORT environment variable")
			}
			c.ListenPort = int32(p)
		} else {
			c.ListenPort = DefaultListenPort
		}
	}

	if c.KubeNamespace == "" {
		c.KubeNamespace = DefaultKubeNamespace
	}

	if c.TaintKey == "" {
		c.TaintKey = DefaultTaintKey
	}
	if c.TaintEffect == "" {
		c.TaintEffect = DefaultTaintEffect
	}

	if c.KubeConfigPath == "" {
		c.KubeConfigPath = os.Getenv("KUBECONFIG")
		if c.KubeConfigPath == "" {
			home, _ := homedir.Dir()
			if home != "" {
				c.KubeConfigPath = filepath.Join(home, ".kube", "config")
			}
		}
	}

	return nil
}
