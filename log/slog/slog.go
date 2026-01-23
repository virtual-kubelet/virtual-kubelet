// Copyright © 2021 The virtual-kubelet authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// Package slog implements a virtual-kubelet/log.Logger using slog as a backend
// You can use this by creating a slog logger and calling `FromSlog(logger)`
// If you want this to be the default logger for virtual-kubelet, set `log.L` to the value returned by `FromSlog(logger)`
package slog

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/virtual-kubelet/virtual-kubelet/log"
)

// Ensure log.Logger is fully implemented during compile time.
var _ log.Logger = (*adapter)(nil)

// Create a custom logging level for the Fatal level as slog does not
// have this level by default
const LevelFatal = slog.Level(12)

// adapter implements the `log.Logger` interface for slog
type adapter struct {
	inner *slog.Logger
}

// FromSlog creates a new `log.Logger` from a slog logger
func FromSlog(logger *slog.Logger) log.Logger {
	return &adapter{inner: logger}
}

func (l *adapter) Debug(args ...any) {
	msg := args[0].(string)
	l.inner.Debug(msg)
}

func (l *adapter) Debugf(format string, args ...any) {
	formattedArgs := fmt.Sprintf(format, args...)
	l.inner.Debug(formattedArgs)
}

func (l *adapter) Info(args ...any) {
	msg := args[0].(string)
	l.inner.Info(msg)
}

func (l *adapter) Infof(format string, args ...any) {
	formattedArgs := fmt.Sprintf(format, args...)
	l.inner.Info(formattedArgs)
}

func (l *adapter) Warn(args ...any) {
	msg := args[0].(string)
	l.inner.Warn(msg)
}

func (l *adapter) Warnf(format string, args ...any) {
	formattedArgs := fmt.Sprintf(format, args...)
	l.inner.Warn(formattedArgs)
}

func (l *adapter) Error(args ...any) {
	msg := args[0].(string)
	l.inner.Error(msg)
}

func (l *adapter) Errorf(format string, args ...any) {
	formattedArgs := fmt.Sprintf(format, args...)
	l.inner.Error(formattedArgs)
}

func (l *adapter) Fatal(args ...any) {
	msg := args[0].(string)
	l.inner.Log(context.Background(), LevelFatal, msg)
}

func (l *adapter) Fatalf(format string, args ...any) {
	formattedArgs := fmt.Sprintf(format, args...)
	l.inner.Log(context.Background(), LevelFatal, formattedArgs)
}

func (l *adapter) WithField(key string, val any) log.Logger {
	return &adapter{inner: l.inner.With(key, val)}
}

func (l *adapter) WithFields(f log.Fields) log.Logger {
	logger := l.inner
	for k, v := range f {
		logger = logger.With(k, v)
	}
	return &adapter{inner: logger}
}

func (l *adapter) WithError(err error) log.Logger {
	return &adapter{inner: l.inner.With("error", err)}
}
