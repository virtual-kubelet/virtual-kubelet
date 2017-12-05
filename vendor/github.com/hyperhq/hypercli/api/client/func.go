package client

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/container"
	"github.com/docker/engine-api/types/filters"
	"github.com/docker/engine-api/types/network"
	"github.com/docker/engine-api/types/strslice"
	"github.com/docker/go-connections/nat"
	units "github.com/docker/go-units"
	Cli "github.com/hyperhq/hypercli/cli"
	ropts "github.com/hyperhq/hypercli/opts"
	flag "github.com/hyperhq/hypercli/pkg/mflag"
	"github.com/hyperhq/hypercli/pkg/signal"
	"github.com/hyperhq/hypercli/runconfig/opts"
	"golang.org/x/net/context"
)

// CmdFunc is the parent subcommand for all func commands
//
// Usage: docker func <COMMAND> [OPTIONS]
func (cli *DockerCli) CmdFunc(args ...string) error {
	cmd := Cli.Subcmd("func", []string{"COMMAND [OPTIONS]"}, funcUsage(), false)
	cmd.Require(flag.Min, 1)
	err := cmd.ParseFlags(args, true)
	cmd.Usage()
	return err
}

func funcUsage() string {
	funcCommands := [][]string{
		{"create", "Create a new function"},
		{"update", "Update a function"},
		{"ls", "Lists all functions"},
		{"rm", "Remove one or more function"},
		{"inspect", "Display detailed information on the given function"},
		{"call", "Call a function"},
		{"get", "Get the return of a function call"},
		{"logs", "Retrieve the logs of a function"},
		{"status", "Retrieve the status of a function"},
	}

	help := "Commands:\n"

	for _, cmd := range funcCommands {
		help += fmt.Sprintf("  %-25.25s%s\n", cmd[0], cmd[1])
	}

	help += fmt.Sprintf("\nRun 'hyper func COMMAND --help' for more information on a command.")
	return help
}

// CmdFuncCreate creates a new func with a given name
//
// Usage: hyper func create [OPTIONS] IMAGE [COMMAND]
func (cli *DockerCli) CmdFuncCreate(args ...string) error {
	cmd := Cli.Subcmd("func create", []string{"IMAGE [COMMAND] [ARG...]"}, "Create a new function", false)
	var (
		flName          = cmd.String([]string{"-name"}, "", "Function name")
		flContainerSize = cmd.String([]string{"-size"}, "s4", "The size of function containers to run the funciton (e.g. s1, s2, s3, s4, m1, m2, m3, l1, l2, l3)")
		flTimeout       = cmd.Int([]string{"-timeout"}, 300, "The maximum execution duration of function call")

		flEnv     = ropts.NewListOpts(opts.ValidateEnv)
		flEnvFile = ropts.NewListOpts(nil)

		flLabels     = ropts.NewListOpts(opts.ValidateEnv)
		flLabelsFile = ropts.NewListOpts(nil)

		flVolumesFrom  = ropts.NewListOpts(nil)
		flNoAutoVolume = cmd.Bool([]string{"-noauto-volume"}, false, "Do not create volumes specified in image")

		flPublish    = ropts.NewListOpts(nil)
		flExpose     = ropts.NewListOpts(nil)
		flPublishAll = cmd.Bool([]string{"P", "-publish-all"}, false, "Publish all exposed ports to random ports")

		flEntrypoint = cmd.String([]string{"-entrypoint"}, "", "Overwrite the default ENTRYPOINT of the image")
		flWorkingDir = cmd.String([]string{"w", "-workdir"}, "", "Working directory inside the container")

		flTty            = cmd.Bool([]string{"t", "-tty"}, false, "Allocate a pseudo-TTY")
		flLinks          = ropts.NewListOpts(opts.ValidateLink)
		flSecurityGroups = ropts.NewListOpts(nil)
		flStopSignal     = cmd.String([]string{"-stop-signal"}, signal.DefaultStopSignal, fmt.Sprintf("Signal to stop a container, %v by default", signal.DefaultStopSignal))
		flNetMode        = cmd.String([]string{}, "bridge", "Connect containers to a network, only bridge is supported now")
	)
	cmd.Var(&flLabels, []string{"l", "-label"}, "Set meta data on a container")
	cmd.Var(&flLabelsFile, []string{"-label-file"}, "Read in a line delimited file of labels")
	cmd.Var(&flEnv, []string{"e", "-env"}, "Set environment variables")
	cmd.Var(&flEnvFile, []string{"-env-file"}, "Read in a file of environment variables")
	cmd.Var(&flSecurityGroups, []string{"-sg"}, "Security group for each container")
	cmd.Var(&flLinks, []string{"-link"}, "Add link to another container")
	cmd.Var(&flPublish, []string{"p", "-publish"}, "Publish a container's port(s) to the host")
	cmd.Var(&flExpose, []string{"-expose"}, "Expose a port or a range of ports")
	cmd.Var(&flVolumesFrom, []string{"-volumes-from"}, "Mount shared volumes from the specified container(s)")

	cmd.Require(flag.Min, 1)
	err := cmd.ParseFlags(args, true)
	if err != nil {
		return err
	}

	var (
		parsedArgs = cmd.Args()
		runCmd     strslice.StrSlice
		entrypoint strslice.StrSlice
		image      = cmd.Arg(0)
	)
	if len(parsedArgs) > 1 {
		runCmd = strslice.StrSlice(parsedArgs[1:])
	}
	if *flEntrypoint != "" {
		entrypoint = strslice.StrSlice{*flEntrypoint}
	}

	// collect all the environment variables for the container
	envVariables, err := opts.ReadKVStrings(flEnvFile.GetAll(), flEnv.GetAll())
	if err != nil {
		return err
	}

	// collect all the labels for the container
	labels, err := opts.ReadKVStrings(flLabelsFile.GetAll(), flLabels.GetAll())
	if err != nil {
		return err
	}
	for _, sg := range flSecurityGroups.GetAll() {
		if sg == "" {
			continue
		}
		labels = append(labels, fmt.Sprintf("sh_hyper_sg_%s=yes", sg))
	}
	if *flNoAutoVolume {
		labels = append(labels, "sh_hyper_noauto_volume=true")
	}

	ports, portBindings, err := nat.ParsePortSpecs(flPublish.GetAll())
	if err != nil {
		return err
	}

	// Merge in exposed ports to the map of published ports
	for _, e := range flExpose.GetAll() {
		if strings.Contains(e, ":") {
			return fmt.Errorf("Invalid port format for --expose: %s", e)
		}
		//support two formats for expose, original format <portnum>/[<proto>] or <startport-endport>/[<proto>]
		proto, port := nat.SplitProtoPort(e)
		//parse the start and end port and create a sequence of ports to expose
		//if expose a port, the start and end port are the same
		start, end, err := nat.ParsePortRange(port)
		if err != nil {
			return fmt.Errorf("Invalid range format for --expose: %s, error: %s", e, err)
		}
		for i := start; i <= end; i++ {
			p, err := nat.NewPort(proto, strconv.FormatUint(i, 10))
			if err != nil {
				return err
			}
			if _, exists := ports[p]; !exists {
				ports[p] = struct{}{}
			}
		}
	}

	config := types.FuncConfig{
		Tty:          *flTty,
		ExposedPorts: ports,
		Env:          &envVariables,
		Cmd:          runCmd,
		Image:        image,
		Entrypoint:   entrypoint,
		WorkingDir:   *flWorkingDir,
		Labels:       opts.ConvertKVStringsToMap(labels),
		StopSignal:   *flStopSignal,
	}

	hostConfig := types.FuncHostConfig{
		VolumesFrom:     flVolumesFrom.GetAll(),
		PortBindings:    portBindings,
		Links:           flLinks.GetAll(),
		PublishAllPorts: *flPublishAll,
		NetworkMode:     container.NetworkMode(*flNetMode),
	}
	networkingConfig := network.NetworkingConfig{
		EndpointsConfig: make(map[string]*network.EndpointSettings),
	}

	if hostConfig.NetworkMode.IsUserDefined() && len(hostConfig.Links) > 0 {
		epConfig := networkingConfig.EndpointsConfig[string(hostConfig.NetworkMode)]
		if epConfig == nil {
			epConfig = &network.EndpointSettings{}
		}
		epConfig.Links = make([]string, len(hostConfig.Links))
		copy(epConfig.Links, hostConfig.Links)
		networkingConfig.EndpointsConfig[string(hostConfig.NetworkMode)] = epConfig
	}

	fnOpts := types.Func{
		Name:          *flName,
		ContainerSize: *flContainerSize,
		Timeout:       *flTimeout,

		Config:           config,
		HostConfig:       hostConfig,
		NetworkingConfig: networkingConfig,
	}

	fn, err := cli.client.FuncCreate(context.Background(), fnOpts)
	if err != nil {
		return err
	}
	fmt.Fprintf(cli.out, "%s is created with the address of https://%s.hyperfunc.io/call/%s/%s\n", fn.Name, cli.region, fn.Name, fn.UUID)
	return nil
}

// CmdFuncUpdate updates a func with a given name
//
// Usage: hyper func update [OPTIONS] NAME
func (cli *DockerCli) CmdFuncUpdate(args ...string) error {
	cmd := Cli.Subcmd("func update", []string{"NAME"}, "Update a function", false)
	var (
		flContainerSize = cmd.String([]string{"-size"}, "", "The size of function containers to run the funciton (e.g. s1, s2, s3, s4, m1, m2, m3, l1, l2, l3)")
		flEnv           = ropts.NewListOpts(opts.ValidateEnv)
		flEnvFile       = ropts.NewListOpts(nil)
		flRefresh       = cmd.Bool([]string{"-refresh"}, false, "Whether to regenerate the uuid of function")
		flTimeout       = cmd.String([]string{"-timeout"}, "", "The maximum execution duration of function call")
	)
	cmd.Var(&flEnv, []string{"e", "-env"}, "Set environment variables")
	cmd.Var(&flEnvFile, []string{"-env-file"}, "Read in a file of environment variables")

	cmd.Require(flag.Exact, 1)
	err := cmd.ParseFlags(args, true)
	if err != nil {
		return err
	}

	name := cmd.Arg(0)
	name = strings.Replace(name, "/", "", -1)

	// collect all the environment variables for the container
	envVariables, err := opts.ReadKVStrings(flEnvFile.GetAll(), flEnv.GetAll())
	if err != nil {
		return err
	}
	for _, env := range envVariables {
		if env == "" {
			envVariables = []string{}
			break
		}
	}
	env := &envVariables
	if !cmd.IsSet("-env") && !cmd.IsSet("e") && !cmd.IsSet("-env-file") {
		env = nil
	}

	timeout, _ := strconv.Atoi(*flTimeout)

	fnOpts := types.Func{
		Name:          name,
		ContainerSize: *flContainerSize,
		Refresh:       *flRefresh,
		Timeout:       timeout,
		Config: types.FuncConfig{
			Env: env,
		},
	}

	fn, err := cli.client.FuncUpdate(context.Background(), name, fnOpts)
	if err != nil {
		return err
	}
	fmt.Fprintf(cli.out, "%s\n", fn.Name)
	return nil
}

// CmdFuncDelete deletes one or more funcs
//
// Usage: hyper func rm NAME [NAME...]
func (cli *DockerCli) CmdFuncRm(args ...string) error {
	cmd := Cli.Subcmd("func rm", []string{"NAME [NAME...]"}, "Remove one or more function", false)
	cmd.Require(flag.Min, 1)
	if err := cmd.ParseFlags(args, true); err != nil {
		return err
	}

	status := 0
	for _, name := range cmd.Args() {
		name = strings.Replace(name, "/", "", -1)
		if err := cli.client.FuncDelete(context.Background(), name); err != nil {
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

// CmdFuncLs lists all the funcs
//
// Usage: hyper func ls [OPTIONS]
func (cli *DockerCli) CmdFuncLs(args ...string) error {
	cmd := Cli.Subcmd("func ls", nil, "Lists all functions", true)

	flFilter := ropts.NewListOpts(nil)
	cmd.Var(&flFilter, []string{"f", "-filter"}, "Filter output based on conditions provided")

	cmd.Require(flag.Exact, 0)
	err := cmd.ParseFlags(args, true)
	if err != nil {
		return err
	}

	// Consolidate all filter flags, and sanity check them early.
	// They'll get process after get response from server.
	funcFilterArgs := filters.NewArgs()
	for _, f := range flFilter.GetAll() {
		if funcFilterArgs, err = filters.ParseFlag(f, funcFilterArgs); err != nil {
			return err
		}
	}

	options := types.FuncListOptions{
		Filters: funcFilterArgs,
	}

	fns, err := cli.client.FuncList(context.Background(), options)
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(cli.out, 20, 1, 3, ' ', 0)
	fmt.Fprintf(w, "NAME\tSIZE\tIMAGE\tCOMMAND\tCREATED\tUUID\n")
	for _, fn := range fns {
		created := units.HumanDuration(time.Now().UTC().Sub(fn.Created)) + " ago"
		command := strings.Join([]string(fn.Config.Cmd), " ")
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", fn.Name, fn.ContainerSize, fn.Config.Image, command, created, fn.UUID)
	}
	w.Flush()

	return nil
}

// CmdFuncInspect
//
// Usage: docker func inspect [OPTIONS] NAME [NAME...]
func (cli *DockerCli) CmdFuncInspect(args ...string) error {
	cmd := Cli.Subcmd("func inspect", []string{"NAME [NAME...]"}, "Display detailed information on the given function", true)
	tmplStr := cmd.String([]string{"f", "-format"}, "", "Format the output using the given go template")

	cmd.Require(flag.Min, 1)
	cmd.ParseFlags(args, true)

	if err := cmd.Parse(args); err != nil {
		return nil
	}

	ctx := context.Background()

	inspectSearcher := func(name string) (interface{}, []byte, error) {
		name = strings.Replace(name, "/", "", -1)
		i, err := cli.client.FuncInspect(ctx, name)
		return i, nil, err
	}

	return cli.inspectElements(*tmplStr, cmd.Args(), inspectSearcher)
}

// CmdFuncCall call a func
//
// Usage: hyper func call NAME
func (cli *DockerCli) CmdFuncCall(args ...string) error {
	cmd := Cli.Subcmd("func call", []string{"NAME"}, "Call a function", false)
	sync := cmd.Bool([]string{"-sync"}, false, "Block until the call is completed")

	cmd.Require(flag.Exact, 1)
	if err := cmd.ParseFlags(args, true); err != nil {
		return err
	}

	name := cmd.Arg(0)
	name = strings.Replace(name, "/", "", -1)

	var stdin io.Reader
	if fi, err := os.Stdin.Stat(); err == nil {
		if fi.Mode()&os.ModeNamedPipe != 0 {
			stdin = bufio.NewReader(os.Stdin)
		}
	}

	body, err := cli.client.FuncCall(context.Background(), cli.region, name, stdin, *sync)
	if err != nil {
		return err
	}
	defer body.Close()

	if *sync {
		_, err = io.Copy(cli.out, body)
		return err
	}

	var ret types.FuncCallResponse
	err = json.NewDecoder(body).Decode(&ret)
	if err != nil {
		return err
	}
	fmt.Fprintf(cli.out, "CallId: %s\n", ret.CallId)

	return nil
}

// CmdFuncGet Get the return of a func call
//
// Usage: hyper func get [OPTIONS] CALL_ID
func (cli *DockerCli) CmdFuncGet(args ...string) error {
	cmd := Cli.Subcmd("func get", []string{"CALL_ID"}, "Get the return of a function call", false)
	wait := cmd.Bool([]string{"-wait"}, false, "Block until the call is completed")

	cmd.Require(flag.Exact, 1)
	if err := cmd.ParseFlags(args, true); err != nil {
		return err
	}

	callId := cmd.Arg(0)

	body, err := cli.client.FuncGet(context.Background(), cli.region, callId, *wait)
	if err != nil {
		return err
	}
	defer body.Close()

	_, err = io.Copy(cli.out, body)
	return err
}

// CmdFuncLogs Get the return of a func call
//
// Usage: hyper func get [OPTIONS] NAME
func (cli *DockerCli) CmdFuncLogs(args ...string) error {
	cmd := Cli.Subcmd("func logs", []string{"NAME"}, "Retrieve the logs of a function", false)

	follow := cmd.Bool([]string{"f", "-follow"}, false, "Follow log output")
	tail := cmd.String([]string{"-tail"}, "all", "Number of lines to show from the end of the logs")
	callId := cmd.String([]string{"-callid"}, "", "Only retrieve specific logs of CallId")

	cmd.Require(flag.Exact, 1)
	if err := cmd.ParseFlags(args, true); err != nil {
		return err
	}

	name := cmd.Arg(0)
	name = strings.Replace(name, "/", "", -1)

	reader, err := cli.client.FuncLogs(context.Background(), cli.region, name, *callId, *follow, *tail)
	if err != nil {
		return err
	}
	defer reader.Close()
	dec := json.NewDecoder(reader)
	for {
		var log types.FuncLogsResponse
		err := dec.Decode(&log)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		if log.Event != "" {
			logTime := log.Time.Local().Format("2006-01-02T15:04:05Z")
			if log.Event == "CALL" {
				fmt.Fprintf(
					cli.out, "%s [%s] CallId: %s, ShortStdin: %s\n",
					logTime, log.Event, log.CallId, log.ShortStdin,
				)
			} else if log.Event == "FINISHED" {
				fmt.Fprintf(
					cli.out, "%s [%s] CallId: %s, ShortStdout: %s, ShortStderr: %s\n",
					logTime, log.Event, log.CallId, log.ShortStdout, log.ShortStderr,
				)
			} else if log.Event == "FAILED" {
				fmt.Fprintf(
					cli.out, "%s [%s] CallId: %s, Message: %s\n",
					logTime, log.Event, log.CallId, log.Message,
				)
			} else {
				fmt.Fprintf(
					cli.out, "%s [%s] CallId: %s\n",
					logTime, log.Event, log.CallId,
				)
			}
		}
	}
}

// CmdFuncStatus Status the return of a func call
//
// Usage: hyper func status [OPTIONS] NAME
func (cli *DockerCli) CmdFuncStatus(args ...string) error {
	cmd := Cli.Subcmd("func status", []string{"NAME"}, "Retrieve the status of a function", false)

	cmd.Require(flag.Exact, 1)
	if err := cmd.ParseFlags(args, true); err != nil {
		return err
	}

	name := cmd.Arg(0)
	name = strings.Replace(name, "/", "", -1)

	status, err := cli.client.FuncStatus(context.Background(), cli.region, name)
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(cli.out, 20, 1, 3, ' ', 0)
	fmt.Fprintf(w, "TOTAL\tPENDING\tRUNNING\tFINISHED\tFAILED\n")
	fmt.Fprintf(w, "%d\t%d\t%d\t%d\t%d\n", status.Total, status.Pending, status.Running, status.Finished, status.Failed)
	w.Flush()

	return nil
}
