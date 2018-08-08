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

package registry

import (
	"crypto/x509"
	"fmt"
	"net/url"
	"time"

	log "github.com/Sirupsen/logrus"

	urlfetcher "github.com/vmware/vic/pkg/fetcher"
)

// Reachable test if a registry is a valid registry for VIC use and returns a url with scheme prepended
func Reachable(registry, scheme, username, password string, registryCAs *x509.CertPool, timeout time.Duration, skipVerify bool) (string, error) {
	registryPath := fmt.Sprintf("%s/v2/", registry)
	if scheme != "" {
		registryPath = fmt.Sprintf("%s://%s/v2/", scheme, registry)
	}

	url, err := url.Parse(registryPath)
	if err != nil {
		return "", err
	}
	log.Debugf("URL: %s", url)

	fetcher := urlfetcher.NewURLFetcher(urlfetcher.Options{
		Timeout:            timeout,
		Username:           username,
		Password:           password,
		InsecureSkipVerify: skipVerify,
		RootCAs:            registryCAs,
	})

	headers, err := fetcher.Head(url)
	if err != nil {
		return "", err
	}
	// v2 API requires this check
	if headers.Get("Docker-Distribution-API-Version") != "registry/2.0" {
		return "", fmt.Errorf("Missing Docker-Distribution-API-Version header")
	}
	return registryPath, nil
}
