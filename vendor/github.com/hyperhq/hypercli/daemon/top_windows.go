package daemon

import (
	"github.com/hyperhq/hyper-api/types"
	derr "github.com/hyperhq/hypercli/errors"
)

// ContainerTop is not supported on Windows and returns an error.
func (daemon *Daemon) ContainerTop(name string, psArgs string) (*types.ContainerProcessList, error) {
	return nil, derr.ErrorCodeNoTop
}
