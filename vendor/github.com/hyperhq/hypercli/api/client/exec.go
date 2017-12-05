package client

import (
	"fmt"
	"io"
	"strings"
	"time"

	"golang.org/x/net/context"

	"github.com/Sirupsen/logrus"
	"github.com/docker/engine-api/types"
	Cli "github.com/hyperhq/hypercli/cli"
	flag "github.com/hyperhq/hypercli/pkg/mflag"
	"github.com/hyperhq/hypercli/pkg/promise"
)

// CmdExec runs a command in a running container.
//
// Usage: docker exec [OPTIONS] CONTAINER COMMAND [ARG...]
func (cli *DockerCli) CmdExec(args ...string) error {
	cmd := Cli.Subcmd("exec", []string{"CONTAINER COMMAND [ARG...]"}, Cli.DockerCommands["exec"].Description, true)
	detachKeys := cmd.String([]string{}, "", "Override the key sequence for detaching a container")

	execConfig, err := ParseExec(cmd, args)
	container := cmd.Arg(0)
	// just in case the ParseExec does not exit
	if container == "" || err != nil {
		return Cli.StatusError{StatusCode: 1}
	}

	if *detachKeys != "" {
		cli.configFile.DetachKeys = *detachKeys
	}

	// Send client escape keys
	execConfig.DetachKeys = cli.configFile.DetachKeys

	ctx := context.Background()

	response, err := cli.client.ContainerExecCreate(ctx, container, *execConfig)
	if err != nil {
		return err
	}

	execID := response.ID
	if execID == "" {
		fmt.Fprintf(cli.out, "exec ID empty")
		return nil
	}

	//Temp struct for execStart so that we don't need to transfer all the execConfig
	if !execConfig.Detach {
		if err := cli.CheckTtyInput(execConfig.AttachStdin, execConfig.Tty); err != nil {
			return err
		}
	} else {
		execStartCheck := types.ExecStartCheck{
			Detach: execConfig.Detach,
			Tty:    execConfig.Tty,
		}

		if err := cli.client.ContainerExecStart(ctx, execID, execStartCheck); err != nil {
			return err
		}
		// For now don't print this - wait for when we support exec wait()
		// fmt.Fprintf(cli.out, "%s\n", execID)
		return nil
	}

	// Interactive exec requested.
	var (
		out, stderr io.Writer
		in          io.ReadCloser
		errCh       chan error
	)

	if execConfig.AttachStdin {
		in = cli.in
	}
	if execConfig.AttachStdout {
		out = cli.out
	}
	if execConfig.AttachStderr {
		if execConfig.Tty {
			stderr = cli.out
		} else {
			stderr = cli.err
		}
	}

	resp, err := cli.client.ContainerExecAttach(ctx, execID, *execConfig)
	if err != nil {
		return err
	}
	defer resp.Close()
	if in != nil && execConfig.Tty {
		if err := cli.setRawTerminal(); err != nil {
			return err
		}
		defer cli.restoreTerminal(in)
	}
	errCh = promise.Go(func() error {
		return cli.holdHijackedConnection(execConfig.Tty, in, out, stderr, resp)
	})

	if execConfig.Tty && cli.isTerminalIn {
		if err := cli.monitorTtySize(ctx, execID, true); err != nil {
			fmt.Fprintf(cli.err, "Error monitoring TTY size: %s\n", err)
		}
	}

	if err := <-errCh; err != nil {
		logrus.Debugf("Error hijack: %s", err)
		return err
	}

	var status int
	if _, status, err = getExecExitCode(ctx, cli, execID); err != nil {
		return err
	}

	if status != 0 {
		return Cli.StatusError{StatusCode: status}
	}

	return nil
}

// ParseExec parses the specified args for the specified command and generates
// an ExecConfig from it.
// If the minimal number of specified args is not right or if specified args are
// not valid, it will return an error.
func ParseExec(cmd *flag.FlagSet, args []string) (*types.ExecConfig, error) {
	var (
		flStdin      = cmd.Bool([]string{"i", "-interactive"}, false, "Keep STDIN open even if not attached")
		flTty        = cmd.Bool([]string{"t", "-tty"}, false, "Allocate a pseudo-TTY")
		flDetach     = cmd.Bool([]string{"d", "-detach"}, false, "Detached mode: run command in the background")
		flUser       = cmd.String([]string{}, "", "Username or UID (format: <name|uid>[:<group|gid>])")
		flPrivileged = cmd.Bool([]string{}, false, "Give extended privileges to the command")
		execCmd      []string
	)
	cmd.Require(flag.Min, 2)
	if err := cmd.ParseFlags(args, true); err != nil {
		return nil, err
	}
	parsedArgs := cmd.Args()
	execCmd = parsedArgs[1:]

	execConfig := &types.ExecConfig{
		User:       *flUser,
		Privileged: *flPrivileged,
		Tty:        *flTty,
		Cmd:        execCmd,
		Detach:     *flDetach,
	}

	// If -d is not set, attach to everything by default
	if !*flDetach {
		execConfig.AttachStdout = true
		execConfig.AttachStderr = true
		if *flStdin {
			execConfig.AttachStdin = true
		}
	}

	return execConfig, nil
}

func (cli *DockerCli) ExecCmd(ctx context.Context, user, contID string, cmd []string) (string, error) {
	execConfig := &types.ExecConfig{
		User:   user,
		Detach: true,
		Cmd:    cmd,
	}
	execCreateResponse, err := cli.client.ContainerExecCreate(ctx, contID, *execConfig)
	if err != nil {
		return "", err
	}
	execID := execCreateResponse.ID
	if execID == "" {
		err = fmt.Errorf("Failed to exec %s: Empty exec ID", strings.Join(cmd, " "))
		return "", err
	}
	execStartCheck := types.ExecStartCheck{Detach: execConfig.Detach}
	if err := cli.client.ContainerExecStart(ctx, execID, execStartCheck); err != nil {
		return "", err
	}

	return execID, nil
}

func (cli *DockerCli) WaitExec(ctx context.Context, execID string) error {
	for {
		running, status, err := getExecExitCode(ctx, cli, execID)
		switch {
		case err != nil:
			return err
		case running:
			time.Sleep(100 * time.Millisecond)
		case status != 0:
			err = fmt.Errorf("Failed to exec cmd: %d", status)
			return err
		case status == 0:
			return nil
		}
	}

}
