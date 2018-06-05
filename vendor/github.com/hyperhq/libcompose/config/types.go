package config

import (
	"github.com/docker/libcompose/yaml"
	"sync"
)

// EnvironmentLookup defines methods to provides environment variable loading.
type EnvironmentLookup interface {
	Lookup(key, serviceName string, config *ServiceConfig) []string
}

// ResourceLookup defines methods to provides file loading.
type ResourceLookup interface {
	Lookup(file, relativeTo string) ([]byte, string, error)
	ResolvePath(path, inFile string) string
}

// ServiceConfig holds libcompose service configuration
type ServiceConfig struct {
	Build         string               `yaml:"build,omitempty"`
	CapAdd        []string             `yaml:"cap_add,omitempty"`
	CapDrop       []string             `yaml:"cap_drop,omitempty"`
	CgroupParent  string               `yaml:"cgroup_parent,omitempty"`
	CPUQuota      int64                `yaml:"cpu_quota,omitempty"`
	CPUSet        string               `yaml:"cpuset,omitempty"`
	CPUShares     int64                `yaml:"cpu_shares,omitempty"`
	Command       yaml.Command         `yaml:"command,flow,omitempty"`
	ContainerName string               `yaml:"container_name,omitempty"`
	Devices       []string             `yaml:"devices,omitempty"`
	DNS           yaml.Stringorslice   `yaml:"dns,omitempty"`
	DNSSearch     yaml.Stringorslice   `yaml:"dns_search,omitempty"`
	Dockerfile    string               `yaml:"dockerfile,omitempty"`
	DomainName    string               `yaml:"domainname,omitempty"`
	Entrypoint    yaml.Command         `yaml:"entrypoint,flow,omitempty"`
	EnvFile       yaml.Stringorslice   `yaml:"env_file,omitempty"`
	Environment   yaml.MaporEqualSlice `yaml:"environment,omitempty"`
	Hostname      string               `yaml:"hostname,omitempty"`
	Image         string               `yaml:"image,omitempty"`
	Labels        yaml.SliceorMap      `yaml:"labels,omitempty"`
	Links         yaml.MaporColonSlice `yaml:"links,omitempty"`
	LogDriver     string               `yaml:"log_driver,omitempty"`
	MacAddress    string               `yaml:"mac_address,omitempty"`
	MemLimit      int64                `yaml:"mem_limit,omitempty"`
	MemSwapLimit  int64                `yaml:"memswap_limit,omitempty"`
	Name          string               `yaml:"name,omitempty"`
	Net           string               `yaml:"net,omitempty"`
	Pid           string               `yaml:"pid,omitempty"`
	Uts           string               `yaml:"uts,omitempty"`
	Ipc           string               `yaml:"ipc,omitempty"`
	Ports         []string             `yaml:"ports,omitempty"`
	Privileged    bool                 `yaml:"privileged,omitempty"`
	Restart       string               `yaml:"restart,omitempty"`
	ReadOnly      bool                 `yaml:"read_only,omitempty"`
	StdinOpen     bool                 `yaml:"stdin_open,omitempty"`
	SecurityOpt   []string             `yaml:"security_opt,omitempty"`
	Tty           bool                 `yaml:"tty,omitempty"`
	User          string               `yaml:"user,omitempty"`
	VolumeDriver  string               `yaml:"volume_driver,omitempty"`
	Volumes       []string             `yaml:"volumes,omitempty"`
	VolumesFrom   []string             `yaml:"volumes_from,omitempty"`
	WorkingDir    string               `yaml:"working_dir,omitempty"`
	Expose        []string             `yaml:"expose,omitempty"`
	ExternalLinks []string             `yaml:"external_links,omitempty"`
	LogOpt        map[string]string    `yaml:"log_opt,omitempty"`
	ExtraHosts    []string             `yaml:"extra_hosts,omitempty"`
	Ulimits       yaml.Ulimits         `yaml:"ulimits,omitempty"`
}

// NewConfigs initializes a new Configs struct
func NewConfigs() *Configs {
	return &Configs{
		m: make(map[string]*ServiceConfig),
	}
}

// Configs holds a concurrent safe map of ServiceConfig
type Configs struct {
	m  map[string]*ServiceConfig
	mu sync.RWMutex
}

// Has checks if the config map has the specified name
func (c *Configs) Has(name string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, ok := c.m[name]
	return ok
}

// Get returns the config and the presence of the specified name
func (c *Configs) Get(name string) (*ServiceConfig, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	service, ok := c.m[name]
	return service, ok
}

// Add add the specifed config with the specified name
func (c *Configs) Add(name string, service *ServiceConfig) {
	c.mu.Lock()
	c.m[name] = service
	c.mu.Unlock()
}

// Len returns the len of the configs
func (c *Configs) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.m)
}

// Keys returns the names of the config
func (c *Configs) Keys() []string {
	keys := []string{}
	c.mu.RLock()
	defer c.mu.RUnlock()
	for name := range c.m {
		keys = append(keys, name)
	}
	return keys
}

// RawService is represent a Service in map form unparsed
type RawService map[string]interface{}

// RawServiceMap is a collection of RawServices
type RawServiceMap map[string]RawService
