// +build windows

package execdrivers

import (
	"github.com/hyperhq/hypercli/daemon/execdriver"
	"github.com/hyperhq/hypercli/daemon/execdriver/windows"
	"github.com/hyperhq/hypercli/pkg/sysinfo"
)

// NewDriver returns a new execdriver.Driver from the given name configured with the provided options.
func NewDriver(options []string, root, libPath string, sysInfo *sysinfo.SysInfo) (execdriver.Driver, error) {
	return windows.NewDriver(root, options)
}
