package openstack

import (
	"fmt"
	"github.com/BurntSushi/toml"
	"github.com/virtual-kubelet/virtual-kubelet/providers"
	"io"
	"strings"
)

type providerConfig struct {
	Region string
	OperatingSystem string
	CPU string
	Memory string
	Pods string
}

func (p *ZunProvider) loadConfig(r io.Reader) error{
	var config providerConfig
	if _ , err := toml.DecodeReader(r, &config);err != nil{
		return err
	}

	p.region = config.Region

	// Default to 20 mcpu
	p.cpu = "20"

	if config.CPU != ""{
		p.cpu = config.CPU
	}

	// Default to 64Gi
	p.memory = "64Gi"
	if config.Memory != ""{
		p.memory = config.Memory
	}

	//Default to 20 pods
	p.pods = "20"

	if config.Pods != ""{
		p.pods = config.Pods
	}


	if config.OperatingSystem == ""{
		config.OperatingSystem = providers.OperatingSystemLinux
	}else{
		ok,_ := providers.ValidOperatingSystems[config.OperatingSystem]

		if !ok{
			return fmt.Errorf("%q is not a valid operating system, try one of the following instead: %s", config.OperatingSystem, strings.Join(providers.ValidOperatingSystems.Names(), " | "))
		}
	}

	p.operatingSystem = config.OperatingSystem
	return nil
}
