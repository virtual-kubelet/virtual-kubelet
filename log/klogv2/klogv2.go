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
// If you want this to be the default logger for virtual-kubelet, set `log.L` to the value returned by `New(fields)`
//
// We recommend reading the klog conventions to build a better understanding of levels and when they should apply
// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-instrumentation/logging.md
package klogv2

import (
	"fmt"
	"sort"
	"strings"

	"github.com/virtual-kubelet/virtual-kubelet/log"
	"k8s.io/klog/v2"
)

// Ensure log.Logger is fully implemented during compile time.
var _ log.Logger = (*adapter)(nil)

// adapter implements the `log.Logger` interface for klogv2
type adapter struct {
	rawFields map[string]interface{}
	fields    string
}

// New creates a new `log.Logger` from the provided entry
func New(fields map[string]interface{}) log.Logger {
	return &adapter{
		rawFields: fields,
		fields:    processFields(fields),
	}
}

func (l *adapter) Debug(args ...interface{}) {
	if klog.V(4).Enabled() {
		l.Info(args...)
	}
}

func (l *adapter) Debugf(format string, args ...interface{}) {
	if klog.V(4).Enabled() {
		l.Infof(format, args...)
	}
}

func (l *adapter) Info(args ...interface{}) {
	args = append(args, l.fields)
	klog.InfoDepth(1, args...)
}

func (l *adapter) Infof(format string, args ...interface{}) {
	formattedArgs := fmt.Sprintf(format, args...)
	klog.InfoDepth(1, formattedArgs, l.fields)
}

func (l *adapter) Warn(args ...interface{}) {
	args = append(args, l.fields)
	klog.WarningDepth(1, args...)
}

func (l *adapter) Warnf(format string, args ...interface{}) {
	formattedArgs := fmt.Sprintf(format, args...)
	klog.WarningDepth(1, formattedArgs, l.fields)
}

func (l *adapter) Error(args ...interface{}) {
	args = append(args, l.fields)
	klog.ErrorDepth(1, args...)
}

func (l *adapter) Errorf(format string, args ...interface{}) {
	formattedArgs := fmt.Sprintf(format, args...)
	klog.ErrorDepth(1, formattedArgs, l.fields)
}

func (l *adapter) Fatal(args ...interface{}) {
	args = append(args, l.fields)
	klog.FatalDepth(1, args...)
}

func (l *adapter) Fatalf(format string, args ...interface{}) {
	formattedArgs := fmt.Sprintf(format, args...)
	klog.FatalDepth(1, formattedArgs, l.fields)
}

// WithField adds a field to the log entry.
func (l *adapter) WithField(key string, val interface{}) log.Logger {
	return l.WithFields(map[string]interface{}{key: val})
}

// WithFields adds multiple fields to a log entry.
func (l *adapter) WithFields(fields log.Fields) log.Logger {
	// Clone existing fields.
	newFields := make(map[string]interface{})
	for k, v := range l.rawFields {
		newFields[k] = v
	}
	// Append new fields.
	// Existing keys will be overwritten.
	for k, v := range fields {
		newFields[k] = v
	}

	return New(fields)
}

// WithError adds an error to the log entry
func (l *adapter) WithError(err error) log.Logger {
	return l.WithFields(map[string]interface{}{"err": err})
}

// processFields prepares a string to be appended to every log call.
// This is called once to avoid costly log operations.
func processFields(fields map[string]interface{}) string {
	processedFields := make([]string, 0, len(fields))
	for k, v := range fields {
		processedFields = append(processedFields, fmt.Sprintf("%s=%+v", k, v))
	}
	// Order fields lexically for easier reading.
	sort.Strings(processedFields)

	// Resulting string forcibly starts with an empty space.
	return fmt.Sprintf(" [%s]", strings.Join(processedFields, " "))
}
