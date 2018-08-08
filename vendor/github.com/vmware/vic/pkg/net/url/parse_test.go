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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseURL(t *testing.T) {
	var hosts = []string{
		"host.domain.com",
		"host.domain.com:123",
		"1.2.3.4",
		"1.2.3.4:10",
		"[2001:4860:0:2001::68]",
		"[2001:db8:1f70::999:de8:7648:6e8]:123",
	}

	for _, urlString := range hosts {
		u, err := ParseURL(urlString)
		assert.Nil(t, err)
		assert.Equal(t, u.String(), "https://"+urlString)
		// Null the scheme
		u.Scheme = ""
		assert.Equal(t, u.String(), "//"+urlString)
		assert.Equal(t, u.Host, urlString)
	}

	// Add path to create a more significant URL
	var urls = []string{}

	for i, h := range hosts {
		url := fmt.Sprintf("%s/path%d/test", h, i)
		urls = append(urls, url)
	}

	for i, urlString := range urls {
		u, err := ParseURL(urlString)
		assert.Nil(t, err)
		assert.Equal(t, u.String(), "https://"+urlString)

		// Null the scheme
		u.Scheme = ""
		assert.Equal(t, u.String(), "//"+urlString)

		// Check host
		assert.Equal(t, u.Host, hosts[i])
		// Check path
		path := fmt.Sprintf("/path%d/test", i)
		assert.Equal(t, u.Path, path)
		// Check concatenation
		assert.Equal(t, u.Host+u.Path, urlString)
	}

	// Add an HTTP scheme to verify that it is preserved
	var urlsWithHTTPScheme = []string{}

	for _, u := range urls {
		uws := fmt.Sprintf("http://%s", u)
		urlsWithHTTPScheme = append(urlsWithHTTPScheme, uws)
	}

	for _, urlString := range urlsWithHTTPScheme {
		u, err := ParseURL(urlString)
		fmt.Printf("UrlString: %s\n", u.String())
		assert.Nil(t, err)
		assert.Equal(t, u.String(), urlString)
	}

	var invalidUrls = []string{
		"[2001:db8/path",
		"1.2.3.4\\path",
	}

	for _, urlString := range invalidUrls {
		_, err := ParseURL(urlString)
		assert.NotNil(t, err)
	}
}
