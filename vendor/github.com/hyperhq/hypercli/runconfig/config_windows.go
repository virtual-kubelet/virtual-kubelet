package runconfig

import (
	"github.com/hyperhq/hyper-api/types/container"
	networktypes "github.com/hyperhq/hyper-api/types/network"
)

// ContainerConfigWrapper is a Config wrapper that hold the container Config (portable)
// and the corresponding HostConfig (non-portable).
type ContainerConfigWrapper struct {
	*container.Config
	HostConfig       *container.HostConfig          `json:"HostConfig,omitempty"`
	NetworkingConfig *networktypes.NetworkingConfig `json:"NetworkingConfig,omitempty"`
}

// getHostConfig gets the HostConfig of the Config.
func (w *ContainerConfigWrapper) getHostConfig() *container.HostConfig {
	return w.HostConfig
}
