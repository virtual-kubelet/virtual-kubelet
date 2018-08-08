package client

import (
	"fmt"
	"io"

	"golang.org/x/net/context"

	"github.com/hyperhq/hyper-api/types"
	Cli "github.com/hyperhq/hypercli/cli"
	flag "github.com/hyperhq/hypercli/pkg/mflag"
	"github.com/hyperhq/hypercli/pkg/stdcopy"
)

var validDrivers = map[string]bool{
	"json-file": true,
	"journald":  true,
}

// CmdLogs fetches the logs of a given container.
//
// docker logs [OPTIONS] CONTAINER
func (cli *DockerCli) CmdLogs(args ...string) error {
	cmd := Cli.Subcmd("logs", []string{"CONTAINER"}, Cli.DockerCommands["logs"].Description, true)
	follow := cmd.Bool([]string{"f", "-follow"}, false, "Follow log output")
	since := cmd.String([]string{"-since"}, "", "Show logs since timestamp")
	times := cmd.Bool([]string{"t", "-timestamps"}, false, "Show timestamps")
	tail := cmd.String([]string{"-tail"}, "all", "Number of lines to show from the end of the logs")
	cmd.Require(flag.Exact, 1)

	cmd.ParseFlags(args, true)

	name := cmd.Arg(0)

	ctx := context.Background()
	c, err := cli.client.ContainerInspect(ctx, name)
	if err != nil {
		return err
	}

	if !validDrivers[c.HostConfig.LogConfig.Type] {
		return fmt.Errorf("\"logs\" command is supported only for \"json-file\" and \"journald\" logging drivers (got: %s)", c.HostConfig.LogConfig.Type)
	}

	options := types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Since:      *since,
		Timestamps: *times,
		Follow:     *follow,
		Tail:       *tail,
	}
	responseBody, err := cli.client.ContainerLogs(ctx, name, options)
	if err != nil {
		return err
	}
	defer responseBody.Close()

	if c.Config.Tty {
		_, err = io.Copy(cli.out, responseBody)
	} else {
		_, err = stdcopy.StdCopy(cli.out, cli.err, responseBody)
	}
	return err
}
