package client

import (
	"fmt"
	"strings"

	"golang.org/x/net/context"

	Cli "github.com/hyperhq/hypercli/cli"
	flag "github.com/hyperhq/hypercli/pkg/mflag"
)

// CmdPause pauses all processes within one or more containers.
//
// Usage: docker pause CONTAINER [CONTAINER...]
func (cli *DockerCli) Pause(args ...string) error {
	cmd := Cli.Subcmd("pause", []string{"CONTAINER [CONTAINER...]"}, Cli.DockerCommands["pause"].Description, true)
	cmd.Require(flag.Min, 1)

	cmd.ParseFlags(args, true)

	ctx := context.Background()

	var errs []string
	for _, name := range cmd.Args() {
		if err := cli.client.ContainerPause(ctx, name); err != nil {
			errs = append(errs, err.Error())
		} else {
			fmt.Fprintf(cli.out, "%s\n", name)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "\n"))
	}
	return nil
}
