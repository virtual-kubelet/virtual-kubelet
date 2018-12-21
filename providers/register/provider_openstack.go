package register

import (
	"fmt"
	"github.com/virtual-kubelet/virtual-kubelet/providers"
	"github.com/virtual-kubelet/virtual-kubelet/providers/openstack"
)

func init() {
	fmt.Errorf("Jack test lalalala:%s","gongwenqing")
	register("openstack", openstackInit)
}

func openstackInit(cfg InitConfig) (providers.Provider, error) {
	return openstack.NewZunProvider(cfg.ConfigPath,cfg.ResourceManager,cfg.NodeName,cfg.OperatingSystem,cfg.DaemonPort)
}
