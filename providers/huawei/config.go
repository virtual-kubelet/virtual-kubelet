package huawei

import (
	"io"

	"github.com/BurntSushi/toml"
	"github.com/virtual-kubelet/virtual-kubelet/providers"
	"k8s.io/apimachinery/pkg/util/uuid"
)

type providerConfig struct {
	Project         string
	Region          string
	Service         string
	OperatingSystem string
	CPU             string
	Memory          string
	Pods            string
}

func (p *CCIProvider) loadConfig(r io.Reader) error {
	var config providerConfig
	if _, err := toml.DecodeReader(r, &config); err != nil {
		return err
	}

	p.apiEndpoint = defaultApiEndpoint
	p.service = "CCI"
	p.region = config.Region
	if p.region == "" {
		p.region = "southchina"
	}
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
	p.project = config.Project
	if p.project == "" {
		p.project = string(uuid.NewUUID())
	}
	p.operatingSystem = config.OperatingSystem
	if p.operatingSystem == "" {
		p.operatingSystem = providers.OperatingSystemLinux
	}
	return nil
}
