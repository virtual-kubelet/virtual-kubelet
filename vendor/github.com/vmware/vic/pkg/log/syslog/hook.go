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

import "github.com/Sirupsen/logrus"

type Hook struct {
	writer Writer
}

func NewHook(network, raddr string, priority Priority, tag string) (*Hook, error) {
	return newHook(&defaultDialer{
		network:  network,
		raddr:    raddr,
		priority: priority,
		tag:      tag,
	})
}

func newHook(d dialer) (*Hook, error) {
	hook := &Hook{}

	var err error
	hook.writer, err = d.dial()
	if err != nil {
		return nil, err
	}

	return hook, nil
}

func (hook *Hook) Fire(entry *logrus.Entry) error {
	return hook.writeEntry(entry)
}

func (hook *Hook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (hook *Hook) writeEntry(entry *logrus.Entry) error {
	// just use the message since the timestamp
	// is added by the syslog package
	line := entry.Message

	switch entry.Level {
	case logrus.PanicLevel, logrus.FatalLevel:
		return hook.writer.Crit(line)
	case logrus.ErrorLevel:
		return hook.writer.Err(line)
	case logrus.WarnLevel:
		return hook.writer.Warning(line)
	case logrus.InfoLevel:
		return hook.writer.Info(line)
	case logrus.DebugLevel:
		return hook.writer.Debug(line)
	}

	return nil
}
