// Copyright 2017 VMware, Inc. All Rights Reserved.
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

package main

import (
	"fmt"
	"io"

	"github.com/Sirupsen/logrus"
)

// ErrorHook is a simple logrus hook to duplicate Fatalf leveled error messages to stderr
type ErrorHook struct {
	w io.Writer
}

// NewErrorHook returns an empty ErrorHook struct
func NewErrorHook(w io.Writer) *ErrorHook {
	return &ErrorHook{w: w}
}

// Fire dumps the entry.Message to Stderr
func (hook *ErrorHook) Fire(entry *logrus.Entry) error {
	err := fmt.Errorf("%s", entry.Message)
	fmt.Fprintf(hook.w, string(sf.FormatError(err)))

	return nil
}

// Levels returns the logging level that we want to subscribe
func (hook *ErrorHook) Levels() []logrus.Level {
	return []logrus.Level{
		logrus.FatalLevel,
	}
}
