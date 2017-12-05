package config

import (
	"sync"

	"github.com/hyperhq/libcompose/yaml"
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

// ServiceConfigV1 holds version 1 of libcompose service configuration
type ServiceConfigV1 struct {
	/*
		Build         string               `yaml:"build,omitempty"`
		CapAdd        []string             `yaml:"cap_add,omitempty"`
		CapDrop       []string             `yaml:"cap_drop,omitempty"`
		CgroupParent  string               `yaml:"cgroup_parent,omitempty"`
		CPUQuota      int64                `yaml:"cpu_quota,omitempty"`
		CPUSet        string               `yaml:"cpuset,omitempty"`
		CPUShares     int64                `yaml:"cpu_shares,omitempty"`
		Devices       []string             `yaml:"devices,omitempty"`
		DNS           yaml.Stringorslice   `yaml:"dns,omitempty"`
		DNSSearch     yaml.Stringorslice   `yaml:"dns_search,omitempty"`
		Dockerfile    string               `yaml:"dockerfile,omitempty"`
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
		ReadOnly      bool              `yaml:"read_only,omitempty"`
		SecurityOpt   []string          `yaml:"security_opt,omitempty"`
		User          string            `yaml:"user,omitempty"`
		VolumeDriver  string            `yaml:"volume_driver,omitempty"`
		VolumesFrom   []string          `yaml:"volumes_from,omitempty"`
		Expose        []string          `yaml:"expose,omitempty"`
		LogOpt        map[string]string `yaml:"log_opt,omitempty"`
		ExtraHosts    []string          `yaml:"extra_hosts,omitempty"`
		Ulimits       yaml.Ulimits      `yaml:"ulimits,omitempty"`
	*/
	Command       yaml.Command         `yaml:"command,flow,omitempty" json:"command,omitempty"`
	ContainerName string               `yaml:"container_name,omitempty" json:"container_name,omitempty"`
	DomainName    string               `yaml:"domainname,omitempty" json:"domainname,omitempty"`
	Entrypoint    yaml.Command         `yaml:"entrypoint,flow,omitempty" json:"entrypoint,omitempty"`
	EnvFile       yaml.Stringorslice   `yaml:"env_file,omitempty" json:"env_file,omitempty"`
	Environment   yaml.MaporEqualSlice `yaml:"environment,omitempty" json:"environment,omitempty"`
	Hostname      string               `yaml:"hostname,omitempty" json:"hostname,omitempty"`
	Image         string               `yaml:"image,omitempty" json:"image,omitempty"`
	Labels        yaml.SliceorMap      `yaml:"labels,omitempty" json:"labels,omitempty"`
	Links         yaml.MaporColonSlice `yaml:"links,omitempty" json:"links,omitempty"`
	Restart       string               `yaml:"restart,omitempty" json:"restart,omitempty"`
	StdinOpen     bool                 `yaml:"stdin_open,omitempty" json:"stdin_open,omitempty"`
	Tty           bool                 `yaml:"tty,omitempty" json:"tty,omitempty"`
	Volumes       []string             `yaml:"volumes,omitempty" json:"volumes,omitempty"`
	WorkingDir    string               `yaml:"working_dir,omitempty" json:"working_dir,omitempty"`
	ExternalLinks []string             `yaml:"external_links,omitempty" json:"external_links,omitempty"`

	Size           string   `yaml:"size,omitempty" json:"size,omitempty"`
	Fip            string   `yaml:"fip,omitempty" json:"fip,omitempty"`
	SecurityGroups []string `yaml:"security_groups,omitempty" json:"security_groups,omitempty"`
	NoAutoVolume   bool     `yaml:"noauto_volume,omitempty" json:"noauto_volume,omitempty"`
}

// Build holds v2 build information
type Build struct {
	Context    string               `yaml:"context,omitempty"`
	Dockerfile string               `yaml:"dockerfile,omitempty"`
	Args       yaml.MaporEqualSlice `yaml:"args,omitempty"`
}

// Log holds v2 logging information
type Log struct {
	Driver  string            `yaml:"driver,omitempty"`
	Options map[string]string `yaml:"options,omitempty"`
}

// ServiceConfig holds version 2 of libcompose service configuration
type ServiceConfig struct {
	/*
		Build         Build                `yaml:"build,omitempty"`
		CapAdd        []string             `yaml:"cap_add,omitempty"`
		CapDrop       []string             `yaml:"cap_drop,omitempty"`
		CPUSet        string               `yaml:"cpuset,omitempty"`
		CPUShares     int64                `yaml:"cpu_shares,omitempty"`
		CPUQuota      int64                `yaml:"cpu_quota,omitempty"`
		CgroupParent  string               `yaml:"cgroup_parrent,omitempty"`
		Devices       []string             `yaml:"devices,omitempty"`
		DNS           yaml.Stringorslice   `yaml:"dns,omitempty"`
		DNSSearch     yaml.Stringorslice   `yaml:"dns_search,omitempty"`
		Expose        []string             `yaml:"expose,omitempty"`
		Ipc           string               `yaml:"ipc,omitempty"`
		Logging       Log                  `yaml:"logging,omitempty"`
		MacAddress    string               `yaml:"mac_address,omitempty"`
		MemLimit      int64                `yaml:"mem_limit,omitempty"`
		MemSwapLimit  int64                `yaml:"memswap_limit,omitempty"`
		NetworkMode   string               `yaml:"network_mode,omitempty"`
		Networks      []string             `yaml:"networks,omitempty"`
		Pid           string               `yaml:"pid,omitempty"`
		Ports         []string             `yaml:"ports,omitempty"`
		Privileged    bool                 `yaml:"privileged,omitempty"`
		SecurityOpt   []string             `yaml:"security_opt,omitempty"`
		StopSignal    string               `yaml:"stop_signal,omitempty"`
		VolumeDriver  string               `yaml:"volume_driver,omitempty"`
		VolumesFrom   []string             `yaml:"volumes_from,omitempty"`
		Uts           string               `yaml:"uts,omitempty"`
		ReadOnly      bool                 `yaml:"read_only,omitempty"`
		User          string               `yaml:"user,omitempty"`
		Ulimits       yaml.Ulimits         `yaml:"ulimits,omitempty"`
	*/
	Expose        []string             `yaml:"expose,omitempty" json:"expose,omitempty"`
	Ports         []string             `yaml:"ports,omitempty" json:"ports,omitempty"`
	Command       yaml.Command         `yaml:"command,flow,omitempty" json:"command,omitempty"`
	ContainerName string               `yaml:"container_name,omitempty" json:"container_name,omitempty"`
	DomainName    string               `yaml:"domainname,omitempty" json:"domainname,omitempty"`
	DependsOn     []string             `yaml:"depends_on,omitempty" json:"depends_on,omitempty"`
	Entrypoint    yaml.Command         `yaml:"entrypoint,flow,omitempty" json:"entrypoint,omitempty"`
	EnvFile       yaml.Stringorslice   `yaml:"env_file,omitempty" json:"env_file,omitempty"`
	Environment   yaml.MaporEqualSlice `yaml:"environment,omitempty" json:"environment,omitempty"`
	Extends       yaml.MaporEqualSlice `yaml:"extends,omitempty" json:"extends,omitempty"`
	ExternalLinks []string             `yaml:"external_links,omitempty" json:"external_links"`
	Image         string               `yaml:"image,omitempty" json:"image,omitempty"`
	Hostname      string               `yaml:"hostname,omitempty" json:"hostname,omitempty"`
	Labels        yaml.SliceorMap      `yaml:"labels,omitempty" json:"labels,omitempty"`
	Links         yaml.MaporColonSlice `yaml:"links,omitempty" json:"links,omitempty"`
	Volumes       []string             `yaml:"volumes,omitempty" json:"volumes,omitempty"`
	Restart       string               `yaml:"restart,omitempty" json:"restart,omitempty"`
	StdinOpen     bool                 `yaml:"stdin_open,omitempty" json:"stdin_open,omitempty"`
	Tty           bool                 `yaml:"tty,omitempty" json:"tty,omitempty"`
	WorkingDir    string               `yaml:"working_dir,omitempty" json:"working_dir,omitempty"`

	Size           string   `yaml:"size,omitempty" json:"size,omitempty"`
	Fip            string   `yaml:"fip,omitempty" json:"fip,omitempty"`
	SecurityGroups []string `yaml:"security_groups,omitempty" json:"security_groups,omitempty"`
	NoAutoVolume   bool     `yaml:"noauto_volume,omitempty" json:"noauto_volume,omitempty"`
}

// VolumeConfig holds v2 volume configuration
type VolumeConfig struct {
	Driver     string            `yaml:"driver,omitempty"`
	DriverOpts map[string]string `yaml:"driver_opts,omitempty"`
	External   bool              `yaml:"external,omitempty"`
}

// Ipam holds v2 network IPAM information
type Ipam struct {
	Driver string   `yaml:"driver,omitempty"`
	Config []string `yaml:"config,omitempty"`
}

// NetworkConfig holds v2 network configuration
type NetworkConfig struct {
	Driver     string            `yaml:"driver,omitempty"`
	DriverOpts map[string]string `yaml:"driver_opts,omitempty"`
	External   bool              `yaml:"external,omitempty"`
	Ipam       Ipam              `yaml:"ipam,omitempty"`
}

// Config holds libcompose top level configuration
type Config struct {
	Version  string                    `yaml:"version,omitempty"`
	Services RawServiceMap             `yaml:"services,omitempty"`
	Volumes  map[string]*VolumeConfig  `yaml:"volumes,omitempty"`
	Networks map[string]*NetworkConfig `yaml:"networks,omitempty"`
}

// NewServiceConfigs initializes a new Configs struct
func NewServiceConfigs() *ServiceConfigs {
	return &ServiceConfigs{
		M: make(map[string]*ServiceConfig),
	}
}

// ServiceConfigs holds a concurrent safe map of ServiceConfig
type ServiceConfigs struct {
	M  map[string]*ServiceConfig
	mu sync.RWMutex
}

// Has checks if the config map has the specified name
func (c *ServiceConfigs) Has(name string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, ok := c.M[name]
	return ok
}

// Get returns the config and the presence of the specified name
func (c *ServiceConfigs) Get(name string) (*ServiceConfig, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	service, ok := c.M[name]
	return service, ok
}

// Add add the specifed config with the specified name
func (c *ServiceConfigs) Add(name string, service *ServiceConfig) {
	c.mu.Lock()
	c.M[name] = service
	c.mu.Unlock()
}

// Len returns the len of the configs
func (c *ServiceConfigs) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.M)
}

// Keys returns the names of the config
func (c *ServiceConfigs) Keys() []string {
	keys := []string{}
	c.mu.RLock()
	defer c.mu.RUnlock()
	for name := range c.M {
		keys = append(keys, name)
	}
	return keys
}

// RawService is represent a Service in map form unparsed
type RawService map[string]interface{}

// RawServiceMap is a collection of RawServices
type RawServiceMap map[string]RawService
