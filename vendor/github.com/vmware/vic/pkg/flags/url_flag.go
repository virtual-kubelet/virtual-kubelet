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

package flags

import (
	"flag"
	"net/url"
	"regexp"
)

var schemeMatch = regexp.MustCompile(`^\w+://`)

type URLFlag struct {
	u **url.URL
}

// Set will add a protocol (https) if there isn't a :// match. This
// ensures tha url.Parse can correctly extract user:password from
// raw URLs such as user:password@hostname
func (f *URLFlag) Set(s string) error {
	var err error
	// Default the scheme to https
	if !schemeMatch.MatchString(s) {
		s = "https://" + s
	}

	url, err := url.Parse(s)
	*f.u = url
	return err
}

func (f *URLFlag) Get() interface{} {
	return *f.u
}

func (f *URLFlag) String() string {
	if f.u == nil || *f.u == nil {
		return "<nil>"
	}
	return (*f.u).String()
}

func (f *URLFlag) IsBoolFlag() bool { return false }

// NewURLFlag returns a flag.Value.
func NewURLFlag(u **url.URL) flag.Value {
	return &URLFlag{u}
}
