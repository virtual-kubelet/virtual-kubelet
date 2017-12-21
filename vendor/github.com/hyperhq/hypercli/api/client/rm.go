package client

import (
	"fmt"
	"strings"

	"golang.org/x/net/context"

	"github.com/hyperhq/hyper-api/types"
	Cli "github.com/hyperhq/hypercli/cli"
	flag "github.com/hyperhq/hypercli/pkg/mflag"
)

// CmdRm removes one or more containers.
//
// Usage: docker rm [OPTIONS] CONTAINER [CONTAINER...]
func (cli *DockerCli) CmdRm(args ...string) error {
	cmd := Cli.Subcmd("rm", []string{"CONTAINER [CONTAINER...]"}, Cli.DockerCommands["rm"].Description, true)
	v := cmd.Bool([]string{"v", "-volumes"}, false, "Remove the volumes associated with the container")
	link := cmd.Bool([]string{"l", "-link"}, false, "Remove the specified link")
	force := cmd.Bool([]string{"f", "-force"}, false, "Force the removal of a running container (uses SIGKILL)")
	cmd.Require(flag.Min, 1)

	cmd.ParseFlags(args, true)
	ctx := context.Background()
	var errs []string
	for _, name := range cmd.Args() {
		if name == "" {
			return fmt.Errorf("Container name cannot be empty")
		}
		name = strings.Trim(name, "/")

		warnings, err := cli.removeContainer(ctx, name, *v, *link, *force)
		if err != nil {
			errs = append(errs, err.Error())
		} else {
			fmt.Fprintf(cli.out, "%s\n", name)
			for _, w := range warnings {
				fmt.Fprintf(cli.out, "NOTICE : %s\n", w)
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "\n"))
	}
	return nil
}

func (cli *DockerCli) removeContainer(ctx context.Context, containerID string, removeVolumes, removeLinks, force bool) ([]string, error) {
	options := types.ContainerRemoveOptions{
		RemoveVolumes: removeVolumes,
		RemoveLinks:   removeLinks,
		Force:         force,
	}
	warnings, err := cli.client.ContainerRemove(ctx, containerID, options)
	if err != nil {
		return nil, err
	}
	return warnings, nil
}
