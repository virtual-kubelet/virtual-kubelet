// +build !exclude_graphdriver_aufs,linux

package register

import (
	// register the aufs graphdriver
	_ "github.com/hyperhq/hypercli/daemon/graphdriver/aufs"
)
