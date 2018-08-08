package client

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"
	"text/tabwriter"

	"golang.org/x/net/context"

	"github.com/hyperhq/hyper-api/types"
	"github.com/hyperhq/hyper-api/types/filters"
	Cli "github.com/hyperhq/hypercli/cli"
	"github.com/hyperhq/hypercli/opts"
	flag "github.com/hyperhq/hypercli/pkg/mflag"
)

var warnMessage = "Please note that Floating IP (FIP) is billed monthly. The billing begins when a new IP is allocated, ends when it is released. Partial month is treated as a entire month. Do you want to continue?"

// CmdFip is the parent subcommand for all fip commands
//
// Usage: docker fip <COMMAND> [OPTIONS]
func (cli *DockerCli) CmdFip(args ...string) error {
	cmd := Cli.Subcmd("fip", []string{"COMMAND [OPTIONS]"}, fipUsage(), false)
	cmd.Require(flag.Min, 1)
	err := cmd.ParseFlags(args, true)
	cmd.Usage()
	return err
}

// CmdNetworkCreate creates a new fip with a given name
//
// Usage: docker fip create [OPTIONS] COUNT
func (cli *DockerCli) CmdFipAllocate(args ...string) error {
	cmd := Cli.Subcmd("fip allocate", []string{"COUNT"}, "Creates some new floating IPs by the user", false)
	flAvailable := cmd.Bool([]string{"-pick"}, false, "Pick an available floating IP if have")
	flForce := cmd.Bool([]string{"y", "-yes"}, false, "Agree to allocate floating IP, will not show prompt")

	cmd.Require(flag.Exact, 1)
	err := cmd.ParseFlags(args, true)
	if err != nil {
		return err
	}
	if *flAvailable == true {
		fipFilterArgs, _ := filters.FromParam("dangling=true")
		options := types.NetworkListOptions{
			Filters: fipFilterArgs,
		}
		fips, err := cli.client.FipList(context.Background(), options)
		if err == nil {
			for _, fip := range fips {
				if fip["container"] == "" && fip["service"] == "" {
					fmt.Fprintf(cli.out, "%s\n", fip["fip"])
					return nil
				}
			}
		}
	}
	if *flForce == false {
		if askForConfirmation(warnMessage) == false {
			return nil
		}
	}

	fips, err := cli.client.FipAllocate(context.Background(), cmd.Arg(0))
	if err != nil {
		return err
	}
	for _, ip := range fips {
		fmt.Fprintf(cli.out, "%s\n", ip)
	}
	return nil
}

// CmdFipRelease deletes one or more fips
//
// Usage: docker fip release FIP [FIP...]
func (cli *DockerCli) CmdFipRelease(args ...string) error {
	cmd := Cli.Subcmd("fip release", []string{"FIP [FIP...]"}, "Release one or more fips", false)
	cmd.Require(flag.Min, 1)
	if err := cmd.ParseFlags(args, true); err != nil {
		return err
	}

	status := 0
	for _, ip := range cmd.Args() {
		if err := cli.client.FipRelease(context.Background(), ip); err != nil {
			fmt.Fprintf(cli.err, "%s\n", err)
			status = 1
			continue
		}
	}
	if status != 0 {
		return Cli.StatusError{StatusCode: status}
	}
	return nil
}

// CmdFipAttach connects a container to a floating IP
//
// Usage: docker fip attach [OPTIONS] <FIP> <CONTAINER>
func (cli *DockerCli) CmdFipAttach(args ...string) error {
	cmd := Cli.Subcmd("fip attach", []string{"FIP CONTAINER"}, "Connects a container to a floating IP", false)
	flForce := cmd.Bool([]string{"f", "-force"}, false, "Deattach that FIP and attach it to this container")
	cmd.Require(flag.Min, 2)
	if err := cmd.ParseFlags(args, true); err != nil {
		return err
	}
	if *flForce {
		filter, _ := filters.FromParam("dangling=false")
		options := types.NetworkListOptions{
			Filters: filter,
		}

		fips, err := cli.client.FipList(context.Background(), options)
		if err != nil {
			return err
		}
		for _, fip := range fips {
			if ip := fip["fip"]; ip == cmd.Arg(0) {
				if fip["container"] != "" {
					cli.client.FipDetach(context.Background(), fip["container"])
				} else if fip["service"] != "" {
					ip = ""
					sv := types.ServiceUpdate{
						FIP: &ip,
					}
					cli.client.ServiceUpdate(context.Background(), fip["service"], sv)
				}
				break
			}
		}
	}
	return cli.client.FipAttach(context.Background(), cmd.Arg(0), cmd.Arg(1))
}

// CmdFipDetach disconnects a container from a floating IP
//
// Usage: docker fip detach <CONTAINER>
func (cli *DockerCli) CmdFipDetach(args ...string) error {
	cmd := Cli.Subcmd("fip detach", []string{"CONTAINER"}, "Disconnects container from a floating IP", false)
	//force := cmd.Bool([]string{"f", "-force"}, false, "Force the container to disconnect from a floating IP")
	cmd.Require(flag.Exact, 1)
	if err := cmd.ParseFlags(args, true); err != nil {
		return err
	}

	ip, err := cli.client.FipDetach(context.Background(), cmd.Arg(0))
	if err != nil {
		return err
	}
	fmt.Fprintf(cli.out, "%s\n", ip)
	return nil
}

// CmdFipLs lists all the fips
//
// Usage: docker fip ls [OPTIONS]
func (cli *DockerCli) CmdFipLs(args ...string) error {
	cmd := Cli.Subcmd("fip ls", nil, "Lists fips", true)

	flFilter := opts.NewListOpts(nil)
	cmd.Var(&flFilter, []string{"f", "-filter"}, "Filter output based on conditions provided")

	cmd.Require(flag.Exact, 0)
	err := cmd.ParseFlags(args, true)
	if err != nil {
		return err
	}

	// Consolidate all filter flags, and sanity check them early.
	// They'll get process after get response from server.
	fipFilterArgs := filters.NewArgs()
	for _, f := range flFilter.GetAll() {
		if fipFilterArgs, err = filters.ParseFlag(f, fipFilterArgs); err != nil {
			return err
		}
	}

	options := types.NetworkListOptions{
		Filters: fipFilterArgs,
	}

	fips, err := cli.client.FipList(context.Background(), options)
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(cli.out, 20, 1, 3, ' ', 0)
	fmt.Fprintf(w, "Floating IP\tName\tContainer\tService\n")
	for _, fip := range fips {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", fip["fip"], fip["name"], fip["container"], fip["service"])
	}

	w.Flush()
	return nil
}

func (cli *DockerCli) CmdFipName(args ...string) error {
	cmd := Cli.Subcmd("fip name", []string{"FIP [NAME]"}, "Set a name for a floating IP", false)
	//force := cmd.Bool([]string{"f", "-force"}, false, "Force the container to disconnect from a floating IP")
	cmd.Require(flag.Min, 1)
	cmd.Require(flag.Max, 2)
	if err := cmd.ParseFlags(args, true); err != nil {
		return err
	}

	err := cli.client.FipName(context.Background(), cmd.Arg(0), cmd.Arg(1))
	if err != nil {
		return err
	}
	return nil
}

func fipUsage() string {
	fipCommands := [][]string{
		{"allocate", "Allocate a or some IPs"},
		{"attach", "Attach floating IP to container"},
		{"detach", "Detach floating IP from container"},
		{"ls", "List all floating IPs"},
		{"release", "Release a floating IP"},
		{"name", "Name a floating IP"},
	}

	help := "Commands:\n"

	for _, cmd := range fipCommands {
		help += fmt.Sprintf("  %-25.25s%s\n", cmd[0], cmd[1])
	}

	help += fmt.Sprintf("\nRun 'hyper fip COMMAND --help' for more information on a command.")
	return help
}

// Allocate and attach a fip
func (cli *DockerCli) associateNewFip(ctx context.Context, contID string) (string, error) {
	fips, err := cli.client.FipAllocate(ctx, "1")
	if err != nil {
		return "", err
	}

	for _, ip := range fips {
		err = cli.client.FipAttach(ctx, ip, contID)
		if err != nil {
			go func() {
				cli.client.FipRelease(ctx, ip)
			}()
			return "", err
		}
		return ip, nil
	}

	return "", fmt.Errorf("Server failed to create new fip")
}

// Release a fip
func (cli *DockerCli) releaseFip(ctx context.Context, ip string) error {
	return cli.client.FipRelease(ctx, ip)
}

// Detach and release a fip
func (cli *DockerCli) releaseContainerFip(ctx context.Context, contID string) error {
	ip, err := cli.client.FipDetach(ctx, contID)
	if err != nil {
		return err
	}

	return cli.client.FipRelease(ctx, ip)
}

// askForConfirmation asks the user for confirmation. A user must type in "yes" or "no" and
// then press enter. It has fuzzy matching, so "y", "Y", "yes", "YES", and "Yes" all count as
// confirmations. If the input is not recognized, it will ask again. The function does not return
// until it gets a valid response from the user.
func askForConfirmation(s string) bool {
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Printf("%s [y/n]: ", s)

		response, err := reader.ReadString('\n')
		if err != nil {
			log.Fatal(err)
		}

		response = strings.ToLower(strings.TrimSpace(response))

		if response == "y" || response == "yes" {
			return true
		} else if response == "n" || response == "no" {
			return false
		}
	}
}
