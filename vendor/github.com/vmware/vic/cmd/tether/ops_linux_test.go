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

// +build linux

package main

import (
	"errors"
	"fmt"
	"syscall"

	"github.com/vishvananda/netlink"

	"github.com/vmware/vic/pkg/trace"
)

// This has been copied from lib/tether/ but should be split into a common base package that is
// dedicated to mocking these operations. We cannot reference lib/tether/*_test elements from
// outside that package.
type Interface struct {
	netlink.LinkAttrs
	Up    bool
	Addrs []netlink.Addr
}

func (t *Interface) Attrs() *netlink.LinkAttrs {
	return &t.LinkAttrs
}

func (t *Interface) Type() string {
	return "mocked"
}

func (t *Mocker) LinkByName(name string) (netlink.Link, error) {
	defer trace.End(trace.Begin(fmt.Sprintf("Getting link by name %s", name)))

	return t.Interfaces[name], nil
}

func (t *Mocker) LinkSetName(link netlink.Link, name string) error {
	defer trace.End(trace.Begin(fmt.Sprintf("Renaming %s to %s", link.Attrs().Name, name)))

	iface := link.(*Interface)
	_, ok := t.Interfaces[name]
	if ok {
		return fmt.Errorf("Interface with name %s already exists", name)
	}

	oldName := iface.Name
	iface.Name = name
	// make sure there's no period where the interface isn't "present"
	t.Interfaces[name] = iface
	delete(t.Interfaces, oldName)

	return nil
}

func (t *Mocker) LinkSetDown(link netlink.Link) error {
	defer trace.End(trace.Begin(fmt.Sprintf("Bringing %s down", link.Attrs().Name)))

	iface := link.(*Interface)
	iface.Up = false
	// TODO: should this drop addresses?
	return nil
}

func (t *Mocker) LinkSetUp(link netlink.Link) error {
	defer trace.End(trace.Begin(fmt.Sprintf("Bringing %s up", link.Attrs().Name)))

	iface := link.(*Interface)
	iface.Up = true
	return nil
}

func (t *Mocker) LinkSetAlias(link netlink.Link, alias string) error {
	defer trace.End(trace.Begin(fmt.Sprintf("Adding alias %s to %s", alias, link.Attrs().Name)))

	iface := link.(*Interface)
	iface.Alias = alias
	return nil
}

func (t *Mocker) AddrList(link netlink.Link, family int) ([]netlink.Addr, error) {
	defer trace.End(trace.Begin(""))

	iface := link.(*Interface)
	return iface.Addrs, nil
}

func (t *Mocker) AddrAdd(link netlink.Link, addr *netlink.Addr) error {
	defer trace.End(trace.Begin(fmt.Sprintf("Adding %s to %s", addr.String(), link.Attrs().Name)))

	iface := link.(*Interface)

	for _, adr := range iface.Addrs {
		if addr.IP.String() == adr.IP.String() {
			return syscall.EEXIST
		}
	}

	iface.Addrs = append(iface.Addrs, *addr)
	return nil
}

func (t *Mocker) AddrDel(link netlink.Link, addr *netlink.Addr) error {
	iface := link.(*Interface)

	for i, adr := range iface.Addrs {
		if addr.IP.String() == adr.IP.String() {
			iface.Addrs = append(iface.Addrs[:i], iface.Addrs[i+1:]...)
			return nil
		}
	}

	return syscall.EADDRNOTAVAIL
}

func (t *Mocker) RouteAdd(route *netlink.Route) error {
	defer trace.End(trace.Begin("no implemented"))

	// currently ignored
	return nil
}

func (t *Mocker) RouteDel(route *netlink.Route) error {
	defer trace.End(trace.Begin("no implemented"))

	// currently ignored
	return nil
}

func (t *Mocker) RuleList(int) ([]netlink.Rule, error) {
	defer trace.End(trace.Begin("not implemented"))

	return nil, nil
}

func (t *Mocker) LinkBySlot(slot int32) (netlink.Link, error) {
	defer trace.End(trace.Begin(""))

	id := int(slot)
	for _, intf := range t.Interfaces {
		if intf.Attrs().Index == id {
			return intf, nil
		}
	}

	return nil, errors.New("no such interface")
}
