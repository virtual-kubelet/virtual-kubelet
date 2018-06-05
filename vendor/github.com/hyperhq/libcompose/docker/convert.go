package docker

import (
	"fmt"
	"strings"

	"github.com/docker/docker/runconfig/opts"
	"github.com/docker/engine-api/types/container"
	"github.com/docker/engine-api/types/network"
	"github.com/docker/engine-api/types/strslice"
	"github.com/docker/go-connections/nat"
	"github.com/docker/go-units"
	"github.com/docker/libcompose/config"
	"github.com/docker/libcompose/project"
	"github.com/docker/libcompose/utils"
)

// ConfigWrapper wraps Config, HostConfig and NetworkingConfig for a container.
type ConfigWrapper struct {
	Config           *container.Config
	HostConfig       *container.HostConfig
	NetworkingConfig *network.NetworkingConfig
}

// Filter filters the specified string slice with the specified function.
func Filter(vs []string, f func(string) bool) []string {
	r := make([]string, 0, len(vs))
	for _, v := range vs {
		if f(v) {
			r = append(r, v)
		}
	}
	return r
}

func isBind(s string) bool {
	return strings.ContainsRune(s, ':')
}

func isVolume(s string) bool {
	return !isBind(s)
}

// ConvertToAPI converts a service configuration to a docker API container configuration.
func ConvertToAPI(s *Service) (*ConfigWrapper, error) {
	config, hostConfig, err := Convert(s.serviceConfig, s.context.Context)
	if err != nil {
		return nil, err
	}

	result := ConfigWrapper{
		Config:     config,
		HostConfig: hostConfig,
	}
	return &result, nil
}

func volumes(c *config.ServiceConfig, ctx project.Context) map[string]struct{} {
	volumes := make(map[string]struct{}, len(c.Volumes))
	for k, v := range c.Volumes {
		vol := ctx.ResourceLookup.ResolvePath(v, ctx.ComposeFiles[0])

		c.Volumes[k] = vol
		if isVolume(vol) {
			volumes[vol] = struct{}{}
		}
	}
	return volumes
}

func restartPolicy(c *config.ServiceConfig) (*container.RestartPolicy, error) {
	restart, err := opts.ParseRestartPolicy(c.Restart)
	if err != nil {
		return nil, err
	}
	return &container.RestartPolicy{Name: restart.Name, MaximumRetryCount: restart.MaximumRetryCount}, nil
}

func ports(c *config.ServiceConfig) (map[nat.Port]struct{}, nat.PortMap, error) {
	ports, binding, err := nat.ParsePortSpecs(c.Ports)
	if err != nil {
		return nil, nil, err
	}

	exPorts, _, err := nat.ParsePortSpecs(c.Expose)
	if err != nil {
		return nil, nil, err
	}

	for k, v := range exPorts {
		ports[k] = v
	}

	exposedPorts := map[nat.Port]struct{}{}
	for k, v := range ports {
		exposedPorts[nat.Port(k)] = v
	}

	portBindings := nat.PortMap{}
	for k, bv := range binding {
		dcbs := make([]nat.PortBinding, len(bv))
		for k, v := range bv {
			dcbs[k] = nat.PortBinding{HostIP: v.HostIP, HostPort: v.HostPort}
		}
		portBindings[nat.Port(k)] = dcbs
	}
	return exposedPorts, portBindings, nil
}

// Convert converts a service configuration to an docker API structures (Config and HostConfig)
func Convert(c *config.ServiceConfig, ctx project.Context) (*container.Config, *container.HostConfig, error) {
	restartPolicy, err := restartPolicy(c)
	if err != nil {
		return nil, nil, err
	}

	exposedPorts, portBindings, err := ports(c)
	if err != nil {
		return nil, nil, err
	}

	deviceMappings, err := parseDevices(c.Devices)
	if err != nil {
		return nil, nil, err
	}

	var volumesFrom []string
	if c.VolumesFrom != nil {
		volumesFrom, err = getVolumesFrom(c.VolumesFrom, ctx.Project.Configs, ctx.ProjectName)
		if err != nil {
			return nil, nil, err
		}
	}

	config := &container.Config{
		Entrypoint:   strslice.StrSlice(utils.CopySlice(c.Entrypoint.Slice())),
		Hostname:     c.Hostname,
		Domainname:   c.DomainName,
		User:         c.User,
		Env:          utils.CopySlice(c.Environment.Slice()),
		Cmd:          strslice.StrSlice(utils.CopySlice(c.Command.Slice())),
		Image:        c.Image,
		Labels:       utils.CopyMap(c.Labels.MapParts()),
		ExposedPorts: exposedPorts,
		Tty:          c.Tty,
		OpenStdin:    c.StdinOpen,
		WorkingDir:   c.WorkingDir,
		Volumes:      volumes(c, ctx),
		MacAddress:   c.MacAddress,
	}

	ulimits := []*units.Ulimit{}
	if c.Ulimits.Elements != nil {
		for _, ulimit := range c.Ulimits.Elements {
			ulimits = append(ulimits, &units.Ulimit{
				Name: ulimit.Name,
				Soft: ulimit.Soft,
				Hard: ulimit.Hard,
			})
		}
	}

	resources := container.Resources{
		CgroupParent: c.CgroupParent,
		Memory:       c.MemLimit,
		MemorySwap:   c.MemSwapLimit,
		CPUShares:    c.CPUShares,
		CPUQuota:     c.CPUQuota,
		CpusetCpus:   c.CPUSet,
		Ulimits:      ulimits,
		Devices:      deviceMappings,
	}

	hostConfig := &container.HostConfig{
		VolumesFrom: volumesFrom,
		CapAdd:      strslice.StrSlice(utils.CopySlice(c.CapAdd)),
		CapDrop:     strslice.StrSlice(utils.CopySlice(c.CapDrop)),
		ExtraHosts:  utils.CopySlice(c.ExtraHosts),
		Privileged:  c.Privileged,
		Binds:       Filter(c.Volumes, isBind),
		DNS:         utils.CopySlice(c.DNS.Slice()),
		DNSSearch:   utils.CopySlice(c.DNSSearch.Slice()),
		LogConfig: container.LogConfig{
			Type:   c.LogDriver,
			Config: utils.CopyMap(c.LogOpt),
		},
		NetworkMode:    container.NetworkMode(c.Net),
		ReadonlyRootfs: c.ReadOnly,
		PidMode:        container.PidMode(c.Pid),
		UTSMode:        container.UTSMode(c.Uts),
		IpcMode:        container.IpcMode(c.Ipc),
		PortBindings:   portBindings,
		RestartPolicy:  *restartPolicy,
		SecurityOpt:    utils.CopySlice(c.SecurityOpt),
		VolumeDriver:   c.VolumeDriver,
		Resources:      resources,
	}

	return config, hostConfig, nil
}

func getVolumesFrom(volumesFrom []string, serviceConfigs *config.Configs, projectName string) ([]string, error) {
	volumes := []string{}
	for _, volumeFrom := range volumesFrom {
		if serviceConfigs.Has(volumeFrom) {
			// It's a service - Use the first one
			name := fmt.Sprintf("%s_%s_1", projectName, volumeFrom)
			volumes = append(volumes, name)
		} else {
			volumes = append(volumes, volumeFrom)
		}
	}
	return volumes, nil
}

func parseDevices(devices []string) ([]container.DeviceMapping, error) {
	// parse device mappings
	deviceMappings := []container.DeviceMapping{}
	for _, device := range devices {
		v, err := opts.ParseDevice(device)
		if err != nil {
			return nil, err
		}
		deviceMappings = append(deviceMappings, container.DeviceMapping{
			PathOnHost:        v.PathOnHost,
			PathInContainer:   v.PathInContainer,
			CgroupPermissions: v.CgroupPermissions,
		})
	}

	return deviceMappings, nil
}
