// +build aws_provider

package register

import (
	"github.com/virtual-kubelet/virtual-kubelet/providers"
	"github.com/virtual-kubelet/virtual-kubelet/providers/aws"
)

func init() {
	register("aws", initAWS)
}

func initAWS(cfg InitConfig) (providers.Provider, error) {
	return aws.NewFargateProvider(cfg.ConfigPath, cfg.ResourceManager, cfg.NodeName, cfg.OperatingSystem, cfg.InternalIP, cfg.DaemonPort)
}
