package main

import (
	"github.com/virtual-kubelet/virtual-kubelet/cmd/virtual-kubelet/internal/provider"
	"github.com/virtual-kubelet/virtual-kubelet/cmd/virtual-kubelet/internal/provider/mock"
	"github.com/virtual-kubelet/virtual-kubelet/cmd/virtual-kubelet/internal/provider/ukama"
	"github.com/virtual-kubelet/virtual-kubelet/cmd/virtual-kubelet/internal/provider/web"
)

func registerMock(s *provider.Store) {
	s.Register("mock", func(cfg provider.InitConfig) (provider.Provider, error) { //nolint:errcheck
		return mock.NewMockProvider(
			cfg.ConfigPath,
			cfg.NodeName,
			cfg.OperatingSystem,
			cfg.InternalIP,
			cfg.DaemonPort,
		)
	})

	s.Register("ukama", func(cfg provider.InitConfig) (provider.Provider, error) { //nolint:errcheck
		return ukama.NewUkamaProvider(
			cfg.ConfigPath,
			cfg.NodeName,
			cfg.OperatingSystem,
			cfg.InternalIP,
			cfg.DaemonPort,
		)
	})

	s.Register("web", func(cfg provider.InitConfig) (provider.Provider, error) { //nolint:errcheck
		return web.NewBrokerProvider(
			// cfg.ConfigPath,
			cfg.NodeName,
			cfg.OperatingSystem,
			//cfg.InternalIP,
			cfg.DaemonPort,
		)
	})
}
