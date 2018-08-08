package client

import (
	"fmt"
	"io"

	"golang.org/x/net/context"

	"github.com/Sirupsen/logrus"
	"github.com/hyperhq/hyper-api/types"
	Cli "github.com/hyperhq/hypercli/cli"
	flag "github.com/hyperhq/hypercli/pkg/mflag"
	"github.com/hyperhq/hypercli/pkg/signal"
)

// CmdAttach attaches to a running container.
//
// Usage: docker attach [OPTIONS] CONTAINER
func (cli *DockerCli) CmdAttach(args ...string) error {
	cmd := Cli.Subcmd("attach", []string{"CONTAINER"}, Cli.DockerCommands["attach"].Description, true)
	noStdin := cmd.Bool([]string{"-no-stdin"}, false, "Do not attach STDIN")
	proxy := cmd.Bool([]string{}, true, "Proxy all received signals to the process")
	detachKeys := cmd.String([]string{}, "", "Override the key sequence for detaching a container")

	cmd.Require(flag.Exact, 1)

	cmd.ParseFlags(args, true)
	ctx := context.Background()
	containerID := cmd.Arg(0)

	c, err := cli.client.ContainerInspect(ctx, containerID)
	if err != nil {
		return err
	}

	if !c.State.Running {
		return fmt.Errorf("You cannot attach to a stopped container, start it first")
	}

	if c.State.Paused {
		return fmt.Errorf("You cannot attach to a paused container, unpause it first")
	}

	if err := cli.CheckTtyInput(!*noStdin, c.Config.Tty); err != nil {
		return err
	}

	if c.Config.Tty && cli.isTerminalOut {
		if err := cli.monitorTtySize(ctx, cmd.Arg(0), false); err != nil {
			logrus.Debugf("Error monitoring TTY size: %s", err)
		}
	}

	if *detachKeys != "" {
		cli.configFile.DetachKeys = *detachKeys
	}

	options := types.ContainerAttachOptions{
		Stream:     true,
		Stdin:      !*noStdin && c.Config.OpenStdin,
		Stdout:     true,
		Stderr:     true,
		DetachKeys: cli.configFile.DetachKeys,
	}

	var in io.ReadCloser
	if options.Stdin {
		in = cli.in
	}

	if *proxy && !c.Config.Tty {
		sigc := cli.forwardAllSignals(ctx, containerID)
		defer signal.StopCatch(sigc)
	}

	resp, err := cli.client.ContainerAttach(ctx, containerID, options)
	if err != nil {
		return err
	}
	defer resp.Close()
	if in != nil && c.Config.Tty {
		if err := cli.setRawTerminal(); err != nil {
			return err
		}
		defer cli.restoreTerminal(in)
	}

	if err := cli.holdHijackedConnection(c.Config.Tty, in, cli.out, cli.err, resp); err != nil {
		return err
	}

	_, status, err := getExitCode(ctx, cli, containerID)
	if err != nil {
		return err
	}
	if status != 0 {
		return Cli.StatusError{StatusCode: status}
	}

	return nil
}
