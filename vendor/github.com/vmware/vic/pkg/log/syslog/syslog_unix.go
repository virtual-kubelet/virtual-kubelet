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

// +build !windows,!nacl,!plan9

package syslog

import (
	"errors"
	"net"
)

type unixSyslogDialer struct{}

// unixSyslog opens a connection to the syslog daemon running on the
// local machine using a Unix domain socket.
func (u *unixSyslogDialer) dial() (net.Conn, error) {
	logTypes := []string{"unixgram", "unix"}
	logPaths := []string{"/dev/log", "/var/run/syslog", "/var/run/log"}
	for _, network := range logTypes {
		for _, path := range logPaths {
			conn, err := net.Dial(network, path)
			if err != nil {
				continue
			} else {
				return conn, nil
			}
		}
	}
	return nil, errors.New("Unix syslog delivery error")
}

func newNetDialer(network, address string) netDialer {
	if network == "" {
		return &unixSyslogDialer{}
	}

	return &defaultNetDialer{
		network: network,
		address: address,
	}
}
