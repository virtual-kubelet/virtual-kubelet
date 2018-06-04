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

package errors

import (
	"fmt"
)

// InternalError is returned when there is internal issue
type InternalError struct {
	Message string
}

func (e InternalError) Error() string {
	return e.Message
}

type DataTypeError struct {
	ExpectedType string
}

func (e DataTypeError) Error() string {
	return fmt.Sprintf("Data type is not %s", e.ExpectedType)
}

type KeyNotFound struct {
	Key     string
	Message string
}

func (e KeyNotFound) Error() string {
	return fmt.Sprintf("key %s is not found: %s", e.Key, e.Message)
}

type InvalidMigrationVersion struct {
	Version string
	Err     error
}

func (e InvalidMigrationVersion) Error() string {
	return fmt.Sprintf("Data migration version is invalid %s: %s", e.Version, e.Err)
}

type DecodeError struct {
	Err error
}

func (e DecodeError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("Failed to decode data: %s", e.Err)
	}

	return fmt.Sprintf("Failed to decode data")
}

type ValueFormatError struct {
	Key   string
	Value interface{}
}

func (e ValueFormatError) Error() string {
	return fmt.Sprintf("Failed to convert value of key %s: %#v", e.Key, e.Value)
}
