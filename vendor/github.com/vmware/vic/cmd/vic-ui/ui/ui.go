// Copyright 2016 VMware, Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ui

import (
	"context"
	"fmt"
	"strings"

	log "github.com/Sirupsen/logrus"
	"gopkg.in/urfave/cli.v1"

	"github.com/vmware/vic/cmd/vic-machine/common"
	"github.com/vmware/vic/lib/install/ova"
	"github.com/vmware/vic/lib/install/plugin"
	"github.com/vmware/vic/pkg/errors"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/version"
	"github.com/vmware/vic/pkg/vsphere/session"
)

// Plugin has all input parameters for vic-ui ui command
type Plugin struct {
	*common.Target
	common.Debug

	Force     bool
	Configure bool
	Insecure  bool

	Company               string
	HideInSolutionManager bool
	Key                   string
	Name                  string
	ServerThumbprint      string
	Summary               string
	Type                  string
	URL                   string
	Version               string
	EntityType            string
}

func NewUI() *Plugin {
	p := &Plugin{Target: common.NewTarget()}
	return p
}

// Flags return all cli flags for ui
func (p *Plugin) Flags() []cli.Flag {
	flags := []cli.Flag{
		cli.BoolFlag{
			Name:        "force, f",
			Usage:       "Force install",
			Destination: &p.Force,
		},
		cli.BoolFlag{
			Name:        "configure-ova",
			Usage:       "Configure OVA ManagedBy information with plugin",
			Destination: &p.Configure,
		},
		cli.StringFlag{
			Name:        "company",
			Value:       "",
			Usage:       "Plugin company name (required)",
			Destination: &p.Company,
		},
		cli.StringFlag{
			Name:        "key",
			Value:       "",
			Usage:       "Plugin key (required)",
			Destination: &p.Key,
		},
		cli.StringFlag{
			Name:        "name",
			Value:       "",
			Usage:       "Plugin name (required)",
			Destination: &p.Name,
		},
		cli.BoolFlag{
			Name:        "no-show",
			Usage:       "Hide plugin in UI",
			Destination: &p.HideInSolutionManager,
		},
		cli.StringFlag{
			Name:        "server-thumbprint",
			Value:       "",
			Usage:       "Plugin server thumbprint (required for HTTPS plugin URL)",
			Destination: &p.ServerThumbprint,
		},
		cli.StringFlag{
			Name:        "summary",
			Value:       "",
			Usage:       "Plugin summary (required)",
			Destination: &p.Summary,
		},
		cli.StringFlag{
			Name:        "url",
			Value:       "",
			Usage:       "Plugin URL (required)",
			Destination: &p.URL,
		},
		cli.StringFlag{
			Name:        "version",
			Value:       "",
			Usage:       "Plugin version (required)",
			Destination: &p.Version,
		},
		cli.StringFlag{
			Name:        "type",
			Value:       "",
			Usage:       "Managed entity type",
			Destination: &p.EntityType,
		},
	}
	flags = append(p.TargetFlags(), flags...)
	flags = append(flags, p.DebugFlags(true)...)

	return flags
}

// InfoFlags return info command
func (p *Plugin) InfoFlags() []cli.Flag {
	flags := []cli.Flag{
		cli.StringFlag{
			Name:        "key",
			Value:       "",
			Usage:       "Plugin key (required)",
			Destination: &p.Key,
		},
	}
	flags = append(p.TargetFlags(), flags...)
	flags = append(flags, p.DebugFlags(true)...)

	return flags
}

func (p *Plugin) processInstallParams(op trace.Operation) error {
	defer trace.End(trace.Begin("", op))

	if err := p.HasCredentials(op); err != nil {
		return err
	}

	if p.Company == "" {
		return cli.NewExitError("--company must be specified", 1)
	}

	if p.Key == "" {
		return cli.NewExitError("--key must be specified", 1)
	}

	if p.Name == "" {
		return cli.NewExitError("--name must be specified", 1)
	}

	if p.Summary == "" {
		return cli.NewExitError("--summary must be specified", 1)
	}

	if p.URL == "" {
		return cli.NewExitError("--url must be specified", 1)
	}

	if p.Version == "" {
		return cli.NewExitError("--version must be specified", 1)
	}

	if strings.HasPrefix(strings.ToLower(p.URL), "https://") && p.ServerThumbprint == "" {
		return cli.NewExitError("--server-thumbprint must be specified when using HTTPS plugin URL", 1)
	}

	p.Insecure = true
	return nil
}

func (p *Plugin) processRemoveParams(op trace.Operation) error {
	defer trace.End(trace.Begin("", op))

	if err := p.HasCredentials(op); err != nil {
		return err
	}

	if p.Key == "" {
		return cli.NewExitError("--key must be specified", 1)
	}

	p.Insecure = true
	return nil
}

func (p *Plugin) processInfoParams(op trace.Operation) error {
	defer trace.End(trace.Begin("", op))

	if err := p.HasCredentials(op); err != nil {
		return err
	}

	if p.Key == "" {
		return cli.NewExitError("--key must be specified", 1)
	}
	return nil
}

func (p *Plugin) Install(cli *cli.Context) error {
	op := trace.NewOperation(context.Background(), "Install")

	var err error
	if err = p.processInstallParams(op); err != nil {
		return err
	}

	if p.Debug.Debug != nil && *p.Debug.Debug > 0 {
		log.SetLevel(log.DebugLevel)
		trace.Logger.Level = log.DebugLevel
	}

	if len(cli.Args()) > 0 {
		log.Error("Install cannot continue: invalid CLI arguments")
		log.Errorf("Unknown argument: %s", cli.Args()[0])
		return errors.New("invalid CLI arguments")
	}

	log.Infof("### Installing UI Plugin ####")

	pInfo := &plugin.Info{
		Company:               p.Company,
		Key:                   p.Key,
		Name:                  p.Name,
		ServerThumbprint:      p.ServerThumbprint,
		ShowInSolutionManager: !p.HideInSolutionManager,
		Summary:               p.Summary,
		Type:                  "vsphere-client-serenity",
		URL:                   p.URL,
		Version:               p.Version,
	}

	if p.EntityType != "" {
		pInfo.ManagedEntityInfo = &plugin.ManagedEntityInfo{
			Description: p.Summary,
			EntityType:  p.EntityType,
		}
	}

	pl, err := plugin.NewPluginator(context.TODO(), p.Target.URL, p.Target.Thumbprint, pInfo)
	if err != nil {
		return err
	}

	reg, err := pl.IsRegistered(pInfo.Key)
	if err != nil {
		return err
	}
	if reg {
		if p.Force {
			log.Info("Removing existing plugin to force install")
			err = pl.Unregister(pInfo.Key)
			if err != nil {
				return err
			}
			log.Info("Removed existing plugin")
		} else {
			msg := fmt.Sprintf("plugin (%s) is already registered", pInfo.Key)
			log.Errorf("Install failed: %s", msg)
			return errors.New(msg)
		}
	}

	log.Info("Installing plugin")
	err = pl.Register()
	if err != nil {
		return err
	}

	reg, err = pl.IsRegistered(pInfo.Key)
	if err != nil {
		return err
	}
	if !reg {
		msg := fmt.Sprintf("post-install check failed to find %s registered", pInfo.Key)
		log.Errorf("Install failed: %s", msg)
		return errors.New(msg)
	}

	log.Info("Installed UI plugin")

	if p.Configure {
		sessionConfig := &session.Config{
			Service:    p.Target.URL.Scheme + "://" + p.Target.URL.Host,
			User:       p.Target.URL.User,
			Thumbprint: p.Thumbprint,
			Insecure:   true,
			UserAgent:  version.UserAgent("vic-ui-installer"),
		}

		// Configure the OVA vm to be managed by this plugin
		if err = ova.ConfigureManagedByInfo(context.TODO(), sessionConfig, pInfo.URL); err != nil {
			return err
		}
	}

	return nil
}

func (p *Plugin) Remove(cli *cli.Context) error {
	op := trace.NewOperation(context.Background(), "Remove")

	var err error
	if err = p.processRemoveParams(op); err != nil {
		return err
	}
	if p.Debug.Debug != nil && *p.Debug.Debug > 0 {
		log.SetLevel(log.DebugLevel)
		trace.Logger.Level = log.DebugLevel
	}

	if len(cli.Args()) > 0 {
		log.Error("Remove cannot continue: invalid CLI arguments")
		log.Errorf("Unknown argument: %s", cli.Args()[0])
		return errors.New("invalid CLI arguments")
	}

	if p.Force {
		log.Info("Ignoring --force")
	}

	log.Infof("### Removing UI Plugin ####")

	pInfo := &plugin.Info{
		Key: p.Key,
	}

	pl, err := plugin.NewPluginator(context.TODO(), p.Target.URL, p.Target.Thumbprint, pInfo)
	if err != nil {
		return err
	}
	reg, err := pl.IsRegistered(pInfo.Key)
	if err != nil {
		return err
	}
	if reg {
		log.Infof("Found target plugin: %s", pInfo.Key)
	} else {
		msg := fmt.Sprintf("failed to find target plugin (%s)", pInfo.Key)
		log.Errorf("Remove failed: %s", msg)
		return errors.New(msg)
	}

	log.Info("Removing plugin")
	err = pl.Unregister(pInfo.Key)
	if err != nil {
		return err
	}

	reg, err = pl.IsRegistered(pInfo.Key)
	if err != nil {
		return err
	}
	if reg {
		msg := fmt.Sprintf("post-remove check found %s still registered", pInfo.Key)
		log.Errorf("Remove failed: %s", msg)
		return errors.New(msg)
	}

	log.Info("Removed UI plugin")
	return nil
}

func (p *Plugin) Info(cli *cli.Context) error {
	op := trace.NewOperation(context.Background(), "Info")

	var err error
	if err = p.processInfoParams(op); err != nil {
		return err
	}

	if len(cli.Args()) > 0 {
		log.Error("Info cannot continue: invalid CLI arguments")
		log.Errorf("Unknown argument: %s", cli.Args()[0])
		return errors.New("invalid CLI arguments")
	}

	pInfo := &plugin.Info{
		Key: p.Key,
	}

	pl, err := plugin.NewPluginator(context.TODO(), p.Target.URL, p.Target.Thumbprint, pInfo)
	if err != nil {
		return err
	}

	reg, err := pl.GetPlugin(p.Key)
	if err != nil {
		return err
	}
	if reg == nil {
		return errors.Errorf("%s is not registered", p.Key)
	}

	log.Infof("%s is registered", p.Key)
	log.Info("")
	log.Infof("Key: %s", reg.Key)
	log.Infof("Name: %s", reg.Description.GetDescription().Label)
	log.Infof("Summary: %s", reg.Description.GetDescription().Summary)
	log.Infof("Company: %s", reg.Company)
	log.Infof("Version: %s", reg.Version)
	return nil
}
