package main

import (
	"github.com/virtual-kubelet/virtual-kubelet/node/cli"
	"github.com/virtual-kubelet/virtual-kubelet/node/cli/provider"
	"github.com/virtual-kubelet/virtual-kubelet/node/cli/provider/mock"
)

func registerMock() []cli.Option {
	return []cli.Option{
		cli.WithProvider("mock", func(cfg provider.InitConfig) (provider.Provider, error) {
			return mock.NewMockProvider(
				cfg.ConfigPath,
				cfg.NodeName,
				cfg.OperatingSystem,
				cfg.InternalIP,
				cfg.DaemonPort,
			)
		}),
		cli.WithProvider("mockV0", func(cfg provider.InitConfig) (provider.Provider, error) {
			return mock.NewMockProvider(
				cfg.ConfigPath,
				cfg.NodeName,
				cfg.OperatingSystem,
				cfg.InternalIP,
				cfg.DaemonPort,
			)
		}),
	}
}
