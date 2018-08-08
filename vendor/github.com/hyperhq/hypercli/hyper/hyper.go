package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/hyperhq/hypercli/api/client"
	"github.com/hyperhq/hypercli/cli"
	"github.com/hyperhq/hypercli/dockerversion"
	"github.com/hyperhq/hypercli/pkg/homedir"
	flag "github.com/hyperhq/hypercli/pkg/mflag"
	"github.com/hyperhq/hypercli/pkg/reexec"
	"github.com/hyperhq/hypercli/pkg/selfupdate"
	"github.com/hyperhq/hypercli/pkg/term"
	"github.com/hyperhq/hypercli/utils"
)

func main() {
	if reexec.Init() {
		return
	}

	// Set terminal emulation based on platform as required.
	stdin, stdout, stderr := term.StdStreams()

	logrus.SetOutput(stderr)

	flag.Merge(flag.CommandLine, clientFlags.FlagSet, commonFlags.FlagSet)

	flag.Usage = func() {
		fmt.Fprint(stdout, "Usage: hyper [OPTIONS] COMMAND [arg...]\n"+daemonUsage+"       hyper [ --help | -v | --version ]\n\n")
		fmt.Fprint(stdout, "A self-sufficient runtime for containers.\n\nOptions:\n")

		flag.CommandLine.SetOutput(stdout)
		flag.PrintDefaults()

		help := "\nCommands:\n"

		for _, cmd := range dockerCommands {
			help += fmt.Sprintf("    %-10.10s%s\n", cmd.Name, cmd.Description)
		}

		help += "\nRun 'hyper COMMAND --help' for more information on a command."
		fmt.Fprintf(stdout, "%s\n", help)
	}

	flag.Parse()

	if *flVersion {
		showVersion()
		return
	}

	if *flHelp {
		// if global flag --help is present, regardless of what other options and commands there are,
		// just print the usage.
		flag.Usage()
		return
	}
	var errChan = make(chan error, 1)
	var update bool = false
	var updater = &selfupdate.Updater{
		CurrentVersion: dockerversion.Version,
		ApiURL:         "https://hyper-update.s3.amazonaws.com/",
		BinURL:         "https://hyper-update.s3.amazonaws.com/",
		DiffURL:        "https://hyper-update.s3.amazonaws.com/",
		Dir:            filepath.Join(homedir.Get(), ".hyper"),
		CmdName:        "hyper", // app name
	}

	if updater != nil {
		if update = updater.WantUpdate(); update {
			go func() {
				errChan <- updater.BackgroundRun(update)
			}()
		}
	}

	clientCli := client.NewDockerCli(stdin, stdout, stderr, clientFlags)

	c := cli.New(clientCli, daemonCli)
	if err := c.Run(flag.Args()...); err != nil {
		if sterr, ok := err.(cli.StatusError); ok {
			if sterr.Status != "" {
				fmt.Fprintln(stderr, sterr.Status)
				os.Exit(1)
			}
			os.Exit(sterr.StatusCode)
		}
		fmt.Fprintln(stderr, err)
		os.Exit(1)
	}

	if updater != nil && update {
		fmt.Fprintln(os.Stdout, "Found a newer version, downloading...")
		select {
		case <-time.After(20 * time.Second):
			break
		case <-errChan:
			break
		}
	}
}

func showVersion() {
	if utils.ExperimentalBuild() {
		fmt.Printf("Hyper version %s, build %s, experimental\n", dockerversion.Version, dockerversion.GitCommit)
	} else {
		fmt.Printf("Hyper version %s, build %s\n", dockerversion.Version, dockerversion.GitCommit)
	}
}
