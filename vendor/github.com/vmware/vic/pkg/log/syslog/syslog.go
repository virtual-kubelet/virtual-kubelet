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
	"errors"
	"os"
	"path/filepath"

	"github.com/Sirupsen/logrus"
)

// Priority is a combination of the syslog facility and
// severity. For example, Alert | Ftp sends an alert severity
// message from the FTP facility. The default severity is Emerg;
// the default facility is Kern.
type Priority int

const severityMask = 0x07
const facilityMask = 0xf8

// maxLogBuffer was set to 100 but debug logging of config overflows that easily so pushing it up
const maxLogBuffer = 500

const (
	// Severity.

	// From /usr/include/sys/syslog.h.
	// These are the same on Linux, BSD, and OS X.
	Emerg   Priority = iota // LOG_EMERG
	Alert                   // LOG_ALERT
	Crit                    // LOG_CRIT
	Err                     // LOG_ERR
	Warning                 // LOG_WARNING
	Notice                  // LOG_NOTICE
	Info                    // LOG_INFO
	Debug                   // LOG_DEBUG
)

const (
	// Facility.

	// From /usr/include/sys/syslog.h.
	// These are the same up to LOG_FTP on Linux, BSD, and OS X.
	Kern     Priority = iota << 3 // LOG_KERN
	User                          // LOG_USER
	Mail                          // LOG_MAIL
	Daemon                        // LOG_DAEMON
	Auth                          // LOG_AUTH
	Syslog                        // LOG_SYSLOG
	Lpr                           // LOG_LPR
	News                          // LOG_NEWS
	Uucp                          // LOG_UUCP
	Cron                          // LOG_CRON
	Authpriv                      // LOG_AUTHPRIV
	Ftp                           // LOG_FTP
	_                             // unused
	_                             // unused
	_                             // unused
	_                             // unused
	Local0                        // LOG_LOCAL0
	Local1                        // LOG_LOCAL1
	Local2                        // LOG_LOCAL2
	Local3                        // LOG_LOCAL3
	Local4                        // LOG_LOCAL4
	Local5                        // LOG_LOCAL5
	Local6                        // LOG_LOCAL6
	Local7                        // LOG_LOCAL7
)

// New establishes a new connection to the system log daemon. Each
// write to the returned writer sends a log message with the given
// priority and prefix.
func New(priority Priority, tag string) (Writer, error) {
	return Dial("", "", priority, tag)
}

// Dial establishes a connection to a log daemon by connecting to
// address raddr on the specified network. Each write to the returned
// writer sends a log message with the given facility, severity and
// tag.
// If network is empty, Dial will connect to the local syslog server.
func Dial(network, raddr string, priority Priority, tag string) (Writer, error) {
	d := &defaultDialer{
		network:  network,
		raddr:    raddr,
		tag:      tag,
		priority: priority,
	}

	return d.dial()
}

type defaultDialer struct {
	network, raddr, tag string
	priority            Priority
}

func validPriority(priority Priority) bool {
	return priority >= 0 && priority <= Local7|Debug
}

func (d *defaultDialer) dial() (Writer, error) {
	if !validPriority(d.priority) {
		return nil, errors.New("log/syslog: invalid priority")
	}

	tag := MakeTag("", d.tag)
	// #nosec: Errors unhandled.
	hostname, _ := os.Hostname()

	w := newWriter(d.priority, tag, hostname, newNetDialer(d.network, d.raddr), newFormatter(d.network, RFC3164))

	go w.run()

	return w, nil
}

const sep = "/"

// MakeTag returns prfeix + sep + proc if prefix is not empty.
// If proc is empty, proc is set to filepath.Base(os.Args[0]).
// If prefix is empty, MakeTag returns proc.
func MakeTag(prefix, proc string) string {
	if len(proc) == 0 {
		proc = filepath.Base(os.Args[0])
	}

	if len(prefix) > 0 {
		return prefix + sep + proc
	}

	return proc
}

// Logger is the logger object used by the package
var Logger = logrus.New()

// Format is the syslog format, e.g. RFC 3164
type Format int

const (
	RFC3164 Format = iota
)
