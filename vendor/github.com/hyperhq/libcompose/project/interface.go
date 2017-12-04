package project

import (
	"github.com/hyperhq/libcompose/config"
	"github.com/hyperhq/libcompose/project/events"
	"github.com/hyperhq/libcompose/project/options"
	"golang.org/x/net/context"
)

// APIProject is an interface defining the methods a libcompose project should implement.
type APIProject interface {
	events.Notifier
	events.Emitter

	Build(options options.Build, sevice ...string) error
	Create(options options.Create, services ...string) error
	Delete(options options.Delete, services ...string) error
	Down(options options.Down, services ...string) error
	Kill(signal string, services ...string) error
	Log(follow bool, services ...string) error
	Pause(services ...string) error
	Ps(onlyID bool, services ...string) (InfoSet, error)
	// FIXME(vdemeester) we could use nat.Port instead ?
	Port(index int, protocol, serviceName, privatePort string) (string, error)
	Pull(services ...string) error
	Restart(timeout int, services ...string) error
	Run(ctx context.Context, serviceName string, commandParts []string) (int, error)
	Scale(timeout int, servicesScale map[string]int) error
	Start(services ...string) error
	Stop(timeout int, services ...string) error
	Unpause(services ...string) error
	Up(options options.Up, services ...string) error

	Parse() error
	GetConfig() (*config.ServiceConfigs, map[string]*config.VolumeConfig, map[string]*config.NetworkConfig)
}
