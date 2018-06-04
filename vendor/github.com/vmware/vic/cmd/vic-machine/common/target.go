// Copyright 2017 VMware, Inc. All Rights Reserved.
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

package common

import (
	"fmt"
	"net/url"
	"os"

	"golang.org/x/crypto/ssh/terminal"
	"gopkg.in/urfave/cli.v1"

	"github.com/vmware/vic/pkg/flags"
	"github.com/vmware/vic/pkg/trace"
)

type Target struct {
	URL *url.URL `cmd:"target"`

	User        string
	Password    *string
	CloneTicket string
	Thumbprint  string `cmd:"thumbprint"`
}

func NewTarget() *Target {
	return &Target{}
}

func (t *Target) TargetFlags() []cli.Flag {
	return []cli.Flag{
		cli.GenericFlag{
			Name:   "target, t",
			Value:  flags.NewURLFlag(&t.URL),
			Usage:  "REQUIRED. ESXi or vCenter connection URL, specifying a datacenter if multiple exist e.g. root:password@VC-FQDN/datacenter",
			EnvVar: "VIC_MACHINE_TARGET",
		},
		cli.StringFlag{
			Name:        "user, u",
			Value:       "",
			Usage:       "ESX or vCenter user",
			Destination: &t.User,
			EnvVar:      "VIC_MACHINE_USER",
		},
		cli.GenericFlag{
			Name:   "password, p",
			Value:  flags.NewOptionalString(&t.Password),
			Usage:  "ESX or vCenter password",
			EnvVar: "VIC_MACHINE_PASSWORD",
		},
		cli.StringFlag{
			Name:        "thumbprint",
			Value:       "",
			Destination: &t.Thumbprint,
			Usage:       "ESX or vCenter host certificate thumbprint",
			EnvVar:      "VIC_MACHINE_THUMBPRINT",
		},
	}
}

// HasCredentials check that the credentials have been supplied by any of the permitted mechanisms
func (t *Target) HasCredentials(op trace.Operation) error {
	if t.URL == nil {
		return cli.NewExitError("--target argument must be specified", 1)
	}

	// assume if a vsphere session key exists, we want to use that instead of user/pass
	if t.CloneTicket != "" {
		t.URL.User = nil // necessary?
		return nil
	}

	var urlUser string
	var urlPassword *string

	if t.URL.User != nil {
		urlUser = t.URL.User.Username()
		if passwd, set := t.URL.User.Password(); set {
			urlPassword = &passwd
		}
	}
	if t.User == "" && urlUser == "" {
		return cli.NewExitError("vSphere user must be specified, either with --user or as part of --target", 1)
	} else if t.User == "" && urlUser != "" {
		t.User = urlUser
	}

	//prompt for passwd if not specified
	if t.Password == nil && urlPassword == nil {
		op.Infof("vSphere password for %s: ", t.User)
		b, err := terminal.ReadPassword(int(os.Stdin.Fd()))
		if err != nil {
			message := fmt.Sprintf("Failed to read password from stdin: %s", err)
			cli.NewExitError(message, 1)
		}
		sb := string(b)
		t.Password = &sb
	} else if t.Password == nil && urlPassword != nil {
		t.Password = urlPassword
	}

	// Used by vic-machine for Session login
	t.URL.User = url.UserPassword(t.User, *t.Password)

	return nil
}
