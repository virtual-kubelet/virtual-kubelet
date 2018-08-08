// Copyright 2016-2018 VMware, Inc. All Rights Reserved.
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

import "gopkg.in/urfave/cli.v1"

type Compute struct {
	ComputeResourcePath string `cmd:"compute-resource"`
	DisplayName         string `cmd:"name"`
	UseVMGroup          bool   `cmd:"affinity-vm-group"`
	CreateVMGroup       bool
	DeleteVMGroup       bool
}

func (c *Compute) ComputeFlags() []cli.Flag {
	nameFlag := []cli.Flag{
		cli.StringFlag{
			Name:        "name, n",
			Value:       "virtual-container-host",
			Usage:       "The name of the Virtual Container Host",
			Destination: &c.DisplayName,
		},
	}

	return append(nameFlag, c.ComputeFlagsNoName()...)
}

func (c *Compute) ComputeFlagsNoName() []cli.Flag {
	return []cli.Flag{
		cli.StringFlag{
			Name:        "compute-resource, r",
			Value:       "",
			Usage:       "Compute resource path, e.g. myCluster",
			Destination: &c.ComputeResourcePath,
		},
	}
}

func (c *Compute) AffinityFlags() []cli.Flag {
	return []cli.Flag{
		cli.BoolFlag{
			Name:        "affinity-vm-group",
			Usage:       "Use a DRS VM Group to allow VM-Host affinity rules to be defined for the VCH",
			Destination: &c.UseVMGroup,
			Hidden:      true,
		},
	}
}
