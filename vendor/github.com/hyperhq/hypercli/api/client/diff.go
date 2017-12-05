package client

import (
	"fmt"

	"golang.org/x/net/context"

	Cli "github.com/hyperhq/hypercli/cli"
	"github.com/hyperhq/hypercli/pkg/archive"
	flag "github.com/hyperhq/hypercli/pkg/mflag"
)

// CmdDiff shows changes on a container's filesystem.
//
// Each changed file is printed on a separate line, prefixed with a single
// character that indicates the status of the file: C (modified), A (added),
// or D (deleted).
//
// Usage: docker diff CONTAINER
func (cli *DockerCli) Diff(args ...string) error {
	cmd := Cli.Subcmd("diff", []string{"CONTAINER"}, Cli.DockerCommands["diff"].Description, true)
	cmd.Require(flag.Exact, 1)

	cmd.ParseFlags(args, true)

	if cmd.Arg(0) == "" {
		return fmt.Errorf("Container name cannot be empty")
	}

	changes, err := cli.client.ContainerDiff(context.Background(), cmd.Arg(0))
	if err != nil {
		return err
	}

	for _, change := range changes {
		var kind string
		switch change.Kind {
		case archive.ChangeModify:
			kind = "C"
		case archive.ChangeAdd:
			kind = "A"
		case archive.ChangeDelete:
			kind = "D"
		}
		fmt.Fprintf(cli.out, "%s %s\n", kind, change.Path)
	}

	return nil
}
