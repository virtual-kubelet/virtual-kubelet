package client

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/Sirupsen/logrus"
	"github.com/docker/engine-api/client"
	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/filters"
	Cli "github.com/hyperhq/hypercli/cli"
	"github.com/hyperhq/hypercli/pkg/jsonmessage"
	flag "github.com/hyperhq/hypercli/pkg/mflag"
	"github.com/hyperhq/libcompose/docker"
	"github.com/hyperhq/libcompose/logger"
	"github.com/hyperhq/libcompose/project"
	"github.com/hyperhq/libcompose/project/options"
	"golang.org/x/net/context"
)

const ComposeFipAuto = "auto"

// CmdCompose is the parent subcommand for all compose commands
//
// Usage: hyper compose <COMMAND> [OPTIONS]
func (cli *DockerCli) CmdCompose(args ...string) error {
	cmd := Cli.Subcmd("compose", []string{"<COMMAND>"}, composeUsage(), false)
	cmd.Require(flag.Min, 1)
	err := cmd.ParseFlags(args, true)
	cmd.Usage()
	return err
}

// CmdComposeRun
//
// Usage: hyper compose run [OPTIONS] SERVICE [COMMAND] [ARGS...]
func (cli *DockerCli) CmdComposeRun(args ...string) error {
	cmd := Cli.Subcmd("compose run", []string{"SERVICE [COMMAND] [ARGS...]"}, "Run a one-off command on a service", false)
	composeFile := cmd.String([]string{"f", "-file"}, "docker-compose.yml", "Specify an alternate compose file")
	projectName := cmd.String([]string{"p", "-project-name"}, "", "Specify an alternate project name")
	rm := cmd.Bool([]string{"-rm"}, false, "Remove container after run, ignored in detached mode")

	cmd.Require(flag.Min, 1)
	err := cmd.ParseFlags(args, true)
	if err != nil {
		return err
	}
	if *projectName == "" {
		*projectName = getBaseDir()
	}
	project, err := docker.NewProject(&docker.Context{
		Context: project.Context{
			ComposeFiles: []string{*composeFile},
			ProjectName:  *projectName,
			Autoremove:   *rm,
		},
		ClientFactory: cli,
	})

	if err != nil {
		return err
	}
	service := cmd.Args()[0]
	status, err := project.Run(context.Background(), service, cmd.Args()[1:])
	if err != nil {
		return err
	}
	if *rm {
		opts := options.Delete{RemoveVolume: true}
		if err = project.Delete(opts, service); err != nil {
			return err
		}
	}
	if status != 0 {
		return Cli.StatusError{StatusCode: status}
	}

	return nil
}

// CmdComposeDown
//
// Usage: hyper compose down [OPTIONS]
func (cli *DockerCli) CmdComposeDown(args ...string) error {
	cmd := Cli.Subcmd("compose down", []string{}, "Stop and remove containers, images, and volumes\ncreated by `up`. Only containers and networks are removed by default.", false)
	projectName := cmd.String([]string{"p", "-project-name"}, "", "Specify an alternate project name")
	rmi := cmd.String([]string{"-rmi"}, "", "Remove images, type may be one of: 'all' to remove\nall images, or 'local' to remove only images that\ndon't have an custom name set by the `image` field")
	vol := cmd.Bool([]string{"v", "-volumes"}, false, "Remove data volumes")
	rmorphans := cmd.Bool([]string{"-remove-orphans"}, false, "Remove containers for services not defined in the Compose file")

	cmd.Require(flag.Exact, 0)
	err := cmd.ParseFlags(args, true)
	if err != nil {
		return err
	}
	imageType := options.ImageType(*rmi)
	if !imageType.Valid() {
		return fmt.Errorf("rmi with %s is not valid", *rmi)
	}
	if *projectName == "" {
		*projectName = getBaseDir()
	}
	body, err := cli.client.ComposeDown(*projectName, cmd.Args(), *rmi, *vol, *rmorphans)
	if err != nil {
		return err
	}
	defer body.Close()
	return jsonmessage.DisplayJSONMessagesStream(body, cli.out, cli.outFd, cli.isTerminalOut, nil)
}

// CmdComposeUp
//
// Usage: hyper compose up [OPTIONS]
func (cli *DockerCli) CmdComposeUp(args ...string) error {
	cmd := Cli.Subcmd("compose up", []string{"[SERVICE...]"}, "Builds, (re)creates, starts, and attaches to containers for a service.\n\nUnless they are already running, this command also starts any linked services.\n\n"+
		"The `hyper compose up` command aggregates the output of each container. When\n"+
		"the command exits, all containers are stopped. Running `hyper compose up -d`\n"+
		"starts the containers in the background and leaves them running.\n\n"+
		"If there are existing containers for a service, and the service's configuration\n"+
		"or image was changed after the container's creation, `hyper compose up` picks\n"+
		"up the changes by stopping and recreating the containers (preserving mounted\n"+
		"volumes). To prevent Compose from picking up changes, use the `--no-recreate`\n"+
		"flag.\n\n"+
		"If you want to force Compose to stop and recreate all containers, use the\n"+
		"`--force-recreate` flag.", false)
	composeFile := cmd.String([]string{"f", "-file"}, "docker-compose.yml", "Specify an alternate compose file")
	projectName := cmd.String([]string{"p", "-project-name"}, "", "Specify an alternate project name")
	detach := cmd.Bool([]string{"d", "-detach"}, false, "Detached mode: Run containers in the background,\nprint new container names.\nIncompatible with --abort-on-container-exit.")
	forcerecreate := cmd.Bool([]string{"-force-recreate"}, false, "Recreate containers even if their configuration\nand image haven't changed.\nIncompatible with --no-recreate.")
	norecreate := cmd.Bool([]string{"-no-recreate"}, false, "If containers already exist, don't recreate them.\nIncompatible with --force-recreate.")

	cmd.Require(flag.Min, 0)
	err := cmd.ParseFlags(args, true)
	if err != nil {
		return err
	}
	project, err := docker.NewProject(&docker.Context{
		Context: project.Context{
			ComposeFiles:  []string{*composeFile},
			ProjectName:   *projectName,
			LoggerFactory: logger.NewColorLoggerFactory(),
		},
		ClientFactory: cli,
	})

	if err != nil {
		return err
	}

	services := cmd.Args()
	c, vc, nc := project.GetConfig()
	if *projectName == "" {
		*projectName = getBaseDir()
	}
	var fips = []string{}
	var newFipNum = 0
	for _, svconfig := range c.M {
		if svconfig.Fip == ComposeFipAuto {
			newFipNum++
		}
	}
	fipFilterArgs, _ := filters.FromParam("dangling=true")
	options := types.NetworkListOptions{
		Filters: fipFilterArgs,
	}
	fipList, err := cli.client.FipList(context.Background(), options)
	if err == nil {
		for _, fip := range fipList {
			if fip["container"] == "" && fip["service"] == "" {
				fips = append(fips, fip["fip"])
			}
		}
	}
	if newFipNum > len(fips) {
		if askForConfirmation(warnMessage) == true {
			newFips, err := cli.client.FipAllocate(context.Background(), fmt.Sprintf("%d", newFipNum-len(fips)))
			if err != nil {
				return err
			}
			fips = append(fips, newFips...)
		}
	}
	i := 0
	for _, svconfig := range c.M {
		if svconfig.Fip == ComposeFipAuto {
			if i >= newFipNum {
				svconfig.Fip = ""
			} else {
				svconfig.Fip = fips[i]
				i++
			}
		}
	}
	body, err := cli.client.ComposeUp(*projectName, services, c, vc, nc, cli.configFile.AuthConfigs, *forcerecreate, *norecreate)
	if err != nil {
		return err
	}
	defer body.Close()
	err = jsonmessage.DisplayJSONMessagesStream(body, cli.out, cli.outFd, cli.isTerminalOut, nil)
	if err != nil {
		return err
	}
	if !*detach {
		signalChan := make(chan os.Signal, 1)
		cleanupDone := make(chan bool)
		signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
		errChan := make(chan error)
		go func() {
			errChan <- project.Log(true, services...)
		}()
		go func() {
			select {
			case <-signalChan:
				fmt.Printf("\nGracefully stopping...\n")
				project.Stop(0, services...)
				cleanupDone <- true
			case err := <-errChan:
				if err != nil {
					logrus.Fatal(err)
				}
				cleanupDone <- true
			}
		}()
		<-cleanupDone
		return nil
	}

	return nil
}

// CmdComposeStart
//
// Usage: hyper compose start [OPTIONS] [SERVICE]
func (cli *DockerCli) CmdComposeStart(args ...string) error {
	cmd := Cli.Subcmd("compose start", []string{"[SERVICE...]"}, "Start existing containers.", false)
	projectName := cmd.String([]string{"p", "-project-name"}, "", "Specify an alternate project name")
	cmd.Require(flag.Min, 0)
	err := cmd.ParseFlags(args, true)
	if err != nil {
		return err
	}
	services := cmd.Args()
	if *projectName == "" {
		*projectName = getBaseDir()
	}
	body, err := cli.client.ComposeStart(*projectName, services)
	if err != nil {
		return err
	}
	defer body.Close()
	return jsonmessage.DisplayJSONMessagesStream(body, cli.out, cli.outFd, cli.isTerminalOut, nil)
}

// CmdComposeStop
//
// Usage: hyper compose stop [OPTIONS]
func (cli *DockerCli) CmdComposeStop(args ...string) error {
	cmd := Cli.Subcmd("compose stop", []string{"[SERVICE...]"}, "Stop running containers without removing them.\n\nThey can be started again with `hyper compose start`.", false)
	projectName := cmd.String([]string{"p", "-project-name"}, "", "Specify an alternate project name")
	nSeconds := cmd.Int([]string{"t", "-timeout"}, 10, "Specify a shutdown timeout in seconds.")
	cmd.Require(flag.Min, 0)
	err := cmd.ParseFlags(args, true)
	if err != nil {
		return err
	}

	services := cmd.Args()
	if *projectName == "" {
		*projectName = getBaseDir()
	}
	body, err := cli.client.ComposeStop(*projectName, services, *nSeconds)
	if err != nil {
		return err
	}
	defer body.Close()
	return jsonmessage.DisplayJSONMessagesStream(body, cli.out, cli.outFd, cli.isTerminalOut, nil)
}

// CmdComposeCreate
//
// Usage: hyper compose create [OPTIONS]
func (cli *DockerCli) CmdComposeCreate(args ...string) error {
	cmd := Cli.Subcmd("compose create", []string{"[SERVICE...]"}, "Creates containers for a service.", false)
	composeFile := cmd.String([]string{"f", "-file"}, "docker-compose.yml", "Specify an alternate compose file")
	projectName := cmd.String([]string{"p", "-project-name"}, "", "Specify an alternate project name")
	forcerecreate := cmd.Bool([]string{"-force-recreate"}, false, "Recreate containers even if their configuration\nand image haven't changed.\nIncompatible with --no-recreate.")
	norecreate := cmd.Bool([]string{"-no-recreate"}, false, "If containers already exist, don't recreate them.\nIncompatible with --force-recreate.")
	cmd.Require(flag.Min, 0)
	err := cmd.ParseFlags(args, true)
	if err != nil {
		return err
	}
	project, err := docker.NewProject(&docker.Context{
		Context: project.Context{
			ComposeFiles:  []string{*composeFile},
			ProjectName:   *projectName,
			LoggerFactory: logger.NewColorLoggerFactory(),
		},
		ClientFactory: cli,
	})

	if err != nil {
		return err
	}

	services := cmd.Args()
	c, vc, nc := project.GetConfig()
	if *projectName == "" {
		*projectName = getBaseDir()
	}
	body, err := cli.client.ComposeCreate(*projectName, services, c, vc, nc, cli.configFile.AuthConfigs, *forcerecreate, *norecreate)
	if err != nil {
		return err
	}
	defer body.Close()

	return jsonmessage.DisplayJSONMessagesStream(body, cli.out, cli.outFd, cli.isTerminalOut, nil)
}

// CmdComposePs
//
// Usage: hyper compose ps [OPTIONS]
func (cli *DockerCli) CmdComposePs(args ...string) error {
	cmd := Cli.Subcmd("compose ps", []string{"[SERVICE...]"}, "List containers.", false)
	composeFile := cmd.String([]string{"f", "-file"}, "docker-compose.yml", "Specify an alternate compose file")
	projectName := cmd.String([]string{"p", "-project-name"}, "", "Specify an alternate project name")
	quiet := cmd.Bool([]string{"q", "-quiet"}, false, "Only display IDs")
	cmd.Require(flag.Min, 0)
	err := cmd.ParseFlags(args, true)
	if err != nil {
		return err
	}
	if *projectName == "" {
		*projectName = getBaseDir()
	}
	project, err := docker.NewProject(&docker.Context{
		Context: project.Context{
			ComposeFiles: []string{*composeFile},
			ProjectName:  *projectName,
		},
		ClientFactory: cli,
	})

	if err != nil {
		return err
	}
	ps, err := project.Ps(*quiet, cmd.Args()...)
	if err != nil {
		return err
	}
	fmt.Printf(ps.String(true))

	return nil
}

// CmdComposeKill
//
// Usage: hyper compose kill [OPTIONS]
func (cli *DockerCli) CmdComposeKill(args ...string) error {
	cmd := Cli.Subcmd("compose kill", []string{"[SERVICE...]"}, "Force stop service containers.", false)
	projectName := cmd.String([]string{"p", "-project-name"}, "", "Specify an alternate project name")
	signal := cmd.String([]string{}, "KILL", "Signal to send to the container")
	cmd.Require(flag.Min, 0)
	err := cmd.ParseFlags(args, true)
	if err != nil {
		return err
	}
	services := cmd.Args()
	if *projectName == "" {
		*projectName = getBaseDir()
	}
	body, err := cli.client.ComposeKill(*projectName, services, *signal)
	if err != nil {
		return err
	}
	defer body.Close()
	return jsonmessage.DisplayJSONMessagesStream(body, cli.out, cli.outFd, cli.isTerminalOut, nil)
}

// CmdComposeRm
//
// Usage: hyper compose rm [OPTIONS] [SERVICE]
func (cli *DockerCli) CmdComposeRm(args ...string) error {
	cmd := Cli.Subcmd("compose rm", []string{"[SERVICE...]"}, "Remove stopped service containers.", false)
	projectName := cmd.String([]string{"p", "-project-name"}, "", "Specify an alternate project name")
	removeVol := cmd.Bool([]string{"v"}, false, "Remove volumes associated with containers")
	cmd.Require(flag.Min, 0)
	err := cmd.ParseFlags(args, true)
	if err != nil {
		return err
	}
	services := cmd.Args()
	if *projectName == "" {
		*projectName = getBaseDir()
	}
	body, err := cli.client.ComposeRm(*projectName, services, *removeVol)
	if err != nil {
		return err
	}
	defer body.Close()
	return jsonmessage.DisplayJSONMessagesStream(body, cli.out, cli.outFd, cli.isTerminalOut, nil)
}

// CmdComposeScale
//
// Usage: hyper compose scale [OPTIONS] [SERVICE=NUM...]
func (cli *DockerCli) CmdComposeScale(args ...string) error {
	cmd := Cli.Subcmd("compose scale", []string{"[SERVICE=NUM...]"}, "Set number of containers to run for a service.", false)
	composeFile := cmd.String([]string{"f", "-file"}, "docker-compose.yml", "Specify an alternate compose file")
	projectName := cmd.String([]string{"p", "-project-name"}, "", "Specify an alternate project name")
	timeout := cmd.Int([]string{"t", "-timeout"}, 10, "Specify a shutdown timeout in seconds")
	cmd.Require(flag.Min, 0)
	err := cmd.ParseFlags(args, true)
	if err != nil {
		return err
	}
	if *projectName == "" {
		*projectName = getBaseDir()
	}
	project, err := docker.NewProject(&docker.Context{
		Context: project.Context{
			ComposeFiles: []string{*composeFile},
			ProjectName:  *projectName,
		},
		ClientFactory: cli,
	})

	if err != nil {
		return err
	}
	servicesScale := map[string]int{}
	for _, ss := range cmd.Args() {
		fields := strings.SplitN(ss, "=", 2)
		if len(fields) != 2 {
			continue
		}
		num, err := strconv.Atoi(fields[1])
		if err != nil {
			return err
		}
		servicesScale[fields[0]] = num
	}
	err = project.Scale(*timeout, servicesScale)
	if err != nil {
		return err
	}

	return nil
}

// CmdComposePull
//
// Usage: hyper compose pull [OPTIONS]
func (cli *DockerCli) CmdComposePull(args ...string) error {
	cmd := Cli.Subcmd("compose pull", []string{"[SERVICE...]"}, "Pull images of services.", false)
	composeFile := cmd.String([]string{"f", "-file"}, "docker-compose.yml", "Specify an alternate compose file")
	cmd.Require(flag.Min, 0)
	err := cmd.ParseFlags(args, true)
	if err != nil {
		return err
	}
	project, err := docker.NewProject(&docker.Context{
		Context: project.Context{
			ComposeFiles: []string{*composeFile},
		},
		ClientFactory: cli,
	})

	if err != nil {
		return err
	}
	err = project.Pull(cmd.Args()...)
	if err != nil {
		return err
	}
	return nil
}

func composeUsage() string {
	composeCommands := [][]string{
		{"create", "Creates containers for a service"},
		{"down", "Stop and remove containers, images, and volumes"},
		{"kill", "Force stop service containers"},
		{"ps", "List containers"},
		{"pull", "Pull images of services"},
		{"rm", "Remove stopped service containers"},
		{"run", "Run a one-off command"},
		{"scale", "Set number of containers for a service"},
		{"start", "Start services"},
		{"stop", "Stop services"},
		{"up", "Create and start containers"},
	}

	help := "Commands:\n"

	for _, cmd := range composeCommands {
		help += fmt.Sprintf("  %-25.25s%s\n", cmd[0], cmd[1])
	}

	help += fmt.Sprintf("\nRun 'hyper compose COMMAND --help' for more information on a command.")
	return help
}

func (cli *DockerCli) Create(s project.Service) client.APIClient {
	return cli.client
}

func getBaseDir() string {
	file, err := os.Getwd()
	if err != nil {
		return ""
	}
	return filepath.Base(file)
}
