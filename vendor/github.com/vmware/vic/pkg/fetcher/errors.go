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

package fetcher

import (
	"fmt"
	"net/url"
)

// DoNotRetry is an error wrapper indicating that the error cannot be resolved with a retry.
type DoNotRetry struct {
	Err error
}

// Error returns the stringified representation of the encapsulated error.
func (e DoNotRetry) Error() string {
	return fmt.Sprintf("download failed: %s", e.Err.Error())
}

// ImageNotFoundError is returned when an image is not found.
type ImageNotFoundError struct {
	Err error
}

func (e ImageNotFoundError) Error() string {
	return fmt.Sprintf("image not found: %s", e.Err.Error())
}

// TagNotFoundError is returned when an image's tag doesn't exist.
type TagNotFoundError struct {
	Err error
}

func (e TagNotFoundError) Error() string {
	return fmt.Sprintf("image tag not found: %s", e.Err.Error())
}

// AuthTokenError is returned when authentication with a registry fails
type AuthTokenError struct {
	TokenServer url.URL
	Err         error
}

func (e AuthTokenError) Error() string {
	return fmt.Sprintf("Failed to fetch auth token from %s", e.TokenServer.Host)
}
