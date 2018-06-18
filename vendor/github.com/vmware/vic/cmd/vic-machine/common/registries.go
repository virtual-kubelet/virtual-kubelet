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
	"io/ioutil"

	"gopkg.in/urfave/cli.v1"

	"github.com/vmware/vic/pkg/errors"
	"github.com/vmware/vic/pkg/trace"
)

// Registries contains metadata used to create/configure registry CA data
type Registries struct {
	RegistryCAsArg         cli.StringSlice `arg:"registry-ca"`
	InsecureRegistriesArg  cli.StringSlice `arg:"insecure-registry"`
	WhitelistRegistriesArg cli.StringSlice `arg:"whitelist-registry"`

	RegistryCAs []byte

	InsecureRegistries  []string `cmd:"insecure-registry"`
	WhitelistRegistries []string `cmd:"whitelist-registry"`
}

// Flags generates command line flags
func (r *Registries) Flags() []cli.Flag {
	return []cli.Flag{
		cli.StringSliceFlag{
			Name:  "registry-ca, rc",
			Usage: "Specify a list of additional certificate authority files to use to verify secure registry servers",
			Value: &r.RegistryCAsArg,
		},
	}
}

// LoadRegistryCAs loads additional CA certs for docker registry usage
func (r *Registries) loadRegistryCAs(op trace.Operation) ([]byte, error) {
	defer trace.End(trace.Begin("", op))

	var registryCerts []byte
	for _, f := range r.RegistryCAsArg {
		b, err := ioutil.ReadFile(f)
		if err != nil {
			err = errors.Errorf("Failed to load authority from file %s: %s", f, err)
			return nil, err
		}

		registryCerts = append(registryCerts, b...)
		op.Infof("Loaded registry CA from %s", f)
	}

	return registryCerts, nil
}

func (r *Registries) ProcessRegistries(op trace.Operation) error {
	// load additional certificate authorities for use with registries
	if len(r.RegistryCAsArg) > 0 {
		registryCAs, err := r.loadRegistryCAs(op)
		if err != nil {
			return errors.Errorf("Unable to load CA certificates for registry logins: %s", err)
		}

		r.RegistryCAs = registryCAs
	}

	r.InsecureRegistries = r.InsecureRegistriesArg.Value()
	r.WhitelistRegistries = r.WhitelistRegistriesArg.Value()
	return nil
}
