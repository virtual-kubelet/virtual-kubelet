// +build web_provider

package register

import (
	"github.com/virtual-kubelet/virtual-kubelet/providers"
	"github.com/virtual-kubelet/virtual-kubelet/providers/web"
)

func init() {
	register("web", initWeb)
}

func initWeb(cfg InitConfig) (providers.Provider, error) {
	return web.NewBrokerProvider(cfg.NodeName, cfg.OperatingSystem, cfg.DaemonPort)
}
