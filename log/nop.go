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

package log

type nopLogger struct{}

func (nopLogger) Debug(...interface{})          {}
func (nopLogger) Debugf(string, ...interface{}) {}
func (nopLogger) Info(...interface{})           {}
func (nopLogger) Infof(string, ...interface{})  {}
func (nopLogger) Warn(...interface{})           {}
func (nopLogger) Warnf(string, ...interface{})  {}
func (nopLogger) Error(...interface{})          {}
func (nopLogger) Errorf(string, ...interface{}) {}
func (nopLogger) Fatal(...interface{})          {}
func (nopLogger) Fatalf(string, ...interface{}) {}

func (l nopLogger) WithField(string, interface{}) Logger { return l }
func (l nopLogger) WithFields(Fields) Logger             { return l }
func (l nopLogger) WithError(error) Logger               { return l }
