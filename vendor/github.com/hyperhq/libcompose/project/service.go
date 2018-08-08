package project

import (
	"errors"

	"golang.org/x/net/context"

	"github.com/hyperhq/libcompose/config"
	"github.com/hyperhq/libcompose/project/options"
)

// Service defines what a libcompose service provides.
type Service interface {
	Info(qFlag bool) (InfoSet, error)
	Name() string
	Build(buildOptions options.Build) error
	Create(options options.Create) error
	Up(options options.Up) error
	Start() error
	Stop(timeout int) error
	Delete(options options.Delete) error
	Restart(timeout int) error
	Log(follow bool) error
	Pull() error
	Kill(signal string) error
	Config() *config.ServiceConfig
	DependentServices() []ServiceRelationship
	Containers() ([]Container, error)
	Scale(count int, timeout int) error
	Pause() error
	Unpause() error
	Run(ctx context.Context, commandParts []string) (int, error)

	RemoveImage(imageType options.ImageType) error
}

// ServiceState holds the state of a service.
type ServiceState string

// State definitions
var (
	StateExecuted = ServiceState("executed")
	StateUnknown  = ServiceState("unknown")
)

// Error definitions
var (
	ErrRestart     = errors.New("Restart execution")
	ErrUnsupported = errors.New("UnsupportedOperation")
)

// ServiceFactory is an interface factory to create Service object for the specified
// project, with the specified name and service configuration.
type ServiceFactory interface {
	Create(project *Project, name string, serviceConfig *config.ServiceConfig) (Service, error)
}

// ServiceRelationshipType defines the type of service relationship.
type ServiceRelationshipType string

// RelTypeLink means the services are linked (docker links).
const RelTypeLink = ServiceRelationshipType("")

// RelTypeNetNamespace means the services share the same network namespace.
const RelTypeNetNamespace = ServiceRelationshipType("netns")

// RelTypeIpcNamespace means the service share the same ipc namespace.
const RelTypeIpcNamespace = ServiceRelationshipType("ipc")

// RelTypeVolumesFrom means the services share some volumes.
const RelTypeVolumesFrom = ServiceRelationshipType("volumesFrom")

// ServiceRelationship holds the relationship information between two services.
type ServiceRelationship struct {
	Target, Alias string
	Type          ServiceRelationshipType
	Optional      bool
}

// NewServiceRelationship creates a new Relationship based on the specified alias
// and relationship type.
func NewServiceRelationship(nameAlias string, relType ServiceRelationshipType) ServiceRelationship {
	name, alias := NameAlias(nameAlias)
	return ServiceRelationship{
		Target: name,
		Alias:  alias,
		Type:   relType,
	}
}
