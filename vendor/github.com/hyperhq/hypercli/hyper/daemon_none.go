// +build !daemon

package main

import "github.com/hyperhq/hypercli/cli"

const daemonUsage = ""

var daemonCli cli.Handler

// notifySystem sends a message to the host when the server is ready to be used
func notifySystem() {
}
