package alibabacloud

import (
	"io"

	"github.com/BurntSushi/toml"
	"github.com/virtual-kubelet/virtual-kubelet/providers"
)

type providerConfig struct {
	Region          string
	OperatingSystem string
	CPU             string
	Memory          string
	Pods            string
	VSwitch         string
	SecureGroup     string
	ClusterName     string
}

func (p *ECIProvider) loadConfig(r io.Reader) error {
	var config providerConfig
	if _, err := toml.DecodeReader(r, &config); err != nil {
		return err
	}

	p.region = config.Region
	if p.region == "" {
		p.region = "cn-hangzhou"
	}

	p.vSwitch = config.VSwitch
	p.secureGroup = config.SecureGroup

	p.cpu = config.CPU
	if p.cpu == "" {
		p.cpu = "20"
	}
	p.memory = config.Memory
	if p.memory == "" {
		p.memory = "100Gi"
	}
	p.pods = config.Pods
	if p.pods == "" {
		p.pods = "20"
	}
	p.operatingSystem = config.OperatingSystem
	if p.operatingSystem == "" {
		p.operatingSystem = providers.OperatingSystemLinux
	}
	p.clusterName = config.ClusterName
	if p.clusterName == "" {
		p.clusterName = "default"
	}
	return nil
}
