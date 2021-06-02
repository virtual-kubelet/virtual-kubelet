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
	"context"
	"os"
	"runtime"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/virtual-kubelet/virtual-kubelet/cmd/virtual-kubelet/internal/provider"
	"github.com/virtual-kubelet/virtual-kubelet/errdefs"
	"github.com/virtual-kubelet/virtual-kubelet/internal/manager"
	"github.com/virtual-kubelet/virtual-kubelet/log"
	"github.com/virtual-kubelet/virtual-kubelet/node"
	"github.com/virtual-kubelet/virtual-kubelet/node/nodeutil"
	corev1 "k8s.io/api/core/v1"
)

// NewCommand creates a new top-level command.
// This command is used to start the virtual-kubelet daemon
func NewCommand(ctx context.Context, name string, s *provider.Store, c Opts) *cobra.Command {
	cmd := &cobra.Command{
		Use:   name,
		Short: name + " provides a virtual kubelet interface for your kubernetes cluster.",
		Long: name + ` implements the Kubelet interface with a pluggable
backend implementation allowing users to create kubernetes nodes without running the kubelet.
This allows users to schedule kubernetes workloads on nodes that aren't running Kubernetes.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRootCommand(ctx, s, c)
		},
	}

	installFlags(cmd.Flags(), &c)
	return cmd
}

func runRootCommand(ctx context.Context, s *provider.Store, c Opts) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if ok := provider.ValidOperatingSystems[c.OperatingSystem]; !ok {
		return errdefs.InvalidInputf("operating system %q is not supported", c.OperatingSystem)
	}

	if c.PodSyncWorkers == 0 {
		return errdefs.InvalidInput("pod sync workers must be greater than 0")
	}

	var taint *corev1.Taint
	if !c.DisableTaint {
		var err error
		taint, err = getTaint(c)
		if err != nil {
			return err
		}
	}

	client, err := nodeutil.ClientsetFromEnv(c.KubeConfigPath)
	if err != nil {
		return err
	}

	cancelHTTP := func() {}
	defer func() {
		// note: this is purposefully using a closure so that when this is actually set the correct function will be called
		if cancelHTTP != nil {
			cancelHTTP()
		}
	}()
	newProvider := func(cfg nodeutil.ProviderConfig) (node.PodLifecycleHandler, node.NodeProvider, error) {
		var err error
		rm, err := manager.NewResourceManager(cfg.Pods, cfg.Secrets, cfg.ConfigMaps, cfg.Services)
		if err != nil {
			return nil, nil, errors.Wrap(err, "could not create resource manager")
		}
		initConfig := provider.InitConfig{
			ConfigPath:        c.ProviderConfigPath,
			NodeName:          c.NodeName,
			OperatingSystem:   c.OperatingSystem,
			ResourceManager:   rm,
			DaemonPort:        c.ListenPort,
			InternalIP:        os.Getenv("VKUBELET_POD_IP"),
			KubeClusterDomain: c.KubeClusterDomain,
		}
		pInit := s.Get(c.Provider)
		if pInit == nil {
			return nil, nil, errors.Errorf("provider %q not found", c.Provider)
		}

		p, err := pInit(initConfig)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "error initializing provider %s", c.Provider)
		}
		p.ConfigureNode(ctx, cfg.Node)

		apiConfig, err := getAPIConfig(c)
		if err != nil {
			return nil, nil, err
		}

		cancelHTTP, err = setupHTTPServer(ctx, p, apiConfig, func(context.Context) ([]*corev1.Pod, error) {
			return rm.GetPods(), nil
		})
		if err != nil {
			return nil, nil, err
		}

		return p, nil, nil
	}

	cm, err := nodeutil.NewNodeFromClient(client, c.NodeName, newProvider, func(cfg *nodeutil.NodeConfig) error {
		cfg.InformerResyncPeriod = c.InformerResyncPeriod

		if taint != nil {
			cfg.NodeSpec.Spec.Taints = append(cfg.NodeSpec.Spec.Taints, *taint)
		}
		cfg.NodeSpec.Status.NodeInfo.Architecture = runtime.GOARCH
		cfg.NodeSpec.Status.NodeInfo.OperatingSystem = c.OperatingSystem

		return nil
	})
	if err != nil {
		return err
	}

	if err := setupTracing(ctx, c); err != nil {
		return err
	}

	ctx = log.WithLogger(ctx, log.G(ctx).WithFields(log.Fields{
		"provider":         c.Provider,
		"operatingSystem":  c.OperatingSystem,
		"node":             c.NodeName,
		"watchedNamespace": c.KubeNamespace,
	}))

	defer cancelHTTP()

	go cm.Run(ctx, c.PodSyncWorkers) // nolint:errcheck

	defer func() {
		log.G(ctx).Debug("Waiting for controllers to be done")
		cancel()
		<-cm.Done()
	}()

	log.G(ctx).Info("Waiting for controller to be ready")
	if err := cm.WaitReady(ctx, c.StartupTimeout); err != nil {
		return err
	}

	select {
	case <-ctx.Done():
	case <-cm.Done():
		return cm.Err()
	}
	return nil
}
