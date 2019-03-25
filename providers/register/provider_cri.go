// +build linux,cri_provider

package register

import (
	"github.com/virtual-kubelet/virtual-kubelet/providers"
	"github.com/virtual-kubelet/virtual-kubelet/providers/cri"
)

func init() {
	register("cri", criInit)
}

func criInit(cfg InitConfig) (providers.Provider, error) {
	return cri.NewCRIProvider(
		cfg.NodeName,
		cfg.OperatingSystem,
		cfg.InternalIP,
		cfg.ResourceManager,
		cfg.DaemonPort,
	)
}
