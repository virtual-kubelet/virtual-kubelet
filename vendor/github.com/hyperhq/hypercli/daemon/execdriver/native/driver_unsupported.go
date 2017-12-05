// +build !linux

package native

import (
	"fmt"

	"github.com/hyperhq/hypercli/daemon/execdriver"
)

// NewDriver returns a new native driver, called from NewDriver of execdriver.
func NewDriver(root string, options []string) (execdriver.Driver, error) {
	return nil, fmt.Errorf("native driver not supported on non-linux")
}
