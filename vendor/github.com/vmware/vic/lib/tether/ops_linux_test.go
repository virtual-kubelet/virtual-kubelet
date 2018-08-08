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

// +build linux

package tether

import (
	"errors"
	"fmt"
	"os/user"
	"path"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vishvananda/netlink"

	"os"

	"io/ioutil"

	"github.com/vmware/vic/pkg/trace"
)

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

func TestSlotToPciPath(t *testing.T) {
	var tests = []struct {
		slot int32
		p    string
		err  error
	}{
		{0, path.Join(pciDevPath, "0000:00:00.0"), nil},
		{32, path.Join(pciDevPath, "0000:00:11.0", "0000:*:00.0"), nil},
		{33, path.Join(pciDevPath, "0000:00:11.0", "0000:*:01.0"), nil},
		{192, path.Join(pciDevPath, "0000:00:16.0", "0000:*:00.0"), nil},
		{1184, path.Join(pciDevPath, "0000:00:15.1", "0000:*:00.0"), nil},
		{1216, path.Join(pciDevPath, "0000:00:16.1", "0000:*:00.0"), nil},
		{1248, path.Join(pciDevPath, "0000:00:17.1", "0000:*:00.0"), nil},
		{1280, path.Join(pciDevPath, "0000:00:18.1", "0000:*:00.0"), nil},
	}

	for _, te := range tests {
		p, err := slotToPCIPath(te.slot, 0)
		if te.err != nil {
			if err == nil {
				t.Fatalf("slotToPCIPath(%d) => (%#v, %#v), want (%s, nil)", te.slot, p, err, te.p)
			}

			continue
		}

		if p != te.p {
			t.Fatalf("slotToPCIPath(%d) => (%#v, %#v), want (%s, nil)", te.slot, p, err, te.p)
		}
	}
}

func TestGetUserSysProcAttr(t *testing.T) {
	curr, err := user.Current()
	if err != nil {
		t.Logf("Failed to get current user, skip test")
		return
	}
	currUID, _ := strconv.Atoi(curr.Uid)
	currGID, _ := strconv.Atoi(curr.Gid)

	moreThanMax := strconv.Itoa(1 << 33)
	lessThanMin := "-100"

	var tests = []struct {
		uid  string
		gid  string
		ruid int
		rgid int
		err  error
	}{
		{"", "", 0, 0, nil},
		{"notexist", "notexist", 0, 0, errors.New("Unable to find user notexist")},
		{"notexist", curr.Gid, 0, 0, errors.New("Unable to find user notexist")},
		{"", "notexist", 0, 0, errors.New("Unable to find group notexist")},
		{"0", "notexist", 0, 0, errors.New("Unable to find group notexist")},
		{curr.Uid, "notexist", 0, 0, errors.New("Unable to find group notexist")},
		{"2000000", "notexist", 0, 0, errors.New("Unable to find group notexist")},
		{curr.Username, "notexist", 0, 0, errors.New("Unable to find group notexist")},
		{curr.Username, "0", currUID, 0, nil},
		{curr.Uid, "1000", currUID, 1000, nil},
		{curr.Uid, curr.Gid, currUID, currGID, nil},
		{"root", curr.Gid, 0, currGID, nil},
		{"root", "root", 0, 0, nil},
		{moreThanMax, "root", 0, 0, errors.New("Uids and gids must be in range 0-2147483647")},
		{curr.Username, moreThanMax, 0, 0, errors.New("Uids and gids must be in range 0-2147483647")},
		{lessThanMin, "root", 0, 0, errors.New("Uids and gids must be in range 0-2147483647")},
		{curr.Username, lessThanMin, 0, 0, errors.New("Uids and gids must be in range 0-2147483647")},
	}
	for _, test := range tests {
		t.Logf("uid: %s, gid: %s", test.uid, test.gid)
		user, err := getUserSysProcAttr(test.uid, test.gid)
		if err != nil {
			assert.True(t, test.err != nil, fmt.Sprintf("Should not have error %s", err))
			if !strings.Contains(err.Error(), test.err.Error()) {
				assert.True(t, false, fmt.Sprintf("error message mismatch, expected %s, actual %s", test.err, err.Error()))
			}
			continue
		}
		assert.True(t, test.err == nil, fmt.Sprintf("didn't get expected error: %s", test.err))
		if user == nil {
			assert.True(t, (test.ruid == 0 && test.rgid == 0), "returned user is nil, but expect not nil result: %d:%d", test.ruid, test.rgid)
			continue
		}
		assert.Equal(t, test.ruid, int(user.Credential.Uid), "returned user id mismatch")
		assert.Equal(t, test.rgid, int(user.Credential.Gid), "returned group id mismatch")
	}
}

func TestIsEmpty(t *testing.T) {
	dir, err := ioutil.TempDir("", "testisEmpty")
	assert.NoError(t, err)

	defer os.RemoveAll(dir)

	err = os.MkdirAll(dir+"/lost+found", 0644)
	assert.NoError(t, err)

	ok, err := isEmpty(dir)
	assert.True(t, ok)
	assert.NoError(t, err)

	err = os.MkdirAll(dir+"/test1", 0644)
	assert.NoError(t, err)
	err = os.MkdirAll(dir+"/test2", 0644)
	assert.NoError(t, err)
	err = ioutil.WriteFile(dir+"/test3.txt", []byte{}, 0644)
	assert.NoError(t, err)

	ok, err = isEmpty(dir)
	assert.False(t, ok)
	assert.NoError(t, err)

}

func TestCreateBindSrcTarget(t *testing.T) {
	dir, err := ioutil.TempDir("", "testCreateBindSrcTarget")
	assert.NoError(t, err)

	defer os.RemoveAll(dir)

	// Create a file under an existing directory
	file := dir + "/test1"
	err = createBindSrcTarget(map[string]os.FileMode{file: 0644})
	assert.NoError(t, err, "createBindSrcTarget failed: %s", err)
	_, err = os.Stat(dir)
	assert.NoError(t, err)

	// Create a file under non-existent directory
	file = dir + "/testDir/test1"
	err = createBindSrcTarget(map[string]os.FileMode{file: 0644})
	assert.NoError(t, err, "createBindSrcTarget failed: %s", err)
	_, err = os.Stat(dir)
	assert.NoError(t, err)

	// Create an existing file
	err = createBindSrcTarget(map[string]os.FileMode{file: 0644})
	assert.NoError(t, err, "createBindSrcTarget failed: %s", err)
}
