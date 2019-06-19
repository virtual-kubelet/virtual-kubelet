// Copyright © 2017 The virtual-kubelet authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cli

import (
	"context"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/virtual-kubelet/virtual-kubelet/node/cli/internal/commands/providers"
	"github.com/virtual-kubelet/virtual-kubelet/node/cli/internal/commands/root"
	"github.com/virtual-kubelet/virtual-kubelet/node/cli/internal/commands/version"
	"github.com/virtual-kubelet/virtual-kubelet/node/cli/opts"
	"github.com/virtual-kubelet/virtual-kubelet/node/cli/provider"
)

// Option sets an option on the command.
type Option func(*Command)

// Command builds the CLI command
type Command struct {
	cmd                *cobra.Command
	s                  *provider.Store
	name               string
	version            string
	buildTime          string
	k8sVersion         string
	persistentFlags    []*pflag.FlagSet
	persistentPreRunCb []func() error
	opts               *opts.Opts
}

// ContextWithCancelOnSignal returns a context which will be cancelled when
// receiving one of the passed in signals.
// If no signals are passed in, the default signals SIGTERM and SIGINT are used.
func ContextWithCancelOnSignal(ctx context.Context, signals ...os.Signal) context.Context {
	ctx, cancel := context.WithCancel(ctx)
	sig := make(chan os.Signal, 1)
	if signals == nil {
		signals = []os.Signal{syscall.SIGINT, syscall.SIGTERM}
	}
	signal.Notify(sig, signals...)
	go func() {
		<-sig
		cancel()
	}()

	return ctx
}

// WithBaseOpts sets the base options used
func WithBaseOpts(o *opts.Opts) Option {
	return func(c *Command) {
		c.opts = o
	}
}

// WithPersistentFlagsAllows you to attach custom, persitent flags to the command.
// The flags are added to the main command and all sub commands.
func WithPersistentFlags(flags *pflag.FlagSet) Option {
	return func(c *Command) {
		c.persistentFlags = append(c.persistentFlags, flags)
	}
}

// WithProvider registers a provider which the cli can be initialized with.
func WithProvider(name string, f provider.InitFunc) Option {
	return func(c *Command) {
		if c.s == nil {
			c.s = provider.NewStore()
		}
		c.s.Register(name, f)
	}
}

// WithCLIVersion sets the version details for the `version` subcommand.
func WithCLIVersion(version, buildTime string) Option {
	return func(c *Command) {
		c.version = version
		c.buildTime = buildTime
	}
}

// WithCLIBaseName sets the name of the command.
// This is used for things like help output.
//
// If not set, the name is taken from `filepath.Base(os.Args[0])`
func WithCLIBaseName(n string) Option {
	return func(c *Command) {
		c.name = n
	}
}

// WithKubernetesNodeVersion sets the version of kubernetes this should report
// as to the Kubernetes API server.
func WithKubernetesNodeVersion(v string) Option {
	return func(c *Command) {
		c.k8sVersion = v
	}
}

// WithPersistentPreRunCallback adds a callback which is called after flags are processed
// but before running the command or any sub-command
func WithPersistentPreRunCallback(f func() error) Option {
	return func(c *Command) {
		c.persistentPreRunCb = append(c.persistentPreRunCb, f)
	}
}

// New creates a new command.
// Call `Run()` on the returned object to run the command.
func New(ctx context.Context, options ...Option) (*Command, error) {
	var c Command
	for _, o := range options {
		o(&c)
	}

	name := c.name
	if name == "" {
		name = filepath.Base(os.Args[0])
	}

	flagOpts := c.opts
	if flagOpts == nil {
		flagOpts = opts.New()
	}

	if c.k8sVersion != "" {
		flagOpts.Version = c.k8sVersion
	}

	c.cmd = root.NewCommand(ctx, name, c.s, flagOpts)
	for _, f := range c.persistentFlags {
		c.cmd.PersistentFlags().AddFlagSet(f)
	}

	c.cmd.PersistentPreRunE = func(_ *cobra.Command, _ []string) error {
		for _, f := range c.persistentPreRunCb {
			if err := f(); err != nil {
				return err
			}
		}
		return nil
	}

	c.cmd.AddCommand(version.NewCommand(c.version, c.buildTime), providers.NewCommand(c.s))
	return &c, nil
}

// Run executes the command with the provided args.
// If args is nil then os.Args[1:] is used
func (c *Command) Run(args ...string) error {
	c.cmd.SetArgs(args)
	return c.cmd.Execute()
}
