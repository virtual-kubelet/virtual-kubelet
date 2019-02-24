// +build !no_sfmesh_provider

package register

import (
	"github.com/iofog/virtual-kubelet/providers"
	"github.com/iofog/virtual-kubelet/providers/sfmesh"
)

func init() {
	register("sfmesh", sfmeshInit)
}

func sfmeshInit(cfg InitConfig) (providers.Provider, error) {
	return sfmesh.NewSFMeshProvider(
		cfg.ResourceManager,
		cfg.NodeName,
		cfg.OperatingSystem,
		cfg.InternalIP,
		cfg.DaemonPort,
	)
}
