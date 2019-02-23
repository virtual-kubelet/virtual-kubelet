// +build alibabacloud_provider

package register

import (
	"github.com/virtual-kubelet/virtual-kubelet/providers"
	"github.com/virtual-kubelet/virtual-kubelet/providers/alibabacloud"
)

func init() {
	register("alibabacloud", aliCloudInit)
}

func aliCloudInit(cfg InitConfig) (providers.Provider, error) {
	return alibabacloud.NewECIProvider(
		cfg.ConfigPath,
		cfg.ResourceManager,
		cfg.NodeName,
		cfg.OperatingSystem,
		cfg.InternalIP,
		cfg.DaemonPort,
	)
}
