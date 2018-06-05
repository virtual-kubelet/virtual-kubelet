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

package util

import (
	"fmt"
	"net/http"
	"testing"
)

func TestNewError(t *testing.T) {
	e := NewError(123, "new error %d %s %q")
	c := StatusCode(e)

	if c != 123 {
		t.Errorf("Status code was %d, not 123.", c)
	}

	if e.Error() != "new error %d %s %q" {
		t.Error("NewError did not preserve message.")
	}
}

func TestNewErrorWithArgs(t *testing.T) {
	e := NewError(123, "new error %d %s %q", 1, "a", "foo")
	c := StatusCode(e)

	if c != 123 {
		t.Errorf("Status code was %d, not 123.", c)
	}

	if e.Error() != "new error 1 a \"foo\"" {
		t.Error("NewError did not preserve message.")
	}
}

func TestWrappedError(t *testing.T) {
	e := WrapError(234, fmt.Errorf("fmt error"))
	c := StatusCode(e)

	if c != 234 {
		t.Errorf("Status code was %d, not 234.", c)
	}

	if e.Error() != "fmt error" {
		t.Error("WrapError did not preserve message.")
	}
}

func TestDoublyWrappedError(t *testing.T) {
	e := WrapError(234, WrapError(123, fmt.Errorf("fmt error")))
	c := StatusCode(e)

	if c != 234 {
		t.Errorf("Status code was %d, not 234.", c)
	}
}

func TestStatusCodeFallback(t *testing.T) {
	e := fmt.Errorf("fmt error")
	c := StatusCode(e)

	if c != http.StatusInternalServerError {
		t.Errorf("Default status code was %d, not 500.", c)
	}
}
