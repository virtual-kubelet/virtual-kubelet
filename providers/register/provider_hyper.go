// +build !no_hyper_provider

package register

import (
	"github.com/virtual-kubelet/virtual-kubelet/providers"
	"github.com/virtual-kubelet/virtual-kubelet/providers/hypersh"
)

func init() {
	register("hyper", initHyper)
}

func initHyper(cfg InitConfig) (providers.Provider, error) {
	return hypersh.NewHyperProvider(cfg.ConfigPath, cfg.ResourceManager, cfg.NodeName, cfg.OperatingSystem)
}
