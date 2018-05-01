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

package network

import "net"
import "github.com/vishvananda/netlink"

type link struct {
	netlink.Link

	attrs *LinkAttrs
}

func newLink(l netlink.Link) Link {
	attrs := &LinkAttrs{Name: l.Attrs().Name}
	return &link{Link: l, attrs: attrs}
}

func LinkByName(name string) (Link, error) {
	l, err := netlink.LinkByName(name)
	if err != nil {
		return nil, err
	}

	return newLink(l), nil
}

func LinkByAlias(alias string) (Link, error) {
	l, err := netlink.LinkByAlias(alias)
	if err != nil {
		return nil, err
	}

	return newLink(l), nil
}

func (l *link) AddrAdd(addr net.IPNet) error {
	return netlink.AddrAdd(l.Link, &netlink.Addr{IPNet: &addr})
}

func (l *link) AddrDel(addr net.IPNet) error {
	return netlink.AddrDel(l.Link, &netlink.Addr{IPNet: &addr})
}

func (l *link) Attrs() *LinkAttrs {
	return l.attrs
}
