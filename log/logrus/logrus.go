// Copyright © 2017 The virtual-kubelet authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package logrus implements a github.com/virtual-kubelet/virtual-kubelet/log.Logger using Logrus as a backend
// You can use this by creating a logrus logger and calling `FromLogrus(entry)`.
// If you want this to be the default logger for virtual-kubelet, set `log.L` to the value returned by `FromLogrus`
package logrus

import (
	"github.com/sirupsen/logrus"
	"github.com/virtual-kubelet/virtual-kubelet/log"
)

// Ensure log.Logger is fully implemented during compile time.
var _ log.Logger = (*adapter)(nil)

// adapter implements the `log.Logger` interface for logrus
type adapter struct {
	*logrus.Entry
}

// FromLogrus creates a new `log.Logger` from the provided entry
func FromLogrus(entry *logrus.Entry) log.Logger {
	return &adapter{entry}
}

// WithField adds a field to the log entry.
func (l *adapter) WithField(key string, val interface{}) log.Logger {
	return FromLogrus(l.Entry.WithField(key, val))
}

// WithFields adds multiple fields to a log entry.
func (l *adapter) WithFields(f log.Fields) log.Logger {
	return FromLogrus(l.Entry.WithFields(logrus.Fields(f)))
}

// WithError adds an error to the log entry
func (l *adapter) WithError(err error) log.Logger {
	return FromLogrus(l.Entry.WithError(err))
}
