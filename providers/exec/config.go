package exec

import (
	"fmt"
	"io"
	"strings"

	"github.com/virtual-kubelet/virtual-kubelet/providers"
	"github.com/BurntSushi/toml"
	"runtime"
	"os"
	"strconv"
)

type providerConfig struct {
	LogDir			string
	OperatingSystem string
	Pods            string
	StateDir		string
	Memory			string
}

func (p *ExecProvider) loadConfig(r io.Reader) error {
	var config providerConfig
	if _, err := toml.DecodeReader(r, &config); err != nil {
		return err
	}

	// Default to 20 pods
	p.pods = "20"
	if config.Pods != "" {
		p.pods = config.Pods
	}

	// Default to Linux if the operating system was not defined in the config.
	if config.OperatingSystem == "" {
		config.OperatingSystem = providers.OperatingSystemLinux
	} else {
		// Validate operating system from config.
		ok, _ := providers.ValidOperatingSystems[config.OperatingSystem]
		if !ok {
			return fmt.Errorf("%q is not a valid operating system, try one of the following instead: %s",
				config.OperatingSystem, strings.Join(providers.ValidOperatingSystems.Names(), " | "))
		}
	}

	// Default logDir to the temporary directory
	p.logDir = os.TempDir()
	if config.LogDir != "" {
		p.logDir = config.LogDir
	}

	// Default stateDir to the temporary directory
	// stateDir is used to store the state file (exec.db) and the exit codes of all "containers"
	p.stateDir = os.TempDir()
	if config.StateDir != "" {
		p.stateDir = config.StateDir
	}

	// System CPU
	p.cpu = strconv.Itoa(runtime.NumCPU())

	// Configurable Memory
	// Default to 8Gi
	p.memory = "8Gi"
	if config.Memory != "" {
		p.memory = config.Memory
	}

	p.operatingSystem = config.OperatingSystem
	return nil
}