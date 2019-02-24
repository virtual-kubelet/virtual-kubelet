// +build linux,!no_vic_provider

package register

import (
	"github.com/iofog/virtual-kubelet/providers"
	"github.com/iofog/virtual-kubelet/providers/vic"
)

func init() {
	register("vic", initVic)
}

func initVic(cfg InitConfig) (providers.Provider, error) {
	return vic.NewVicProvider(cfg.ConfigPath, cfg.ResourceManager, cfg.NodeName, cfg.OperatingSystem)
}
