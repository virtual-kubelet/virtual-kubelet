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

package tether

import (
	"net"
	"time"

	"github.com/vmware/vic/lib/etcconf"
)

type MockSyscall struct{}

func (m MockSyscall) Mount(source string, target string, fstype string, flags uintptr, data string) error {
	return nil
}

func (m MockSyscall) Sethostname(_ []byte) error {
	return nil
}

func (m MockSyscall) Symlink(_, _ string) error {
	return nil
}

func (m MockSyscall) Unmount(_ string, _ int) error {
	return nil
}

type MockHosts struct{}

func (h MockHosts) Load() error {
	return nil
}

func (h MockHosts) Save() error {
	return nil
}

func (h MockHosts) Copy(conf etcconf.Conf) error {
	return nil
}

func (h MockHosts) SetHost(_ string, _ net.IP) {
}

func (h MockHosts) RemoveHost(_ string) {
}

func (h MockHosts) RemoveAll() {
}

func (h MockHosts) HostIP(_ string) []net.IP {
	return nil
}

func (h MockHosts) Path() string {
	return ""
}

type MockResolvConf struct{}

func (h MockResolvConf) Load() error {
	return nil
}

func (h MockResolvConf) Save() error {
	return nil
}

func (h MockResolvConf) Copy(conf etcconf.Conf) error {
	return nil
}

func (h MockResolvConf) AddNameservers(...net.IP) {
}

func (h MockResolvConf) RemoveNameservers(...net.IP) {
}

func (h MockResolvConf) Nameservers() []net.IP {
	return nil
}

func (h MockResolvConf) Attempts() uint {
	return etcconf.DefaultAttempts
}

func (h MockResolvConf) Timeout() time.Duration {
	return etcconf.DefaultTimeout
}

func (h MockResolvConf) SetAttempts(uint) {
}

func (h MockResolvConf) SetTimeout(time.Duration) {
}

func (h MockResolvConf) Path() string {
	return ""
}
