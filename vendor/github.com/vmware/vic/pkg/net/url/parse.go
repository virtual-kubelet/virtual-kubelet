// Copyright 2018 VMware, Inc. All Rights Reserved.
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

package url

import (
	"net/url"
	"regexp"
)

var schemeMatch = regexp.MustCompile(`^\w+://`)

// Starting from Go 1.8 the URL parser does not
// work properly with URLs with no Scheme,
// this function adds "https" as Scheme if necessary
func ParseURL(s string) (*url.URL, error) {
	var err error
	var u *url.URL

	if s != "" {
		// Default the scheme to https
		if !schemeMatch.MatchString(s) {
			s = "https://" + s
		}

		u, err = url.Parse(s)
		if err != nil {
			return nil, err
		}
	}

	return u, nil
}
