package register

import (
	"sort"

	"github.com/cpuguy83/strongerrors"
	"github.com/pkg/errors"
	"github.com/virtual-kubelet/virtual-kubelet/manager"
	"github.com/virtual-kubelet/virtual-kubelet/providers"
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

// Exists checks if a provider is regstered
func Exists(name string) bool {
	_, ok := providerInits[name]
	return ok
}

// List gets the list of all provider names
func List() []string {
	ls := make([]string, 0, len(providerInits))
	for name := range providerInits {
		ls = append(ls, name)
	}
	sort.Strings(ls)
	return ls
}

func register(name string, f initFunc) {
	providerInits[name] = f
}
