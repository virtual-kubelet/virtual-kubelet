// Copyright 2016-2017 VMware, Inc. All Rights Reserved.
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

package main

import (
	"context"
	"fmt"
	"io/ioutil"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/libnetwork/iptables"
	"github.com/vishvananda/netlink"

	"github.com/vmware/vic/lib/tether"
)

const (
	publicIfaceName = "public"
)

func (t *operations) SetupFirewall(ctx context.Context, config *tether.ExecutorConfig) error {
	// get the public interface name
	l, err := netlink.LinkByName(publicIfaceName)
	if l == nil {
		l, err = netlink.LinkByAlias(publicIfaceName)
		if l == nil {
			return fmt.Errorf("could not find interface: %s", publicIfaceName)
		}
	}

	if _, err = iptables.Raw(string(iptables.Append), "FORWARD", "-i", "bridge", "-o", l.Attrs().Name, "-j", "ACCEPT"); err != nil {
		return err
	}

	if config.AsymmetricRouting {
		// set rp_filter to "loose" mode; see https://en.wikipedia.org/wiki/Reverse_path_forwarding#Loose_mode
		//
		// this is so the kernel will not drop packets sent to the public interface from an
		// address that is reachable by other interfaces on the VCH. specifically, when
		// packets from the bridge network are directed to another network (say, management),
		// the incoming reply to the VCH can be dropped if rp_filter is set to the default 1
		if err = ioutil.WriteFile(fmt.Sprintf("/proc/sys/net/ipv4/conf/%s/rp_filter", l.Attrs().Name), []byte("2"), 0644); err != nil {
			// not a fatal error, so just log it here
			log.Warnf("error while setting rp_filter for interface %s: %s", l.Attrs().Name, err)
		}
	}

	return nil
}
