// Copyright 2018 VMware, Inc. All Rights Reserved.
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
	"gopkg.in/urfave/cli.v1"

	"github.com/vmware/vic/pkg/errors"
	"github.com/vmware/vic/pkg/flags"
	"github.com/vmware/vic/pkg/trace"
)

// Kubelet holds credentials for the VCH operations user
type Kubelet struct {
	ServerAddress *string `cmd:"kubelet"`
	ConfigFile    *string
}

func (v *Kubelet) Flags(hidden bool) []cli.Flag {
	return []cli.Flag{
		cli.GenericFlag{
			Name:   "k8s-server-address",
			Value:  flags.NewOptionalString(&v.ServerAddress),
			Usage:  "The Kubernetes Server URL, <hostname/ip>:<port>",
			Hidden: hidden,
		},
		cli.GenericFlag{
			Name:   "k8s-config",
			Value:  flags.NewOptionalString(&v.ConfigFile),
			Usage:  "Kubernetes client config file",
			Hidden: hidden,
		},
	}
}

func (v *Kubelet) ProcessKubelet(op trace.Operation, isCreateOp bool) error {
	if isCreateOp {
		if v.ServerAddress != nil && v.ConfigFile == nil {
			return errors.Errorf("A Kubernetes Config File must be specified when specifying a Kubernetes Server Address")
		}
		if v.ServerAddress == nil && v.ConfigFile != nil {
			return errors.Errorf("A Kubernetes Server Address must be specified when specifying a Kubernetes Config File")
		}
	}

	return nil
}
