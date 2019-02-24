// +build !no_nomad_provider

package register

import (
	"github.com/iofog/virtual-kubelet/providers"
	"github.com/iofog/virtual-kubelet/providers/nomad"
)

func init() {
	register("nomad", initNomad)
}

func initNomad(cfg InitConfig) (providers.Provider, error) {
	return nomad.NewProvider(cfg.ResourceManager, cfg.NodeName, cfg.OperatingSystem)
}
