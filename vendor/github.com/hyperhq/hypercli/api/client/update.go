package client

import (
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/net/context"

	Cli "github.com/hyperhq/hypercli/cli"
	"github.com/hyperhq/hypercli/opts"
	flag "github.com/hyperhq/hypercli/pkg/mflag"
)

// CmdUpdate updates resources of one or more containers.
//
// Usage: hyper update [OPTIONS] CONTAINER [CONTAINER...]
func (cli *DockerCli) CmdUpdate(args ...string) error {
	cmd := Cli.Subcmd("update", []string{"CONTAINER [CONTAINER...]"}, Cli.DockerCommands["update"].Description, true)
	flAddSecurityGroups := opts.NewListOpts(nil)
	flRmSecurityGroups := opts.NewListOpts(nil)
	cmd.Var(&flAddSecurityGroups, []string{"-sg-add"}, "Add a new security group for each container")
	cmd.Var(&flRmSecurityGroups, []string{"-sg-rm"}, "Remove a new security group for each container")
	// make this flag string type to distinguish between 'not set', 'set to false' and 'set to true'
	flContainerProtection := cmd.String([]string{"-protection"}, "", "Termination protection for container (true|false)")

	cmd.Require(flag.Min, 1)
	cmd.ParseFlags(args, true)
	if cmd.NFlag() == 0 {
		return fmt.Errorf("You must provide one or more flags when using this command.")
	}
	if *flContainerProtection != "" {
		value, err := strconv.ParseBool(*flContainerProtection)
		if err != nil {
			return fmt.Errorf("Parse protection false failed: %v", err)
		}
		*flContainerProtection = strconv.FormatBool(value)
	}

	ctx := context.Background()
	names := cmd.Args()
	var errs []string
	for _, name := range names {
		var updateConfig struct {
			AddSecurityGroups      map[string]string
			RemoveSecurityGroups   map[string]string
			SetContainerProtection string
		}
		sgs := map[string]string{}
		for _, label := range flAddSecurityGroups.GetAll() {
			if label == "" {
				continue
			}
			sgs[label] = "yes"
		}
		updateConfig.AddSecurityGroups = sgs
		sgs = map[string]string{}
		for _, label := range flRmSecurityGroups.GetAll() {
			if label == "" {
				continue
			}
			sgs[label] = "yes"
		}
		updateConfig.RemoveSecurityGroups = sgs
		updateConfig.SetContainerProtection = *flContainerProtection
		if err := cli.client.ContainerUpdate(ctx, name, updateConfig); err != nil {
			errs = append(errs, err.Error())
		} else {
			fmt.Fprintf(cli.out, "%s\n", name)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "\n"))
	}

	return nil
}
