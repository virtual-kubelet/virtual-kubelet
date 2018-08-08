package client

import (
	"fmt"
	"text/tabwriter"

	"golang.org/x/net/context"

	"github.com/hyperhq/hyper-api/types"
	"github.com/hyperhq/hyper-api/types/filters"
	Cli "github.com/hyperhq/hypercli/cli"
	"github.com/hyperhq/hypercli/opts"
	flag "github.com/hyperhq/hypercli/pkg/mflag"
)

// CmdSnapshot is the parent subcommand for all snapshot commands
//
// Usage: docker snapshot <COMMAND> <OPTS>
func (cli *DockerCli) CmdSnapshot(args ...string) error {
	description := Cli.DockerCommands["snapshot"].Description + "\n\nSnapshots:\n"
	commands := [][]string{
		{"create", "Create a snapshot"},
		{"inspect", "Return low-level information on a snapshot"},
		{"ls", "List snapshots"},
		{"rm", "Remove a snapshot"},
	}

	for _, cmd := range commands {
		description += fmt.Sprintf("  %-25.25s%s\n", cmd[0], cmd[1])
	}

	description += "\nRun 'hyper snapshot COMMAND --help' for more information on a command"
	cmd := Cli.Subcmd("snapshot", []string{"[COMMAND]"}, description, false)

	cmd.Require(flag.Exact, 0)
	err := cmd.ParseFlags(args, true)
	cmd.Usage()
	return err
}

// CmdSnapshotLs outputs a list of Docker snapshots.
//
// Usage: hyper snapshot ls [OPTIONS]
func (cli *DockerCli) CmdSnapshotLs(args ...string) error {
	cmd := Cli.Subcmd("snapshot ls", nil, "List snapshots", true)

	quiet := cmd.Bool([]string{"q", "-quiet"}, false, "Only display snapshot names")
	flFilter := opts.NewListOpts(nil)
	cmd.Var(&flFilter, []string{"f", "-filter"}, "Provide filter values (i.e. 'dangling=true')")

	cmd.Require(flag.Exact, 0)
	cmd.ParseFlags(args, true)

	volFilterArgs := filters.NewArgs()
	for _, f := range flFilter.GetAll() {
		var err error
		volFilterArgs, err = filters.ParseFlag(f, volFilterArgs)
		if err != nil {
			return err
		}
	}

	snapshots, err := cli.client.SnapshotList(context.Background(), volFilterArgs)
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(cli.out, 20, 1, 3, ' ', 0)
	if !*quiet {
		for _, warn := range snapshots.Warnings {
			fmt.Fprintln(cli.err, warn)
		}
		fmt.Fprintf(w, "Snapshot Name \tVolume\tSize")
		fmt.Fprintf(w, "\n")
	}

	for _, vol := range snapshots.Snapshots {
		if *quiet {
			fmt.Fprintln(w, vol.Name)
			continue
		}
		fmt.Fprintf(w, "%s\t%s\t%d\n", vol.Name, vol.Volume, vol.Size)
	}
	w.Flush()
	return nil
}

// CmdSnapshotInspect displays low-level information on one or more snapshots.
//
// Usage: docker snapshot inspect [OPTIONS] snapshot [snapshot...]
func (cli *DockerCli) CmdSnapshotInspect(args ...string) error {
	cmd := Cli.Subcmd("snapshot inspect", []string{"snapshot [snapshot...]"}, "Return low-level information on a snapshot", true)
	tmplStr := cmd.String([]string{"f", "-format"}, "", "Format the output using the given go template")

	cmd.Require(flag.Min, 1)
	cmd.ParseFlags(args, true)

	if err := cmd.Parse(args); err != nil {
		return nil
	}

	inspectSearcher := func(name string) (interface{}, []byte, error) {
		i, err := cli.client.SnapshotInspect(context.Background(), name)
		return i, nil, err
	}

	return cli.inspectElements(*tmplStr, cmd.Args(), inspectSearcher)
}

// CmdSnapshotCreate creates a new snapshot.
//
// Usage: docker snapshot create [OPTIONS]
func (cli *DockerCli) CmdSnapshotCreate(args ...string) error {
	cmd := Cli.Subcmd("snapshot create", []string{"-v volume"}, "Create a snapshot", true)
	flForce := cmd.Bool([]string{"f", "-force"}, false, "Force to create snapshot, needed if volume is in use")
	flVolume := cmd.String([]string{"v", "-volume"}, "", "Specify volume to create snapshot")
	flName := cmd.String([]string{"-name"}, "", "Specify snapshot name")

	cmd.Require(flag.Exact, 0)
	cmd.ParseFlags(args, true)

	volReq := types.SnapshotCreateRequest{
		Name:   *flName,
		Volume: *flVolume,
		Force:  *flForce,
	}

	vol, err := cli.client.SnapshotCreate(context.Background(), volReq)
	if err != nil {
		return err
	}

	fmt.Fprintf(cli.out, "%s\n", vol.Name)
	return nil
}

// CmdSnapshotRm removes one or more snapshots.
//
// Usage: docker snapshot rm snapshot [snapshot...]
func (cli *DockerCli) CmdSnapshotRm(args ...string) error {
	cmd := Cli.Subcmd("snapshot rm", []string{"snapshot [snapshot...]"}, "Remove a snapshot", true)
	cmd.Require(flag.Min, 1)
	cmd.ParseFlags(args, true)

	var status = 0

	for _, name := range cmd.Args() {
		if err := cli.client.SnapshotRemove(context.Background(), name); err != nil {
			fmt.Fprintf(cli.err, "%s\n", err)
			status = 1
			continue
		}
		fmt.Fprintf(cli.out, "%s\n", name)
	}

	if status != 0 {
		return Cli.StatusError{StatusCode: status}
	}
	return nil
}
