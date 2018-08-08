// Copyright 2017-2018 VMware, Inc. All Rights Reserved.
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

// Package errors provides constructors and wrappers for http-aware errors.
package errors

import (
	"fmt"
	"net/http"
)

// StatusCode extracts the HTTP status code associated with an error, falling
// back to 500 Internal Server Error if none has been specified.
func StatusCode(err error) int {
	e, ok := err.(statusCode)
	if !ok {
		return http.StatusInternalServerError
	}

	return e.Code()
}

// NewError constructs a new error with the supplied status code and message.
func NewError(code int, message string, a ...interface{}) error {
	if a != nil {
		return httpError{code: code, message: fmt.Sprintf(message, a...)}
	}
	return httpError{code: code, message: message}
}

// WrapError wraps a status code around an existing error.
func WrapError(code int, err error) error {
	return wrappedError{error: err, code: code}
}

// statusCode is used by the error implementations within the package to
// expose a status code to StatusCode.
//
// This pattern is based on https://dave.cheney.net/2016/04/27/dont-just-check-errors-handle-them-gracefully.
type statusCode interface {
	Code() int
}

type httpError struct {
	code    int
	message string
}

func (e httpError) Code() int {
	return e.code
}

func (e httpError) Error() string {
	return e.message
}

type wrappedError struct {
	error
	code int
}

func (e wrappedError) Code() int {
	return e.code
}

func (e wrappedError) Error() string {
	return e.error.Error()
}
