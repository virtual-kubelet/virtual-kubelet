package system

import (
	"github.com/hyperhq/hyper-api/types"
	"github.com/hyperhq/hyper-api/types/events"
	"github.com/hyperhq/hyper-api/types/filters"
)

// Backend is the methods that need to be implemented to provide
// system specific functionality.
type Backend interface {
	SystemInfo() (*types.Info, error)
	SystemVersion() types.Version
	SubscribeToEvents(since, sinceNano int64, ef filters.Args) ([]events.Message, chan interface{})
	UnsubscribeFromEvents(chan interface{})
	AuthenticateToRegistry(authConfig *types.AuthConfig) (string, error)
}
