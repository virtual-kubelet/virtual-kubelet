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

package errors

import (
	"fmt"
	"github.com/cpuguy83/strongerrors"
)

// DeadlineWithMessage creates a "Deadline" error with the specified message and formatting arguments.
func DeadlineWithMessage(format string, args ...interface{}) error {
	return strongerrors.Deadline(fmt.Errorf(format, args...))
}

// Exhausted creates an "Exhausted" error from the specified error.
func Exhausted(err error) error {
	return strongerrors.Exhausted(err)
}

// InvalidArgument creates an "InvalidArgument" error from the specified error.
func InvalidArgument(err error) error {
	return strongerrors.InvalidArgument(err)
}

// InvalidArgumentWithMessage creates an "InvalidArgument" error with the specified message and formatting arguments.
func InvalidArgumentWithMessage(format string, args ...interface{}) error {
	return strongerrors.InvalidArgument(fmt.Errorf(format, args...))
}

// Unknown creates an "Unknown" error from the specified error and optional message and formatting arguments.
func Unknown(err error, args ...interface{}) error {
	// If we've been given additional arguments, re-format the error
	if len(args) >= 1 {
		err = fmt.Errorf("%s: %s", fmt.Sprintf(args[0].(string), args[1:]...), err)
	}
	return strongerrors.Unknown(err)
}
