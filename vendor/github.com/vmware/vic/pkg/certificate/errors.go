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

package certificate

import "fmt"

// CertParseError is returned when there's an error parsing a cert.
type CertParseError struct {
	msg string
}

func (e CertParseError) Error() string {
	return fmt.Sprintf("Unable to parse client certificate: %s", e.msg)
}

// CreateCAPoolError is returned when there's an error creating a CA cert pool.
type CreateCAPoolError struct{}

func (e CreateCAPoolError) Error() string {
	return "Unable to create CA pool to check client certificate"
}

// CertVerifyError is returned when the client cert cannot be validated against the CA.
type CertVerifyError struct{}

func (e CertVerifyError) Error() string {
	return "Client certificate in certificate path does not validate with provided CA"
}
