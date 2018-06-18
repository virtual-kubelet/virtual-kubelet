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
	"os"

	"golang.org/x/crypto/ssh/terminal"
	"gopkg.in/urfave/cli.v1"

	"github.com/vmware/vic/pkg/errors"
	"github.com/vmware/vic/pkg/flags"
	"github.com/vmware/vic/pkg/trace"
)

// OpsCredentials holds credentials for the VCH operations user
type OpsCredentials struct {
	OpsUser     *string `cmd:"ops-user"`
	OpsPassword *string
	GrantPerms  *bool
	IsSet       bool
}

func (o *OpsCredentials) Flags(hidden bool) []cli.Flag {
	return []cli.Flag{
		cli.GenericFlag{
			Name:   "ops-user",
			Value:  flags.NewOptionalString(&o.OpsUser),
			Usage:  "The user with which the VCH operates after creation. Defaults to the credential supplied with target",
			Hidden: hidden,
		},
		cli.GenericFlag{
			Name:   "ops-password",
			Value:  flags.NewOptionalString(&o.OpsPassword),
			Usage:  "Password or token for the operations user. Defaults to the credential supplied with target",
			Hidden: hidden,
		},
		cli.GenericFlag{
			Name:   "ops-grant-perms",
			Value:  flags.NewOptionalBool(&o.GrantPerms),
			Usage:  "Create roles and grant required permissions to the specified ops-use",
			Hidden: hidden,
		},
	}
}

// ProcessOpsCredentials processes fields for the VCH operations user. When invoked
// during a VCH create operation, adminUser and adminPassword must be supplied to
// be used as ops credentials if they are not specified by the user. For a configure
// operation, adminUser and adminPassword are not needed.
func (o *OpsCredentials) ProcessOpsCredentials(op trace.Operation, isCreateOp bool, adminUser string, adminPassword *string) error {
	if o.OpsUser == nil && o.OpsPassword != nil {
		return errors.New("Password for operations user specified without user having been specified")
	}

	if isCreateOp {
		if o.OpsUser == nil {
			// Check if there was a request to setup ops-user Roles and Permissions
			if o.GrantPerms != nil {
				// If true return error
				if *o.GrantPerms {
					return errors.Errorf("Invalid use of flag: --ops-grant-perms. Cannot setup Roles and Permissions for administrative user.")
				}
				// If false ignore
				o.GrantPerms = nil
			}
			op.Warn("Using administrative user for VCH operation - use --ops-user to improve security (see -x for advanced help)")
			o.OpsUser = &adminUser
			if adminPassword == nil {
				return errors.New("Unable to use nil password from administrative user for operations user")
			}
			o.OpsPassword = adminPassword
			return nil
		}
	} else {
		if o.OpsUser != nil {
			o.IsSet = true
		}
	}

	if o.OpsPassword != nil {
		return nil
	}

	// Prompt for the ops password only during a create operation or a configure
	// operation where the ops user is specified.
	if isCreateOp || o.IsSet {
		op.Infof("vSphere password for %s: ", *o.OpsUser)
		b, err := terminal.ReadPassword(int(os.Stdin.Fd()))
		if err != nil {
			message := fmt.Sprintf("Failed to read password from stdin: %s", err)
			cli.NewExitError(message, 1)
		}
		sb := string(b)
		o.OpsPassword = &sb
	}

	return nil
}
