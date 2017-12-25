package daemon

import (
	"github.com/hyperhq/hyper-api/types"
	"github.com/hyperhq/hypercli/container"
	"github.com/hyperhq/hypercli/daemon/execdriver"
)

// setPlatformSpecificExecProcessConfig sets platform-specific fields in the
// ProcessConfig structure. This is a no-op on Windows
func setPlatformSpecificExecProcessConfig(config *types.ExecConfig, container *container.Container, pc *execdriver.ProcessConfig) {
}
