package provider

import (
	"sync"

	"github.com/virtual-kubelet/virtual-kubelet/errdefs"
	"github.com/virtual-kubelet/virtual-kubelet/internal/manager"
)

// Store is used for registering/fetching providers
type Store struct {
	mu sync.Mutex
	ls map[string]InitFunc
}

func NewStore() *Store {
	return &Store{
		ls: make(map[string]InitFunc),
	}
}

// Register registers a providers init func by name
func (s *Store) Register(name string, f InitFunc) error {
	if f == nil {
		return errdefs.InvalidInput("provided init function cannot not be nil")
	}
	s.mu.Lock()
	s.ls[name] = f
	s.mu.Unlock()
	return nil
}

// Get gets the registered init func for the given name
// The returned function may be nil if the given name is not registered.
func (s *Store) Get(name string) InitFunc {
	s.mu.Lock()
	f := s.ls[name]
	s.mu.Unlock()
	return f
}

// List lists all the registered providers
func (s *Store) List() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	ls := make([]string, 0, len(s.ls))
	for p := range s.ls {
		ls = append(ls, p)
	}

	return ls
}

// Exists returns if there is an init function registered under the provided name
func (s *Store) Exists(name string) bool {
	s.mu.Lock()
	_, ok := s.ls[name]
	s.mu.Unlock()
	return ok
}

// InitConfig is the config passed to initialize a registered provider.
type InitConfig struct {
	ConfigPath        string
	NodeName          string
	OperatingSystem   string
	InternalIP        string
	DaemonPort        int32
	KubeClusterDomain string
	ResourceManager   *manager.ResourceManager
}

type InitFunc func(InitConfig) (Provider, error)
