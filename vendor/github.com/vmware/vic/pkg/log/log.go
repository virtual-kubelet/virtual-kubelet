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

package log

import (
	"sync"

	"github.com/Sirupsen/logrus"

	"github.com/vmware/vic/pkg/log/syslog"
)

type LoggingConfig struct {
	Formatter logrus.Formatter
	Level     logrus.Level
	Syslog    *SyslogConfig
}

type SyslogConfig struct {
	Network  string
	RAddr    string
	Tag      string
	Priority syslog.Priority
}

var initializer struct {
	once sync.Once
	err  error
}

func NewLoggingConfig() *LoggingConfig {
	return &LoggingConfig{
		Formatter: NewTextFormatter(),
		Level:     logrus.InfoLevel,
	}
}

func Init(cfg *LoggingConfig) error {
	initializer.once.Do(
		func() {

			var err error

			logger := logrus.StandardLogger()
			f := logger.Formatter
			l := logger.Level
			defer func() {
				initializer.err = err
				if err != nil {
					// revert
					logrus.SetFormatter(f)
					logrus.SetLevel(l)
				}
			}()

			logrus.SetFormatter(cfg.Formatter)
			logrus.SetLevel(cfg.Level)

			logrus.Debugf("log cfg: %+v", *cfg)

			hook, err := CreateSyslogHook(cfg)
			if err == nil && hook != nil {
				logrus.AddHook(hook)
			}
		})

	return initializer.err
}

func CreateSyslogHook(cfg *LoggingConfig) (logrus.Hook, error) {
	if cfg.Syslog == nil {
		return nil, nil
	}
	hook, err := syslog.NewHook(
		cfg.Syslog.Network,
		cfg.Syslog.RAddr,
		cfg.Syslog.Priority,
		cfg.Syslog.Tag,
	)
	if err != nil {
		// not a fatal error, so just log a warning
		logrus.Warnf("error trying to initialize syslog: %s", err)
	}
	return hook, err
}

func (l *LoggingConfig) SetLogLevel(level uint8) {
	l.Level = logrus.Level(level)
}
