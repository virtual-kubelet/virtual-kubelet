// +build azure_provider

package register

import (
	"github.com/virtual-kubelet/virtual-kubelet/providers"
	"github.com/virtual-kubelet/virtual-kubelet/providers/azure"
)

func init() {
	register("azure", initAzure)
}

func initAzure(cfg InitConfig) (providers.Provider, error) {
	return azure.NewACIProvider(
		cfg.ConfigPath,
		cfg.ResourceManager,
		cfg.NodeName,
		cfg.OperatingSystem,
		cfg.InternalIP,
		cfg.DaemonPort,
	)
}
