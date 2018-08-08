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

// The system package provides a collection of interfaces
// that can be used to modify certain properties of the system
// the go program is running on. It can also be used to make
// system calls. The main purpose of this package is to
// abstract away the system so that it can be mocked for
// unit tests.

package system

import (
	"path"

	"github.com/vmware/vic/lib/etcconf"
	"github.com/vmware/vic/pkg/vsphere/sys"
)

type System struct {
	Hosts      etcconf.Hosts      // the hosts file on the system, e.g. /etc/hosts
	ResolvConf etcconf.ResolvConf // the resolv.conf file on the system, e.g. /etc/resolv.conf
	Syscall    Syscall            // syscall interface for making system calls

	// constants
	Root string // system's root path
	UUID string // machine id
}

func New() System {
	// #nosec: Errors unhandled.
	id, _ := sys.UUID()
	return System{
		Hosts:      etcconf.NewHosts(""),      // default hosts files, e.g. /etc/hosts on linux
		ResolvConf: etcconf.NewResolvConf(""), // default resolv.conf file, e.g. /etc/resolv.conf on linux
		Syscall:    &syscallImpl{},            // the syscall interface
		Root:       "/",                       // the system root path
		UUID:       id,
	}
}

// NewWithRoot takes a path at which to set the "root" of the system.
// This will cause the hosts and resolv.conf files to be in their default paths, but
// relative to that root
func NewWithRoot(root string) System {
	id, _ := sys.UUID()
	return System{
		Hosts:      etcconf.NewHosts(path.Join(root, etcconf.HostsPath)),           // default hosts files, e.g. /etc/hosts on linux
		ResolvConf: etcconf.NewResolvConf(path.Join(root, etcconf.ResolvConfPath)), // default resolv.conf file, e.g. /etc/resolv.conf on linux
		Syscall:    &syscallImpl{},                                                 // the syscall interface
		Root:       root,                                                           // the system root path
		UUID:       id,
	}
}
