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

	"github.com/vmware/vic/pkg/flags"

	"gopkg.in/urfave/cli.v1"
)

type Proxies struct {
	HTTPSProxy *string
	HTTPProxy  *string
	IsSet      bool
}

func (p *Proxies) ProxyFlags() []cli.Flag {
	return []cli.Flag{
		// proxies
		cli.GenericFlag{
			Name:   "https-proxy",
			Value:  flags.NewOptionalString(&p.HTTPSProxy),
			Usage:  "An HTTPS proxy for use when fetching images, in the form https://fqdn_or_ip:port",
			Hidden: true,
		},
		cli.GenericFlag{
			Name:   "http-proxy",
			Value:  flags.NewOptionalString(&p.HTTPProxy),
			Usage:  "An HTTP proxy for use when fetching images, in the form http://fqdn_or_ip:port",
			Hidden: true,
		},
	}
}

func (p *Proxies) ProcessProxies() (hproxy, sproxy *url.URL, err error) {
	if p.HTTPProxy != nil || p.HTTPSProxy != nil {
		p.IsSet = true
	}
	if p.HTTPProxy != nil && *p.HTTPProxy != "" {
		hproxy, err = url.Parse(*p.HTTPProxy)
		if err != nil || hproxy.Host == "" || hproxy.Scheme != "http" {
			err = cli.NewExitError(fmt.Sprintf("Could not parse HTTP proxy - expected format http://fqnd_or_ip:port: %s", *p.HTTPProxy), 1)
			return
		}
	}

	if p.HTTPSProxy != nil && *p.HTTPSProxy != "" {
		sproxy, err = url.Parse(*p.HTTPSProxy)
		if err != nil || sproxy.Host == "" || sproxy.Scheme != "https" {
			err = cli.NewExitError(fmt.Sprintf("Could not parse HTTPS proxy - expected format https://fqnd_or_ip:port: %s", *p.HTTPSProxy), 1)
			return
		}
	}
	return
}
