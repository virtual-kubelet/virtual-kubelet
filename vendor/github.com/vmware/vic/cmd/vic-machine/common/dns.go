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
	"net"

	"gopkg.in/urfave/cli.v1"

	"github.com/vmware/vic/pkg/errors"
	"github.com/vmware/vic/pkg/trace"
)

// general dns
type DNS struct {
	DNS   cli.StringSlice `arg:"dns-server"`
	IsSet bool
}

func (d *DNS) DNSFlags(hidden bool) []cli.Flag {
	return []cli.Flag{
		cli.StringSliceFlag{
			Name:   "dns-server",
			Value:  &d.DNS,
			Usage:  "DNS server for the client, public, and management networks. Defaults to 8.8.8.8 and 8.8.4.4 when VCH uses static IP",
			Hidden: hidden,
		},
	}
}

// processDNSServers parses DNS servers used for client, public, mgmt networks
func (d *DNS) ProcessDNSServers(op trace.Operation) ([]net.IP, error) {
	var parsedDNS []net.IP
	if len(d.DNS) > 0 {
		d.IsSet = true
	}

	for _, n := range d.DNS {
		if n != "" {
			s := net.ParseIP(n)
			if s == nil {
				return nil, errors.Errorf("Invalid DNS server specified: %s", n)
			}
			parsedDNS = append(parsedDNS, s)
		}
	}

	if len(parsedDNS) > 3 {
		op.Warn("Maximum of 3 DNS servers allowed. Additional servers specified will be ignored.")
	}

	op.Debugf("VCH DNS servers: %s", parsedDNS)
	return parsedDNS, nil
}
