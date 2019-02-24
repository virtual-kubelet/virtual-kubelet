package register

import (
	"github.com/cpuguy83/strongerrors"
	"github.com/iofog/virtual-kubelet/manager"
	"github.com/iofog/virtual-kubelet/providers"
	"github.com/pkg/errors"
)

var providerInits = make(map[string]initFunc)

// InitConfig is the config passed to initialize a registered provider.
type InitConfig struct {
	ConfigPath      string
	NodeName        string
	OperatingSystem string
	InternalIP      string
	DaemonPort      int32
	ResourceManager *manager.ResourceManager
	ControllerToken string
}

type initFunc func(InitConfig) (providers.Provider, error)

// GetProvider gets the provider specified by the given name
func GetProvider(name string, cfg InitConfig) (providers.Provider, error) {
	f, ok := providerInits[name]
	if !ok {
		return nil, strongerrors.NotFound(errors.Errorf("provider not found: %s", name))
	}
	return f(cfg)
}

func register(name string, f initFunc) {
	providerInits[name] = f
}
