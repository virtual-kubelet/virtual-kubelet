// +build !no_azure_provider

package register

import (
	"github.com/iofog/virtual-kubelet/providers"
	"github.com/iofog/virtual-kubelet/providers/azure"
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
