package hypersh

import (
	"fmt"
	"io"

	"github.com/virtual-kubelet/virtual-kubelet/providers"
	"github.com/BurntSushi/toml"
)

type providerConfig struct {
	Region          string
	AccessKey       string
	SecretKey       string
	OperatingSystem string
	CPU             string
	Memory          string
	Pods            string
}

func (p *HyperProvider) loadConfig(r io.Reader) error {
	var config providerConfig
	if _, err := toml.DecodeReader(r, &config); err != nil {
		return err
	}
	p.region = config.Region
	p.accessKey = config.AccessKey
	p.secretKey = config.SecretKey

	// Default to 20 mcpu
	p.cpu = "20"
	if config.CPU != "" {
		p.cpu = config.CPU
	}
	// Default to 100Gi
	p.memory = "100Gi"
	if config.Memory != "" {
		p.memory = config.Memory
	}
	// Default to 20 pods
	p.pods = "20"
	if config.Pods != "" {
		p.pods = config.Pods
	}

	// Default to Linux if the operating system was not defined in the config.
	if config.OperatingSystem == "" {
		config.OperatingSystem = providers.OperatingSystemLinux
	}

	// Validate operating system from config.
	if config.OperatingSystem != providers.OperatingSystemLinux {
		return fmt.Errorf("%q is not a valid operating system, only %s is valid", config.OperatingSystem, providers.OperatingSystemLinux)
	}

	p.operatingSystem = config.OperatingSystem
	return nil
}
