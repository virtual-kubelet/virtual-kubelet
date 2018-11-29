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
	goerrors "errors"
	"testing"

	"github.com/cpuguy83/strongerrors"
)

func TestUnknownFromRoot(t *testing.T) {
	// Create a root cause.
	// This represents an error returned by a function invocation.
	err1 := goerrors.New("root cause")
	// Create an error of type "Unknown" with err1 as the cause and containing an additional message.
	err2 := Unknown(err1)

	// Make sure that the resulting error is indeed of type "Unknown".
	if !strongerrors.IsUnknown(err2) {
		t.Fatal("expected err2 to be of type Unknown")
	}
	// Make sure that the error's message is equal to the root cause's message.
	if err2.Error() != "root cause" {
		t.Fatal("expected err2's error message to be equal to the root cause's message")
	}
}

func TestUnknownFromRootCauseWithMessage(t *testing.T) {
	// Create a root cause.
	// This represents an error returned by a function invocation.
	err1 := goerrors.New("root cause")
	// Create an error of type "Unknown" with err1 as the cause and containing an additional message.
	err2 := Unknown(err1, "extra info")

	// Make sure that the resulting error is indeed of type "Unknown".
	if !strongerrors.IsUnknown(err2) {
		t.Fatal("expected err2 to be of type Unknown")
	}
	// Make sure that the message was adequately formatted.
	if err2.Error() != "extra info: root cause" {
		t.Fatal("expected err2 to be adequately formatted as a string")
	}
}

func TestUnknownFromRootCauseWithMessageAndFormattingParameters(t *testing.T) {
	// Create a root cause.
	// This represents an error returned by a function invocation.
	err1 := goerrors.New("root cause")
	// Create an error of type "Unknown" with err1 as the cause and containing an additional message.
	err2 := Unknown(err1, "expected %d, got %d", 0, 1)

	// Make sure that the resulting error is indeed of type "Unknown".
	if !strongerrors.IsUnknown(err2) {
		t.Fatal("expected err2 to be of type Unknown")
	}
	// Make sure that the message was adequately formatted.
	if err2.Error() != "expected 0, got 1: root cause" {
		t.Fatal("expected err2 to be adequately formatted as a string")
	}
}
