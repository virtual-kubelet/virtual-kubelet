// Copyright 2016 VMware, Inc. All Rights Reserved.
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

// Package errors provides error handling functions.
//
package errors

import (
	"fmt"
)

func ErrorStack(err error) string {
	return err.Error()
}

func Errorf(format string, a ...interface{}) error {
	return fmt.Errorf(format, a...)
}

func New(err string) error {
	return fmt.Errorf("%s", err)
}

func Trace(err error) error {
	if err == nil {
		return nil
	}
	return err
}
