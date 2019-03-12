// +build azurebatch_provider

package register

import (
	"github.com/virtual-kubelet/virtual-kubelet/providers"
	"github.com/virtual-kubelet/virtual-kubelet/providers/azurebatch"
)

func init() {
	register("azurebatch", initAzureBatch)
}

func initAzureBatch(cfg InitConfig) (providers.Provider, error) {
	return azurebatch.NewBatchProvider(
		cfg.ConfigPath,
		cfg.ResourceManager,
		cfg.NodeName,
		cfg.OperatingSystem,
		cfg.InternalIP,
		cfg.DaemonPort,
	)
}
