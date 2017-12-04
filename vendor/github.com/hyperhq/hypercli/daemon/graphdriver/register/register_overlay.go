// +build !exclude_graphdriver_overlay,linux

package register

import (
	// register the overlay graphdriver
	_ "github.com/hyperhq/hypercli/daemon/graphdriver/overlay"
)
