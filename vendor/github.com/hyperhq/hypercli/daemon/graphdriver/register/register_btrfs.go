// +build !exclude_graphdriver_btrfs,linux

package register

import (
	// register the btrfs graphdriver
	_ "github.com/hyperhq/hypercli/daemon/graphdriver/btrfs"
)
