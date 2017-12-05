package client

import (
	"golang.org/x/net/context"

	"github.com/docker/engine-api/types"
	Cli "github.com/hyperhq/hypercli/cli"
	flag "github.com/hyperhq/hypercli/pkg/mflag"
)

// CmdTag tags an image into a repository.
//
// Usage: docker tag [OPTIONS] IMAGE[:TAG] [REGISTRYHOST/][USERNAME/]NAME[:TAG]
func (cli *DockerCli) Tag(args ...string) error {
	cmd := Cli.Subcmd("tag", []string{"IMAGE[:TAG] [REGISTRYHOST/][USERNAME/]NAME[:TAG]"}, Cli.DockerCommands["tag"].Description, true)
	force := cmd.Bool([]string{"#f", "#-force"}, false, "Force the tagging even if there's a conflict")
	cmd.Require(flag.Exact, 2)

	cmd.ParseFlags(args, true)

	options := types.ImageTagOptions{
		Force: *force,
	}

	return cli.client.ImageTag(context.Background(), cmd.Arg(0), cmd.Arg(1), options)
}
