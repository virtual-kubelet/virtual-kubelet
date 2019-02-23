// +build mock_provider

package register

import (
	"github.com/virtual-kubelet/virtual-kubelet/providers"
	"github.com/virtual-kubelet/virtual-kubelet/providers/mock"
)

func init() {
	register("mock", initMock)
}

func initMock(cfg InitConfig) (providers.Provider, error) {
	return mock.NewMockProvider(
		cfg.ConfigPath,
		cfg.NodeName,
		cfg.OperatingSystem,
		cfg.InternalIP,
		cfg.DaemonPort,
	)
}
