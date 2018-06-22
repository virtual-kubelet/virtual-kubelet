package vic

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"

	"gopkg.in/yaml.v2"

	"strings"

	"github.com/vmware/vic/pkg/trace"
)

type VicConfig struct {
	PersonaAddr   string `yaml:"persona-server"`
	PortlayerAddr string `yaml:"portlayer-server"`
	HostUUID      string `yaml:"host-uuid"`
}

const (
	personaAddrEnv      = "PERSONA_ADDR"
	portlayerAddrEnv    = "PORTLAYER_ADDR"
	hostUUIDEnv         = "HOST_UUID"
	localVirtualKubelet = "LOCAL_VIRTUAL_KUBELET"
)

func LocalInstance() bool {
	value := strings.ToLower(os.Getenv(localVirtualKubelet))

	if value == "1" || value == "t" || value == "true" {
		return true
	}

	return false
}

func NewVicConfig(op trace.Operation, configFile string) VicConfig {
	var config VicConfig

	if configFile == "" {
		config.loadConfigFromEnv()
	} else {
		config.loadConfigFile(configFile)
	}

	return config
}

func (v *VicConfig) loadConfigFile(configFile string) error {
	op := trace.NewOperation(context.Background(), "LoadConfigFile - %s", configFile)
	defer trace.End(trace.Begin("", op))

	contents, err := ioutil.ReadFile(configFile)
	if err != nil {
		return err
	}

	var config VicConfig
	err = yaml.Unmarshal(contents, &config)
	if err != nil {
		err = fmt.Errorf("Unable to unmarshal vic virtual kubelet configfile: %s", err.Error())
		op.Error(err)
		return err
	}

	*v = config

	return nil
}

func (v *VicConfig) loadConfigFromEnv() {
	v.PersonaAddr = os.Getenv(personaAddrEnv)
	v.PortlayerAddr = os.Getenv(portlayerAddrEnv)
	v.HostUUID = os.Getenv(hostUUIDEnv)
}
