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

package main

import (
	"context"

	"github.com/sirupsen/logrus"
	cli "github.com/virtual-kubelet/node-cli"
	logruscli "github.com/virtual-kubelet/node-cli/logrus"
	opencensuscli "github.com/virtual-kubelet/node-cli/opencensus"
	"github.com/virtual-kubelet/node-cli/opts"
	"github.com/virtual-kubelet/node-cli/provider"
	"github.com/virtual-kubelet/virtual-kubelet/internal/providers/mock"
	"github.com/virtual-kubelet/virtual-kubelet/log"
	logruslogger "github.com/virtual-kubelet/virtual-kubelet/log/logrus"
	"github.com/virtual-kubelet/virtual-kubelet/trace"
	"github.com/virtual-kubelet/virtual-kubelet/trace/opencensus"
)

var (
	buildVersion = "N/A"
	buildTime    = "N/A"
	k8sVersion   = "v1.13.7" // This should follow the version of k8s.io/kubernetes we are importing
)

func main() {
	ctx := cli.ContextWithCancelOnSignal(context.Background())
	logger := logrus.StandardLogger()

	log.L = logruslogger.FromLogrus(logrus.NewEntry(logger))
	logConfig := &logruscli.Config{LogLevel: "info"}

	trace.T = opencensus.Adapter{}
	traceConfig := opencensuscli.FromEnv()
	traceConfig.AvailableExporters = tracingExporters

	opts, err := opts.FromEnv()
	if err != nil {
		log.G(ctx).Fatal(err)
	}

	node, err := cli.New(ctx,
		cli.WithCLIVersion(buildVersion, buildTime),
		cli.WithBaseOpts(opts),
		cli.WithPersistentFlags(traceConfig.FlagSet()),
		cli.WithPersistentPreRunCallback(func() error {
			return opencensuscli.Configure(ctx, traceConfig, opts)
		}),
		cli.WithPersistentFlags(logConfig.FlagSet()),
		cli.WithPersistentPreRunCallback(func() error {
			return logruscli.Configure(logConfig, logger)
		}),
		cli.WithProvider("mock", func(cfg provider.InitConfig) (provider.Provider, error) {
			return mock.NewMockProvider(cfg.ConfigPath, cfg.NodeName, cfg.OperatingSystem, cfg.InternalIP, cfg.DaemonPort)
		}),
		cli.WithProvider("mockV0", func(cfg provider.InitConfig) (provider.Provider, error) {
			return mock.NewMockV0Provider(cfg.ConfigPath, cfg.NodeName, cfg.OperatingSystem, cfg.InternalIP, cfg.DaemonPort)
		}),
	)
	if err != nil {
		log.G(ctx).Fatal(err)
	}

	if err := node.Run(); err != nil {
		log.G(ctx).Fatal(err)
	}
}
