package docker

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"

	"golang.org/x/net/context"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/promise"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/docker/pkg/term"
	"github.com/docker/docker/reference"
	"github.com/docker/docker/registry"
	"github.com/docker/engine-api/client"
	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/container"
	"github.com/docker/go-connections/nat"
	"github.com/docker/libcompose/config"
	"github.com/docker/libcompose/logger"
	"github.com/docker/libcompose/project"
	util "github.com/docker/libcompose/utils"
)

// DefaultTag is the name of the default tag of an image.
const DefaultTag = "latest"

// ComposeVersion is name of docker-compose.yml file syntax supported version
const ComposeVersion = "1.5.0"

// Container holds information about a docker container and the service it is tied on.
// It implements Service interface by encapsulating a EmptyService.
type Container struct {
	project.EmptyService

	name    string
	service *Service
	client  client.APIClient
}

// NewContainer creates a container struct with the specified docker client, name and service.
func NewContainer(client client.APIClient, name string, service *Service) *Container {
	return &Container{
		client:  client,
		name:    name,
		service: service,
	}
}

func (c *Container) findExisting() (*types.Container, error) {
	return GetContainerByName(c.client, c.name)
}

func (c *Container) findInfo() (types.ContainerJSON, error) {
	container, err := c.findExisting()
	if err != nil {
		return types.ContainerJSON{}, err
	}

	return c.client.ContainerInspect(context.Background(), container.ID)
}

// Info returns info about the container, like name, command, state or ports.
func (c *Container) Info(qFlag bool) (project.Info, error) {
	container, err := c.findExisting()
	if err != nil {
		return nil, err
	}

	result := project.Info{}

	if qFlag {
		result = append(result, project.InfoPart{Key: "Id", Value: container.ID})
	} else {
		result = append(result, project.InfoPart{Key: "Name", Value: name(container.Names)})
		result = append(result, project.InfoPart{Key: "Command", Value: container.Command})
		result = append(result, project.InfoPart{Key: "State", Value: container.Status})
		result = append(result, project.InfoPart{Key: "Ports", Value: portString(container.Ports)})
	}

	return result, nil
}

func portString(ports []types.Port) string {
	result := []string{}

	for _, port := range ports {
		if port.PublicPort > 0 {
			result = append(result, fmt.Sprintf("%s:%d->%d/%s", port.IP, port.PublicPort, port.PrivatePort, port.Type))
		} else {
			result = append(result, fmt.Sprintf("%d/%s", port.PrivatePort, port.Type))
		}
	}

	return strings.Join(result, ", ")
}

func name(names []string) string {
	max := math.MaxInt32
	var current string

	for _, v := range names {
		if len(v) < max {
			max = len(v)
			current = v
		}
	}

	return current[1:]
}

func getContainerNumber(c *Container) string {
	containers, err := c.service.collectContainers()
	if err != nil {
		logrus.Errorf("Unable to collect the containers from service")
		return "1"
	}
	// Returns container count + 1
	return strconv.Itoa(len(containers) + 1)
}

// Recreate will not refresh the container by means of relaxation and enjoyment,
// just delete it and create a new one with the current configuration
func (c *Container) Recreate(imageName string) (*types.Container, error) {
	info, err := c.findInfo()
	if err != nil {
		return nil, err
	}

	hash := info.Config.Labels[HASH.Str()]
	if hash == "" {
		return nil, fmt.Errorf("Failed to find hash on old container: %s", info.Name)
	}

	name := info.Name[1:]
	newName := fmt.Sprintf("%s_%s", name, info.ID[:12])
	logrus.Debugf("Renaming %s => %s", name, newName)
	if err := c.client.ContainerRename(context.Background(), info.ID, newName); err != nil {
		logrus.Errorf("Failed to rename old container %s", c.name)
		return nil, err
	}

	newContainer, err := c.createContainer(imageName, info.ID, nil)
	if err != nil {
		return nil, err
	}
	logrus.Debugf("Created replacement container %s", newContainer.ID)

	if err := c.client.ContainerRemove(context.Background(), types.ContainerRemoveOptions{
		ContainerID:   info.ID,
		Force:         true,
		RemoveVolumes: false,
	}); err != nil {
		logrus.Errorf("Failed to remove old container %s", c.name)
		return nil, err
	}
	logrus.Debugf("Removed old container %s %s", c.name, info.ID)

	return newContainer, nil
}

// Create creates the container based on the specified image name and send an event
// to notify the container has been created. If the container already exists, does
// nothing.
func (c *Container) Create(imageName string) (*types.Container, error) {
	return c.CreateWithOverride(imageName, nil)
}

// CreateWithOverride create container and override parts of the config to
// allow special situations to override the config generated from the compose
// file
func (c *Container) CreateWithOverride(imageName string, configOverride *config.ServiceConfig) (*types.Container, error) {
	container, err := c.findExisting()
	if err != nil {
		return nil, err
	}

	if container == nil {
		container, err = c.createContainer(imageName, "", configOverride)
		if err != nil {
			return nil, err
		}
		c.service.context.Project.Notify(project.EventContainerCreated, c.service.Name(), map[string]string{
			"name": c.Name(),
		})
	}

	return container, err
}

// Stop stops the container.
func (c *Container) Stop() error {
	return c.withContainer(func(container *types.Container) error {
		return c.client.ContainerStop(context.Background(), container.ID, int(c.service.context.Timeout))
	})
}

// Down stops and remove the container.
func (c *Container) Down() error {
	if err := c.Stop(); err != nil {
		return err
	}

	return c.withContainer(func(container *types.Container) error {
		return c.client.ContainerRemove(context.Background(), types.ContainerRemoveOptions{
			ContainerID:   container.ID,
			Force:         true,
			RemoveVolumes: c.service.context.Volume,
		})
	})
}

// Pause pauses the container. If the containers are already paused, don't fail.
func (c *Container) Pause() error {
	return c.withContainer(func(container *types.Container) error {
		if !strings.Contains(container.Status, "Paused") {
			return c.client.ContainerPause(context.Background(), container.ID)
		}
		return nil
	})
}

// Unpause unpauses the container. If the containers are not paused, don't fail.
func (c *Container) Unpause() error {
	return c.withContainer(func(container *types.Container) error {
		if strings.Contains(container.Status, "Paused") {
			return c.client.ContainerUnpause(context.Background(), container.ID)
		}
		return nil
	})
}

// Kill kill the container.
func (c *Container) Kill() error {
	return c.withContainer(func(container *types.Container) error {
		return c.client.ContainerKill(context.Background(), container.ID, c.service.context.Signal)
	})
}

// Delete removes the container if existing. If the container is running, it tries
// to stop it first.
func (c *Container) Delete() error {
	container, err := c.findExisting()
	if err != nil || container == nil {
		return err
	}

	info, err := c.client.ContainerInspect(context.Background(), container.ID)
	if err != nil {
		return err
	}

	if !info.State.Running {
		return c.client.ContainerRemove(context.Background(), types.ContainerRemoveOptions{
			ContainerID:   container.ID,
			Force:         true,
			RemoveVolumes: c.service.context.Volume,
		})
	}

	return nil
}

// IsRunning returns the running state of the container.
func (c *Container) IsRunning() (bool, error) {
	container, err := c.findExisting()
	if err != nil || container == nil {
		return false, err
	}

	info, err := c.client.ContainerInspect(context.Background(), container.ID)
	if err != nil {
		return false, err
	}

	return info.State.Running, nil
}

// Run creates, start and attach to the container based on the image name,
// the specified configuration.
// It will always create a new container.
func (c *Container) Run(imageName string, configOverride *config.ServiceConfig) (int, error) {
	var (
		errCh       chan error
		out, stderr io.Writer
		in          io.ReadCloser
	)

	container, err := c.createContainer(imageName, "", configOverride)
	if err != nil {
		return -1, err
	}

	info, err := c.client.ContainerInspect(context.Background(), container.ID)
	if err != nil {
		return -1, err
	}

	if configOverride.StdinOpen {
		in = os.Stdin
	}
	if configOverride.Tty {
		out = os.Stdout
	}
	if configOverride.Tty {
		stderr = os.Stderr
	}

	options := types.ContainerAttachOptions{
		ContainerID: container.ID,
		Stream:      true,
		Stdin:       configOverride.StdinOpen,
		Stdout:      configOverride.Tty,
		Stderr:      configOverride.Tty,
	}

	resp, err := c.client.ContainerAttach(context.Background(), options)
	if err != nil {
		return -1, err
	}

	// set raw terminal
	inFd, _ := term.GetFdInfo(in)
	state, err := term.SetRawTerminal(inFd)
	if err != nil {
		return -1, err
	}
	// restore raw terminal
	defer term.RestoreTerminal(inFd, state)
	// holdHijackedConnection (in goroutine)
	errCh = promise.Go(func() error {
		return holdHijackedConnection(configOverride.Tty, in, out, stderr, resp)
	})

	if err := c.client.ContainerStart(context.Background(), container.ID); err != nil {
		return -1, err
	}

	if err := <-errCh; err != nil {
		logrus.Debugf("Error hijack: %s", err)
		return -1, err
	}

	info, err = c.client.ContainerInspect(context.Background(), container.ID)
	if err != nil {
		return -1, err
	}

	return info.State.ExitCode, nil
}

func holdHijackedConnection(tty bool, inputStream io.ReadCloser, outputStream, errorStream io.Writer, resp types.HijackedResponse) error {
	var err error
	receiveStdout := make(chan error, 1)
	if outputStream != nil || errorStream != nil {
		go func() {
			// When TTY is ON, use regular copy
			if tty && outputStream != nil {
				_, err = io.Copy(outputStream, resp.Reader)
			} else {
				_, err = stdcopy.StdCopy(outputStream, errorStream, resp.Reader)
			}
			logrus.Debugf("[hijack] End of stdout")
			receiveStdout <- err
		}()
	}

	stdinDone := make(chan struct{})
	go func() {
		if inputStream != nil {
			io.Copy(resp.Conn, inputStream)
			logrus.Debugf("[hijack] End of stdin")
		}

		if err := resp.CloseWrite(); err != nil {
			logrus.Debugf("Couldn't send EOF: %s", err)
		}
		close(stdinDone)
	}()

	select {
	case err := <-receiveStdout:
		if err != nil {
			logrus.Debugf("Error receiveStdout: %s", err)
			return err
		}
	case <-stdinDone:
		if outputStream != nil || errorStream != nil {
			if err := <-receiveStdout; err != nil {
				logrus.Debugf("Error receiveStdout: %s", err)
				return err
			}
		}
	}

	return nil
}

// Up creates and start the container based on the image name and send an event
// to notify the container has been created. If the container exists but is stopped
// it tries to start it.
func (c *Container) Up(imageName string) error {
	var err error

	defer func() {
		if err == nil && c.service.context.Log {
			go c.Log()
		}
	}()

	container, err := c.Create(imageName)
	if err != nil {
		return err
	}

	info, err := c.client.ContainerInspect(context.Background(), container.ID)
	if err != nil {
		return err
	}

	if !info.State.Running {
		c.Start(container)
	}

	return nil
}

// Start the specified container with the specified host config
func (c *Container) Start(container *types.Container) error {
	logrus.WithFields(logrus.Fields{"container.ID": container.ID, "c.name": c.name}).Debug("Starting container")
	if err := c.client.ContainerStart(context.Background(), container.ID); err != nil {
		logrus.WithFields(logrus.Fields{"container.ID": container.ID, "c.name": c.name}).Debug("Failed to start container")
		return err
	}
	c.service.context.Project.Notify(project.EventContainerStarted, c.service.Name(), map[string]string{
		"name": c.Name(),
	})
	return nil
}

// OutOfSync checks if the container is out of sync with the service definition.
// It looks if the the service hash container label is the same as the computed one.
func (c *Container) OutOfSync(imageName string) (bool, error) {
	info, err := c.findInfo()
	if err != nil {
		return false, err
	}

	if info.Config.Image != imageName {
		logrus.Debugf("Images for %s do not match %s!=%s", c.name, info.Config.Image, imageName)
		return true, nil
	}

	if info.Config.Labels[HASH.Str()] != c.getHash() {
		logrus.Debugf("Hashes for %s do not match %s!=%s", c.name, info.Config.Labels[HASH.Str()], c.getHash())
		return true, nil
	}

	image, _, err := c.client.ImageInspectWithRaw(context.Background(), info.Config.Image, false)
	if err != nil {
		if client.IsErrImageNotFound(err) {
			logrus.Debugf("Image %s do not exist, do not know if it's out of sync", info.Config.Image)
			return false, nil
		}
		return false, err
	}

	logrus.Debugf("Checking existing image name vs id: %s == %s", image.ID, info.Image)
	return image.ID != info.Image, err
}

func (c *Container) getHash() string {
	return config.GetServiceHash(c.service.Name(), c.service.Config())
}

func volumeBinds(volumes map[string]struct{}, container *types.ContainerJSON) []string {
	result := make([]string, 0, len(container.Mounts))
	for _, mount := range container.Mounts {
		if _, ok := volumes[mount.Destination]; ok {
			result = append(result, fmt.Sprint(mount.Source, ":", mount.Destination))
		}
	}
	return result
}

func (c *Container) createContainer(imageName, oldContainer string, configOverride *config.ServiceConfig) (*types.Container, error) {
	serviceConfig := c.service.serviceConfig
	if configOverride != nil {
		serviceConfig.Command = configOverride.Command
		serviceConfig.Tty = configOverride.Tty
		serviceConfig.StdinOpen = configOverride.StdinOpen
	}
	configWrapper, err := ConvertToAPI(c.service)
	if err != nil {
		return nil, err
	}

	configWrapper.Config.Image = imageName

	if configWrapper.Config.Labels == nil {
		configWrapper.Config.Labels = map[string]string{}
	}

	configWrapper.Config.Labels[SERVICE.Str()] = c.service.name
	configWrapper.Config.Labels[PROJECT.Str()] = c.service.context.Project.Name
	configWrapper.Config.Labels[HASH.Str()] = c.getHash()
	// libcompose run command not yet supported, so always "False"
	configWrapper.Config.Labels[ONEOFF.Str()] = "False"
	configWrapper.Config.Labels[NUMBER.Str()] = getContainerNumber(c)
	configWrapper.Config.Labels[VERSION.Str()] = ComposeVersion

	err = c.populateAdditionalHostConfig(configWrapper.HostConfig)
	if err != nil {
		return nil, err
	}

	if oldContainer != "" {
		info, err := c.client.ContainerInspect(context.Background(), oldContainer)
		if err != nil {
			return nil, err
		}
		configWrapper.HostConfig.Binds = util.Merge(configWrapper.HostConfig.Binds, volumeBinds(configWrapper.Config.Volumes, &info))
	}

	logrus.Debugf("Creating container %s %#v", c.name, configWrapper)

	container, err := c.client.ContainerCreate(context.Background(), configWrapper.Config, configWrapper.HostConfig, configWrapper.NetworkingConfig, c.name)
	if err != nil {
		if client.IsErrImageNotFound(err) {
			logrus.Debugf("Not Found, pulling image %s", configWrapper.Config.Image)
			if err = c.pull(configWrapper.Config.Image); err != nil {
				return nil, err
			}
			if container, err = c.client.ContainerCreate(context.Background(), configWrapper.Config, configWrapper.HostConfig, configWrapper.NetworkingConfig, c.name); err != nil {
				return nil, err
			}
		} else {
			logrus.Debugf("Failed to create container %s: %v", c.name, err)
			return nil, err
		}
	}

	return GetContainerByID(c.client, container.ID)
}

func (c *Container) populateAdditionalHostConfig(hostConfig *container.HostConfig) error {
	links := map[string]string{}

	for _, link := range c.service.DependentServices() {
		if !c.service.context.Project.Configs.Has(link.Target) {
			continue
		}

		service, err := c.service.context.Project.CreateService(link.Target)
		if err != nil {
			return err
		}

		containers, err := service.Containers()
		if err != nil {
			return err
		}

		if link.Type == project.RelTypeLink {
			c.addLinks(links, service, link, containers)
		} else if link.Type == project.RelTypeIpcNamespace {
			hostConfig, err = c.addIpc(hostConfig, service, containers)
		} else if link.Type == project.RelTypeNetNamespace {
			hostConfig, err = c.addNetNs(hostConfig, service, containers)
		}

		if err != nil {
			return err
		}
	}

	hostConfig.Links = []string{}
	for k, v := range links {
		hostConfig.Links = append(hostConfig.Links, strings.Join([]string{v, k}, ":"))
	}
	for _, v := range c.service.Config().ExternalLinks {
		hostConfig.Links = append(hostConfig.Links, v)
	}

	return nil
}

func (c *Container) addLinks(links map[string]string, service project.Service, rel project.ServiceRelationship, containers []project.Container) {
	for _, container := range containers {
		if _, ok := links[rel.Alias]; !ok {
			links[rel.Alias] = container.Name()
		}

		links[container.Name()] = container.Name()
	}
}

func (c *Container) addIpc(config *container.HostConfig, service project.Service, containers []project.Container) (*container.HostConfig, error) {
	if len(containers) == 0 {
		return nil, fmt.Errorf("Failed to find container for IPC %v", c.service.Config().Ipc)
	}

	id, err := containers[0].ID()
	if err != nil {
		return nil, err
	}

	config.IpcMode = container.IpcMode("container:" + id)
	return config, nil
}

func (c *Container) addNetNs(config *container.HostConfig, service project.Service, containers []project.Container) (*container.HostConfig, error) {
	if len(containers) == 0 {
		return nil, fmt.Errorf("Failed to find container for networks ns %v", c.service.Config().Net)
	}

	id, err := containers[0].ID()
	if err != nil {
		return nil, err
	}

	config.NetworkMode = container.NetworkMode("container:" + id)
	return config, nil
}

// ID returns the container Id.
func (c *Container) ID() (string, error) {
	container, err := c.findExisting()
	if container == nil {
		return "", err
	}
	return container.ID, err
}

// Name returns the container name.
func (c *Container) Name() string {
	return c.name
}

// Pull pulls the image the container is based on.
func (c *Container) Pull() error {
	return c.pull(c.service.serviceConfig.Image)
}

// Restart restarts the container if existing, does nothing otherwise.
func (c *Container) Restart() error {
	container, err := c.findExisting()
	if err != nil || container == nil {
		return err
	}

	return c.client.ContainerRestart(context.Background(), container.ID, int(c.service.context.Timeout))
}

// Log forwards container logs to the project configured logger.
func (c *Container) Log() error {
	container, err := c.findExisting()
	if container == nil || err != nil {
		return err
	}

	info, err := c.client.ContainerInspect(context.Background(), container.ID)
	if err != nil {
		return err
	}

	l := c.service.context.LoggerFactory.Create(c.name)

	options := types.ContainerLogsOptions{
		ContainerID: c.name,
		ShowStdout:  true,
		ShowStderr:  true,
		Follow:      true,
		Tail:        "all",
	}
	responseBody, err := c.client.ContainerLogs(context.Background(), options)
	if err != nil {
		return err
	}
	defer responseBody.Close()

	if info.Config.Tty {
		_, err = io.Copy(&logger.Wrapper{Logger: l}, responseBody)
	} else {
		_, err = stdcopy.StdCopy(&logger.Wrapper{Logger: l}, &logger.Wrapper{Logger: l, Err: true}, responseBody)
	}
	logrus.WithFields(logrus.Fields{"Logger": l, "err": err}).Debug("c.client.Logs() returned error")

	return err
}

func (c *Container) pull(image string) error {
	return pullImage(c.client, c.service, image)
}

func pullImage(client client.APIClient, service *Service, image string) error {
	tag := reference.DefaultTag
	distributionRef, err := reference.ParseNamed(image)
	if err != nil {
		return err
	}

	if named, ok := distributionRef.(reference.NamedTagged); ok {
		tag = named.Tag()
	}

	repoInfo, err := registry.ParseRepositoryInfo(distributionRef)
	if err != nil {
		return err
	}

	authConfig := types.AuthConfig{}
	if service.context.ConfigFile != nil && repoInfo != nil && repoInfo.Index != nil {
		authConfig = registry.ResolveAuthConfig(service.context.ConfigFile.AuthConfigs, repoInfo.Index)
	}

	encodedAuth, err := encodeAuthToBase64(authConfig)
	if err != nil {
		return err
	}

	options := types.ImagePullOptions{
		ImageID:      distributionRef.Name(),
		Tag:          tag,
		RegistryAuth: encodedAuth,
	}
	responseBody, err := client.ImagePull(context.Background(), options, nil)
	if err != nil {
		logrus.Errorf("Failed to pull image %s: %v", image, err)
		return err
	}
	defer responseBody.Close()

	var writeBuff io.Writer = os.Stdout

	outFd, isTerminalOut := term.GetFdInfo(os.Stdout)

	err = jsonmessage.DisplayJSONMessagesStream(responseBody, writeBuff, outFd, isTerminalOut, nil)
	if err != nil {
		if jerr, ok := err.(*jsonmessage.JSONError); ok {
			// If no error code is set, default to 1
			if jerr.Code == 0 {
				jerr.Code = 1
			}
			fmt.Fprintf(os.Stderr, "%s", writeBuff)
			return fmt.Errorf("Status: %s, Code: %d", jerr.Message, jerr.Code)
		}
	}
	return err
}

// encodeAuthToBase64 serializes the auth configuration as JSON base64 payload
func encodeAuthToBase64(authConfig types.AuthConfig) (string, error) {
	buf, err := json.Marshal(authConfig)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(buf), nil
}

func (c *Container) withContainer(action func(*types.Container) error) error {
	container, err := c.findExisting()
	if err != nil {
		return err
	}

	if container != nil {
		return action(container)
	}

	return nil
}

// Port returns the host port the specified port is mapped on.
func (c *Container) Port(port string) (string, error) {
	info, err := c.findInfo()
	if err != nil {
		return "", err
	}

	if bindings, ok := info.NetworkSettings.Ports[nat.Port(port)]; ok {
		result := []string{}
		for _, binding := range bindings {
			result = append(result, binding.HostIP+":"+binding.HostPort)
		}

		return strings.Join(result, "\n"), nil
	}
	return "", nil
}
