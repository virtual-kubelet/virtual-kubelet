// +build !windows

package windows

import (
	"fmt"

	"github.com/hyperhq/hypercli/daemon/execdriver"
)

// NewDriver returns a new execdriver.Driver
func NewDriver(root, initPath string) (execdriver.Driver, error) {
	return nil, fmt.Errorf("Windows driver not supported on non-Windows")
}
