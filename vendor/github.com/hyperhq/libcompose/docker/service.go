package docker

import (
	"fmt"
	"strings"

	"golang.org/x/net/context"

	"github.com/Sirupsen/logrus"
	"github.com/docker/engine-api/client"
	"github.com/docker/go-connections/nat"
	"github.com/docker/libcompose/config"
	"github.com/docker/libcompose/project"
	"github.com/docker/libcompose/utils"
	"github.com/docker/libcompose/yaml"
)

// Service is a project.Service implementations.
type Service struct {
	name          string
	serviceConfig *config.ServiceConfig
	context       *Context
}

// NewService creates a service
func NewService(name string, serviceConfig *config.ServiceConfig, context *Context) *Service {
	return &Service{
		name:          name,
		serviceConfig: serviceConfig,
		context:       context,
	}
}

// Name returns the service name.
func (s *Service) Name() string {
	return s.name
}

// Config returns the configuration of the service (config.ServiceConfig).
func (s *Service) Config() *config.ServiceConfig {
	return s.serviceConfig
}

// DependentServices returns the dependent services (as an array of ServiceRelationship) of the service.
func (s *Service) DependentServices() []project.ServiceRelationship {
	return project.DefaultDependentServices(s.context.Project, s)
}

// Create implements Service.Create. It ensures the image exists or build it
// if it can and then create a container.
func (s *Service) Create() error {
	containers, err := s.collectContainers()
	if err != nil {
		return err
	}

	imageName, err := s.ensureImageExists()
	if err != nil {
		return err
	}

	if len(containers) != 0 {
		return s.eachContainer(func(c *Container) error {
			return s.recreateIfNeeded(imageName, c)
		})
	}

	_, err = s.createOne(imageName)
	return err
}

func (s *Service) collectContainers() ([]*Container, error) {
	client := s.context.ClientFactory.Create(s)
	containers, err := GetContainersByFilter(client, SERVICE.Eq(s.name), PROJECT.Eq(s.context.Project.Name))
	if err != nil {
		return nil, err
	}

	result := []*Container{}

	for _, container := range containers {
		// Compose add "/" before name, so Name[1] will store actaul name.
		name := strings.SplitAfter(container.Names[0], "/")
		result = append(result, NewContainer(client, name[1], s))
	}

	return result, nil
}

func (s *Service) createOne(imageName string) (*Container, error) {
	containers, err := s.constructContainers(imageName, 1)
	if err != nil {
		return nil, err
	}

	return containers[0], err
}

func (s *Service) ensureImageExists() (string, error) {
	err := s.imageExists()

	if err == nil {
		return s.imageName(), nil
	}

	if err != nil && !client.IsErrImageNotFound(err) {
		return "", err
	}

	if s.Config().Build != "" {
		if s.context.NoBuild {
			return "", fmt.Errorf("Service %q needs to be built, but no-build was specified", s.name)
		}
		return s.imageName(), s.build()
	}

	return s.imageName(), s.Pull()
}

func (s *Service) imageExists() error {
	client := s.context.ClientFactory.Create(s)

	_, _, err := client.ImageInspectWithRaw(context.Background(), s.imageName(), false)
	return err
}

func (s *Service) imageName() string {
	if s.Config().Image != "" {
		return s.Config().Image
	}
	return fmt.Sprintf("%s_%s", s.context.ProjectName, s.Name())
}

// Build implements Service.Build. If an imageName is specified or if the context has
// no build to work with it will do nothing. Otherwise it will try to build
// the image and returns an error if any.
func (s *Service) Build() error {
	if s.Config().Image != "" {
		return nil
	}
	return s.build()
}

func (s *Service) build() error {
	if s.context.Builder == nil {
		return fmt.Errorf("Cannot build an image without a builder configured")
	}

	return s.context.Builder.Build(s.imageName(), s.context.Project, s)
}

func (s *Service) constructContainers(imageName string, count int) ([]*Container, error) {
	result, err := s.collectContainers()
	if err != nil {
		return nil, err
	}

	client := s.context.ClientFactory.Create(s)

	var namer Namer

	if s.serviceConfig.ContainerName != "" {
		if count > 1 {
			logrus.Warnf(`The "%s" service is using the custom container name "%s". Docker requires each container to have a unique name. Remove the custom name to scale the service.`, s.name, s.serviceConfig.ContainerName)
		}
		namer = NewSingleNamer(s.serviceConfig.ContainerName)
	} else {
		namer = NewNamer(client, s.context.Project.Name, s.name)
	}

	defer namer.Close()

	for i := len(result); i < count; i++ {
		containerName := namer.Next()

		c := NewContainer(client, containerName, s)

		dockerContainer, err := c.Create(imageName)
		if err != nil {
			return nil, err
		}

		logrus.Debugf("Created container %s: %v", dockerContainer.ID, dockerContainer.Names)

		result = append(result, NewContainer(client, containerName, s))
	}

	return result, nil
}

// Up implements Service.Up. It builds the image if needed, creates a container
// and start it.
func (s *Service) Up() error {
	containers, err := s.collectContainers()
	if err != nil {
		return err
	}

	var imageName = s.imageName()
	if len(containers) == 0 || !s.context.NoRecreate {
		imageName, err = s.ensureImageExists()
		if err != nil {
			return err
		}
	}

	return s.up(imageName, true)
}

// Run implements Service.Run. It runs a one of command within the service container.
func (s *Service) Run(commandParts []string) (int, error) {
	imageName, err := s.ensureImageExists()
	if err != nil {
		return -1, err
	}

	client := s.context.ClientFactory.Create(s)

	namer := NewNamer(client, s.context.Project.Name, s.name+"_run")
	defer namer.Close()

	containerName := namer.Next()

	c := NewContainer(client, containerName, s)

	return c.Run(imageName, &config.ServiceConfig{Command: yaml.NewCommand(commandParts...), Tty: true, StdinOpen: true})
}

// Info implements Service.Info. It returns an project.InfoSet with the containers
// related to this service (can be multiple if using the scale command).
func (s *Service) Info(qFlag bool) (project.InfoSet, error) {
	result := project.InfoSet{}
	containers, err := s.collectContainers()
	if err != nil {
		return nil, err
	}

	for _, c := range containers {
		info, err := c.Info(qFlag)
		if err != nil {
			return nil, err
		}
		result = append(result, info)
	}

	return result, nil
}

// Start implements Service.Start. It tries to start a container without creating it.
func (s *Service) Start() error {
	return s.up("", false)
}

func (s *Service) up(imageName string, create bool) error {
	containers, err := s.collectContainers()
	if err != nil {
		return err
	}

	logrus.Debugf("Found %d existing containers for service %s", len(containers), s.name)

	if len(containers) == 0 && create {
		c, err := s.createOne(imageName)
		if err != nil {
			return err
		}
		containers = []*Container{c}
	}

	return s.eachContainer(func(c *Container) error {
		if create {
			if err := s.recreateIfNeeded(imageName, c); err != nil {
				return err
			}
		}

		return c.Up(imageName)
	})
}

func (s *Service) recreateIfNeeded(imageName string, c *Container) error {
	if s.context.NoRecreate {
		return nil
	}
	outOfSync, err := c.OutOfSync(imageName)
	if err != nil {
		return err
	}

	logrus.WithFields(logrus.Fields{
		"outOfSync":     outOfSync,
		"ForceRecreate": s.context.ForceRecreate,
		"NoRecreate":    s.context.NoRecreate}).Debug("Going to decide if recreate is needed")

	if s.context.ForceRecreate || outOfSync {
		logrus.Infof("Recreating %s", s.name)
		if _, err := c.Recreate(imageName); err != nil {
			return err
		}
	}

	return nil
}

func (s *Service) eachContainer(action func(*Container) error) error {
	containers, err := s.collectContainers()
	if err != nil {
		return err
	}

	tasks := utils.InParallel{}
	for _, container := range containers {
		task := func(container *Container) func() error {
			return func() error {
				return action(container)
			}
		}(container)

		tasks.Add(task)
	}

	return tasks.Wait()
}

// Stop implements Service.Stop. It stops any containers related to the service.
func (s *Service) Stop() error {
	return s.eachContainer(func(c *Container) error {
		return c.Stop()
	})
}

// Down implements Service.Down. It stops any containers related to the service and removes them.
func (s *Service) Down() error {
	return s.eachContainer(func(c *Container) error {
		return c.Down()
	})
}

// Restart implements Service.Restart. It restarts any containers related to the service.
func (s *Service) Restart() error {
	return s.eachContainer(func(c *Container) error {
		return c.Restart()
	})
}

// Kill implements Service.Kill. It kills any containers related to the service.
func (s *Service) Kill() error {
	return s.eachContainer(func(c *Container) error {
		return c.Kill()
	})
}

// Delete implements Service.Delete. It removes any containers related to the service.
func (s *Service) Delete() error {
	return s.eachContainer(func(c *Container) error {
		return c.Delete()
	})
}

// Log implements Service.Log. It returns the docker logs for each container related to the service.
func (s *Service) Log() error {
	return s.eachContainer(func(c *Container) error {
		return c.Log()
	})
}

// Scale implements Service.Scale. It creates or removes containers to have the specified number
// of related container to the service to run.
func (s *Service) Scale(scale int) error {
	if s.specificiesHostPort() {
		logrus.Warnf("The \"%s\" service specifies a port on the host. If multiple containers for this service are created on a single host, the port will clash.", s.Name())
	}

	foundCount := 0
	err := s.eachContainer(func(c *Container) error {
		foundCount++
		if foundCount > scale {
			err := c.Down()
			if err != nil {
				return err
			}

			return c.Delete()
		}
		return nil
	})

	if err != nil {
		return err
	}

	if foundCount != scale {
		imageName, err := s.ensureImageExists()
		if err != nil {
			return err
		}

		if _, err = s.constructContainers(imageName, scale); err != nil {
			return err
		}
	}

	return s.up("", false)
}

// Pull implements Service.Pull. It pulls the image of the service and skip the service that
// would need to be built.
func (s *Service) Pull() error {
	if s.Config().Image == "" {
		return nil
	}

	return pullImage(s.context.ClientFactory.Create(s), s, s.Config().Image)
}

// Pause implements Service.Pause. It puts into pause the container(s) related
// to the service.
func (s *Service) Pause() error {
	return s.eachContainer(func(c *Container) error {
		return c.Pause()
	})
}

// Unpause implements Service.Pause. It brings back from pause the container(s)
// related to the service.
func (s *Service) Unpause() error {
	return s.eachContainer(func(c *Container) error {
		return c.Unpause()
	})
}

// Containers implements Service.Containers. It returns the list of containers
// that are related to the service.
func (s *Service) Containers() ([]project.Container, error) {
	result := []project.Container{}
	containers, err := s.collectContainers()
	if err != nil {
		return nil, err
	}

	for _, c := range containers {
		result = append(result, c)
	}

	return result, nil
}

func (s *Service) specificiesHostPort() bool {
	_, bindings, err := nat.ParsePortSpecs(s.Config().Ports)

	if err != nil {
		fmt.Println(err)
	}

	for _, portBindings := range bindings {
		for _, portBinding := range portBindings {
			if portBinding.HostPort != "" {
				return true
			}
		}
	}

	return false
}
