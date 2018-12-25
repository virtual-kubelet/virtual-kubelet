// +build !no_alicloud_provider

package register

import (
	"github.com/virtual-kubelet/virtual-kubelet/providers"
	"github.com/virtual-kubelet/virtual-kubelet/providers/alicloud"
)

func init() {
	register("alicloud", aliCloudInit)
}

func aliCloudInit(cfg InitConfig) (providers.Provider, error) {
	return alicloud.NewECIProvider(
		cfg.ConfigPath,
		cfg.ResourceManager,
		cfg.NodeName,
		cfg.OperatingSystem,
		cfg.InternalIP,
		cfg.DaemonPort,
	)
}
