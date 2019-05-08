package main

import (
	"context"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/virtual-kubelet/virtual-kubelet/cmd/virtual-kubelet/commands/providers"
	"github.com/virtual-kubelet/virtual-kubelet/cmd/virtual-kubelet/commands/root"
	"github.com/virtual-kubelet/virtual-kubelet/cmd/virtual-kubelet/commands/version"
	"github.com/virtual-kubelet/virtual-kubelet/log"
	logruslogger "github.com/virtual-kubelet/virtual-kubelet/log/logrus"
	"github.com/virtual-kubelet/virtual-kubelet/trace"
	"github.com/virtual-kubelet/virtual-kubelet/trace/opencensus"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		cancel()
	}()

	log.L = logruslogger.FromLogrus(logrus.NewEntry(logrus.StandardLogger()))
	trace.T = opencensus.Adapter{}

	var opts root.Opts
	optsErr := root.SetDefaultOpts(&opts)

	rootCmd := root.NewCommand(ctx, filepath.Base(os.Args[0]), opts)
	rootCmd.AddCommand(version.NewCommand(), providers.NewCommand())
	preRun := rootCmd.PreRunE

	var logLevel string
	rootCmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if optsErr != nil {
			return optsErr
		}
		if preRun != nil {
			return preRun(cmd, args)
		}
		return nil
	}

	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", `set the log level, e.g. "debug", "info", "warn", "error"`)

	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if logLevel != "" {
			lvl, err := logrus.ParseLevel(logLevel)
			if err != nil {
				return errors.Wrap(err, "could not parse log level")
			}
			logrus.SetLevel(lvl)
		}
		return nil
	}

	if err := rootCmd.Execute(); err != nil && errors.Cause(err) != context.Canceled {
		log.G(ctx).Fatal(err)
	}
}
