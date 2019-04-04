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

package session

import "fmt"

// SDKURLError is returned when the soap SDK URL cannot be parsed
type SDKURLError struct {
	Service string
	Err     error
}

func (e SDKURLError) Error() string {
	return fmt.Sprintf("SDK URL (%s) could not be parsed: %s", e.Service, e.Err)
}

// SoapClientError is returned when we're unable to obtain a vim client
type SoapClientError struct {
	Host string
	Err  error
}

func (e SoapClientError) Error() string {
	return fmt.Sprintf("Failed to connect to %s: %s", e.Host, e.Err)
}

// UserPassLoginError is returned when login via username/password is unsuccessful
type UserPassLoginError struct {
	Host string
	Err  error
}

func (e UserPassLoginError) Error() string {
	return fmt.Sprintf("Failed to log in to %s: %s", e.Host, e.Err)
}
