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

package syslog

import (
	"fmt"
	"net"
	"os"
	"time"
)

type formatter interface {
	Format(p Priority, ts time.Time, hostname, tag, msg string) string
}

type localFormatter struct{}

func (l *localFormatter) Format(p Priority, ts time.Time, _, tag, msg string) string {
	return fmt.Sprintf("<%d>%s %s[%d]: %s", p, ts.Format(time.Stamp), tag, os.Getpid(), msg)
}

type rfc3164Formatter struct{}

func (c *rfc3164Formatter) Format(p Priority, ts time.Time, hostname, tag, msg string) string {
	return fmt.Sprintf("<%d>%s %s %s[%d]: %s", p, ts.Format(time.RFC3339), hostname, tag, os.Getpid(), msg)
}

type netDialer interface {
	dial() (net.Conn, error)
}

type defaultNetDialer struct {
	network, address string
}

func (d *defaultNetDialer) dial() (net.Conn, error) {
	Logger.Infof("trying to connect to %s://%s", d.network, d.address)
	return net.DialTimeout(d.network, d.address, defaultDialTimeout)
}

func newFormatter(network string, f Format) formatter {
	if network == "" {
		return &localFormatter{}
	}

	switch f {
	case RFC3164:
		return &rfc3164Formatter{}
	}

	return nil
}
