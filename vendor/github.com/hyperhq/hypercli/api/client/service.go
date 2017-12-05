package client

import (
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/filters"
	"github.com/docker/engine-api/types/strslice"
	Cli "github.com/hyperhq/hypercli/cli"
	ropts "github.com/hyperhq/hypercli/opts"
	flag "github.com/hyperhq/hypercli/pkg/mflag"
	"github.com/hyperhq/hypercli/pkg/signal"
	"github.com/hyperhq/hypercli/runconfig/opts"
	"golang.org/x/net/context"
)

// CmdService is the parent subcommand for all service commands
//
// Usage: docker service <COMMAND> [OPTIONS]
func (cli *DockerCli) CmdService(args ...string) error {
	cmd := Cli.Subcmd("service", []string{"COMMAND [OPTIONS]"}, serviceUsage(), false)
	cmd.Require(flag.Min, 1)
	err := cmd.ParseFlags(args, true)
	cmd.Usage()
	return err
}

// CmdServiceCreate creates a new service with a given name
//
// Usage: hyper service create [OPTIONS] COUNT
func (cli *DockerCli) CmdServiceCreate(args ...string) error {
	cmd := Cli.Subcmd("service create", []string{"IMAGE"}, "Create a new service", false)
	var (
		flSecurityGroups = ropts.NewListOpts(nil)
		flEnv            = ropts.NewListOpts(opts.ValidateEnv)
		flLabels         = ropts.NewListOpts(opts.ValidateEnv)
		flEnvFile        = ropts.NewListOpts(nil)
		flVolumes        = ropts.NewListOpts(nil)
		flLabelsFile     = ropts.NewListOpts(nil)

		flName                = cmd.String([]string{"-name"}, "", "Service name")
		flStdin               = cmd.Bool([]string{"i", "-interactive"}, false, "Keep STDIN open even if not attached")
		flTty                 = cmd.Bool([]string{"t", "-tty"}, false, "Allocate a pseudo-TTY")
		flEntrypoint          = cmd.String([]string{"-entrypoint"}, "", "Overwrite the default ENTRYPOINT of the image")
		flNetMode             = cmd.String([]string{}, "bridge", "Connect containers to a network, only bridge is supported now")
		flStopSignal          = cmd.String([]string{"-stop-signal"}, signal.DefaultStopSignal, fmt.Sprintf("Signal to stop a container, %v by default", signal.DefaultStopSignal))
		flContainerSize       = cmd.String([]string{"-size"}, "s4", "The size of service containers (e.g. s1, s2, s3, s4, m1, m2, m3, l1, l2, l3)")
		flWorkingDir          = cmd.String([]string{"w", "-workdir"}, "", "Working directory inside the container")
		flSSLCert             = cmd.String([]string{"-ssl-cert"}, "", "SSL cert file for httpsTerm service")
		flServicePort         = cmd.Int([]string{"-service-port"}, 0, "Publish port of the service")
		flContainerPort       = cmd.Int([]string{"-container-port"}, 0, "Container port of the service, default same with service port")
		flReplicas            = cmd.Int([]string{"-replicas"}, -1, "Number of containers belonging to this service")
		flHealthCheckInterval = cmd.Int([]string{"-health-check-interval"}, 3, "Interval in seconds for health checking the containers")
		flHealthCheckFall     = cmd.Int([]string{"-health-check-fall"}, 3, "Number of consecutive valid health checks before considering the server as DOWN")
		flHealthCheckRise     = cmd.Int([]string{"-health-check-rise"}, 2, "Number of consecutive valid health checks before considering the server as UP")
		flSessionAffinity     = cmd.Bool([]string{"-session-affinity"}, false, "Whether the service uses sticky sessions")
		flAlgorithm           = cmd.String([]string{"-algorithm"}, types.LBAlgorithmRoundRobin, "Algorithm of the service (e.g. roundrobin, leastconn, source)")
		flProtocol            = cmd.String([]string{"-protocol"}, types.LBProtocolTCP, "Protocol of the service (e.g. http, https, tcp, httpsTerm).")
	)
	cmd.Var(&flLabels, []string{"l", "-label"}, "Set meta data on a container")
	cmd.Var(&flLabelsFile, []string{"-label-file"}, "Read in a line delimited file of labels")
	cmd.Var(&flEnv, []string{"e", "-env"}, "Set environment variables")
	cmd.Var(&flEnvFile, []string{"-env-file"}, "Read in a file of environment variables")
	cmd.Var(&flSecurityGroups, []string{"-sg"}, "Security group for each container")
	cmd.Var(&flVolumes, []string{"v", "--volume"}, "Volume for each container")

	cmd.Require(flag.Exact, 1)
	err := cmd.ParseFlags(args, true)
	if err != nil {
		return err
	}

	if *flReplicas <= 0 {
		return fmt.Errorf("replicas must be bigger than 0")
	}

	var binds = map[string]struct{}{}
	// add any bind targets to the list of container services
	for bind := range flVolumes.GetMap() {
		binds[bind] = struct{}{}
	}
	var (
		parsedArgs = cmd.Args()
		runCmd     strslice.StrSlice
		entrypoint strslice.StrSlice
		image      = cmd.Arg(0)
	)

	if _, _, err = cli.client.ImageInspectWithRaw(context.Background(), image, false); err != nil && strings.Contains(err.Error(), "No such image") {
		if err := cli.pullImage(context.Background(), image); err != nil {
			return err
		}
	}

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
	var sgs = map[string]struct{}{}
	for sg := range flSecurityGroups.GetMap() {
		sgs[sg] = struct{}{}
	}

	sslData := []byte{}
	if *flSSLCert != "" {
		sslData, err = ioutil.ReadFile(*flSSLCert)
	}

	sv := types.Service{
		Name:                *flName,
		Image:               image,
		WorkingDir:          *flWorkingDir,
		ContainerSize:       *flContainerSize,
		ServicePort:         *flServicePort,
		ContainerPort:       *flContainerPort,
		Replicas:            *flReplicas,
		Entrypoint:          entrypoint,
		Cmd:                 runCmd,
		Env:                 envVariables,
		Volumes:             binds,
		Labels:              opts.ConvertKVStringsToMap(labels),
		SecurityGroups:      sgs,
		Tty:                 *flTty,
		Stdin:               *flStdin,
		NetMode:             *flNetMode,
		StopSignal:          *flStopSignal,
		HealthCheckInterval: *flHealthCheckInterval,
		HealthCheckFall:     *flHealthCheckFall,
		HealthCheckRise:     *flHealthCheckRise,
		Algorithm:           *flAlgorithm,
		Protocol:            *flProtocol,
		SessionAffinity:     *flSessionAffinity,
		SSLCert:             string(sslData),
	}

	service, err := cli.client.ServiceCreate(context.Background(), sv)
	if err != nil {
		return err
	}
	fmt.Fprintf(cli.out, "Service %s is created.\n", service.Name)
	return nil
}

// CmdServiceDelete deletes one or more services
//
// Usage: hyper service rm service [service...]
func (cli *DockerCli) CmdServiceRm(args ...string) error {
	cmd := Cli.Subcmd("service rm", []string{"service [service...]"}, "Remove one or more services", false)
	flKeep := cmd.Bool([]string{"-keep"}, false, "Keep the service container")
	cmd.Require(flag.Min, 1)
	if err := cmd.ParseFlags(args, true); err != nil {
		return err
	}

	status := 0
	for _, sn := range cmd.Args() {
		if err := cli.client.ServiceDelete(context.Background(), sn, *flKeep); err != nil {
			fmt.Fprintf(cli.err, "%s\n", err)
			status = 1
			continue
		}
		fmt.Fprintf(cli.out, "%s\n", sn)
	}
	if status != 0 {
		return Cli.StatusError{StatusCode: status}
	}
	return nil
}

// CmdServiceLs lists all the services
//
// Usage: hyper service ls [OPTIONS]
func (cli *DockerCli) CmdServiceLs(args ...string) error {
	cmd := Cli.Subcmd("service ls", nil, "Lists services", true)

	flFilter := ropts.NewListOpts(nil)
	cmd.Var(&flFilter, []string{"f", "-filter"}, "Filter output based on conditions provided")

	cmd.Require(flag.Exact, 0)
	err := cmd.ParseFlags(args, true)
	if err != nil {
		return err
	}

	// Consolidate all filter flags, and sanity check them early.
	// They'll get process after get response from server.
	serviceFilterArgs := filters.NewArgs()
	for _, f := range flFilter.GetAll() {
		if serviceFilterArgs, err = filters.ParseFlag(f, serviceFilterArgs); err != nil {
			return err
		}
	}

	options := types.ServiceListOptions{
		Filters: serviceFilterArgs,
	}

	services, err := cli.client.ServiceList(context.Background(), options)
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(cli.out, 20, 1, 3, ' ', 0)
	fmt.Fprintf(w, "Name\tFIP\tContainers\tStatus\tMessage\n")
	for _, service := range services {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", service.Name, service.FIP, showContainersInList(service.Containers), service.Status, service.Message)
	}

	w.Flush()
	return nil
}

func showContainersInList(containers []string) string {
	var result = []string{}
	for _, c := range containers {
		result = append(result, c[:12])
	}
	if len(result) > 2 {
		return strings.Join([]string{result[0], result[1], "..."}, ", ")
	}
	return strings.Join(result, ", ")
}

// CmdServiceInspect
//
// Usage: docker service inspect [OPTIONS] service [service...]
func (cli *DockerCli) CmdServiceInspect(args ...string) error {
	cmd := Cli.Subcmd("service inspect", []string{"service [service...]"}, "Display detailed information on the given service", true)
	tmplStr := cmd.String([]string{"f", "-format"}, "", "Format the output using the given go template")

	cmd.Require(flag.Min, 1)
	cmd.ParseFlags(args, true)

	if err := cmd.Parse(args); err != nil {
		return nil
	}

	ctx := context.Background()

	inspectSearcher := func(name string) (interface{}, []byte, error) {
		i, err := cli.client.ServiceInspect(ctx, name)
		return i, nil, err
	}

	return cli.inspectElements(*tmplStr, cmd.Args(), inspectSearcher)
}

// CmdServiceScale
//
// Usage: hyper service scale [OPTIONS] SERVICE=REPLICAS [SERVICE=REPLICAS...]
func (cli *DockerCli) CmdServiceScale(args ...string) error {
	cmd := Cli.Subcmd("service scale", []string{"SERVICE=REPLICAS [SERVICE=REPLICAS...]"}, "", true)

	cmd.Require(flag.Min, 1)
	cmd.ParseFlags(args, true)

	if err := cmd.Parse(args); err != nil {
		return nil
	}

	ctx := context.Background()

	for _, sr := range cmd.Args() {
		fields := strings.SplitN(sr, "=", 2)
		if len(fields) != 2 {
			fmt.Fprintf(cli.err, "invalid argument")
			continue
		}
		replicas, err := strconv.Atoi(fields[1])
		if err != nil {
			fmt.Fprintf(cli.err, "%v\n", err)
			continue
		}
		sv := types.ServiceUpdate{
			Replicas: &replicas,
		}

		service, err := cli.client.ServiceUpdate(ctx, fields[0], sv)
		if err != nil {
			return err
		}
		fmt.Fprintf(cli.out, "%s\n", service.Name)
	}
	return nil
}

// CmdServiceRolling_update
//
// Usage: hyper service rolling-update [OPTIONS] SERVICE [SERVICE...]
func (cli *DockerCli) CmdServiceRolling_update(args ...string) error {
	cmd := Cli.Subcmd("service rolling-update", []string{"SERVICE [SERVICE...]"}, "Perform a rolling update of the given service", true)
	flImage := cmd.String([]string{"-image"}, "", "New container image")

	cmd.Require(flag.Min, 1)
	cmd.ParseFlags(args, true)

	if err := cmd.Parse(args); err != nil {
		return nil
	}

	if len(*flImage) == 0 {
		return fmt.Errorf("image is required for rolling-update")
	}

	ctx := context.Background()
	if _, _, err := cli.client.ImageInspectWithRaw(ctx, *flImage, false); err != nil && strings.Contains(err.Error(), "No such image") {
		if err := cli.pullImage(ctx, *flImage); err != nil {
			return err
		}
	}

	for _, sr := range cmd.Args() {
		sv := types.ServiceUpdate{
			Image: flImage,
		}

		service, err := cli.client.ServiceUpdate(ctx, sr, sv)
		if err != nil {
			return err
		}
		fmt.Fprintf(cli.out, "Rolling-update is requested for service %s.\n", service.Name)
	}
	return nil
}

// CmdServiceAttach_fip
//
// Usage: hyper service attach_fip [OPTIONS] SERVICE [SERVICE...]
func (cli *DockerCli) CmdServiceAttach_fip(args ...string) error {
	cmd := Cli.Subcmd("service attach-fip", []string{"SERVICE"}, "Attach a fip to the service", true)
	flFip := cmd.String([]string{"-fip"}, "", "Attach a fip to the service")

	cmd.Require(flag.Exact, 1)
	cmd.ParseFlags(args, true)

	if err := cmd.Parse(args); err != nil {
		return nil
	}
	if *flFip == "" {
		return fmt.Errorf("Error: please provide the attached FIP via --fip")
	}

	ctx := context.Background()

	sv := types.ServiceUpdate{
		FIP: flFip,
	}

	service, err := cli.client.ServiceUpdate(ctx, cmd.Arg(0), sv)
	if err != nil {
		return err
	}
	fmt.Fprintf(cli.out, "%s\n", service.Name)
	return nil
}

// CmdServiceDetach_fip
//
// Usage: hyper service detach_fip [OPTIONS] SERVICE [SERVICE...]
func (cli *DockerCli) CmdServiceDetach_fip(args ...string) error {
	cmd := Cli.Subcmd("service detach-fip", []string{"SERVICE [SERVICE...]"}, "Detach a fip from the service", true)

	cmd.Require(flag.Min, 1)
	cmd.ParseFlags(args, true)

	if err := cmd.Parse(args); err != nil {
		return nil
	}

	ctx := context.Background()

	fip := ""
	for _, sr := range cmd.Args() {
		sv := types.ServiceUpdate{
			FIP: &fip,
		}

		service, err := cli.client.ServiceUpdate(ctx, sr, sv)
		if err != nil {
			return err
		}
		fmt.Fprintf(cli.out, "%s\n", service.Name)
	}
	return nil
}

func serviceUsage() string {
	serviceCommands := [][]string{
		{"create", "Create a service"},
		{"inspect", "Display detailed information on the given service"},
		{"ls", "List all services"},
		{"scale", "Scale the service"},
		{"rolling-update", "Perform a rolling update of the given service"},
		{"attach-fip", "Attach a fip to the service"},
		{"detach-fip", "Detach the fip from the service"},
		{"rm", "Remove one or more services"},
	}

	help := "Commands:\n"

	for _, cmd := range serviceCommands {
		help += fmt.Sprintf("  %-25.25s%s\n", cmd[0], cmd[1])
	}

	help += fmt.Sprintf("\nRun 'hyper service COMMAND --help' for more information on a command.")
	return help
}
