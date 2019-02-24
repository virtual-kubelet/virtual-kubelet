// +build !no_aws_provider

package register

import (
	"github.com/iofog/virtual-kubelet/providers"
	"github.com/iofog/virtual-kubelet/providers/aws"
)

func init() {
	register("aws", initAWS)
}

func initAWS(cfg InitConfig) (providers.Provider, error) {
	return aws.NewFargateProvider(cfg.ConfigPath, cfg.ResourceManager, cfg.NodeName, cfg.OperatingSystem, cfg.InternalIP, cfg.DaemonPort)
}
