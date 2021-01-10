// Copyright © 2021 The virtual-kubelet authors
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

// Package klogv2 implements a virtual-kubelet/log.Logger using klogv2 as a backend
//
// You can use this by creating a klogv2 logger and calling `FromKlogv2(fields)`.
// If you want this to be the default logger for virtual-kubelet, set `log.L` to the value returned by `FromKlogv2`
//
// We recommend reading the klog conventions to build a better understanding of levels and when they should apply
// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-instrumentation/logging.md
package klogv2

import (
	"github.com/virtual-kubelet/virtual-kubelet/log"
	"k8s.io/klog/v2"
)

// adapter implements the `log.Logger` interface for klogv2
type adapter struct {
	fields map[string]interface{}
}

// FromKlogv2 creates a new `log.Logger` from the provided entry
func FromKlogv2(fields map[string]interface{}) log.Logger {
	return &adapter{fields}
}

func (l *adapter) Debug(args ...interface{}) {
	klog.V(4).Info(args, l.fields)
}

func (l *adapter) Debugf(format string, args ...interface{}) {
	klog.V(4).Infof(format, args, l.fields)
}

func (l *adapter) Info(args ...interface{}) {
	klog.Info(args, l.fields)
}

func (l *adapter) Infof(format string, args ...interface{}) {
	klog.Infof(format, args, l.fields)
}

func (l *adapter) Warn(args ...interface{}) {
	klog.Warning(args, l.fields)
}

func (l *adapter) Warnf(format string, args ...interface{}) {
	klog.Warningf(format, args, l.fields)
}

func (l *adapter) Error(args ...interface{}) {
	klog.Error(args, l.fields)
}

func (l *adapter) Errorf(format string, args ...interface{}) {
	klog.Errorf(format, args, l.fields)
}

func (l *adapter) Fatal(args ...interface{}) {
	klog.Fatal(args, l.fields)
}

func (l *adapter) Fatalf(format string, args ...interface{}) {
	klog.Fatalf(format, args, l.fields)
}

// WithField adds a field to the log entry.
func (l *adapter) WithField(key string, val interface{}) log.Logger {
	fields := map[string]interface{}{
		key: val,
	}
	return FromKlogv2(fields)
}

// WithFields adds multiple fields to a log entry.
func (l *adapter) WithFields(fields log.Fields) log.Logger {
	return FromKlogv2(fields)
}

// WithError adds an error to the log entry
func (l *adapter) WithError(err error) log.Logger {
	return l.WithField("err", err)
}
