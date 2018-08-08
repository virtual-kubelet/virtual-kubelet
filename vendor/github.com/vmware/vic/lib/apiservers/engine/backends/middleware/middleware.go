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

package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"golang.org/x/net/context"

	vicdns "github.com/vmware/vic/lib/dns"
)

// HostCheckMiddleware provides middleware for Host header correctness enforcement
type HostCheckMiddleware struct {
	ValidDomains vicdns.SetOfDomains
}

// validateHostname trims the port off the Host header in an HTTP request and returns either the bare IP (v4 or v6) or the FQDN with the port truncated. Returns non-nil error if Host field doesn't make sense.
func validateHostname(r *http.Request) (hostname string, err error) {
	if r.Host == "" {
		// this really shouldn't be necessary https://tools.ietf.org/html/rfc2616#section-14.23
		// you can delete this if stanza if you're braver than me.
		return "", fmt.Errorf("empty host header from %s", r.RemoteAddr)
	}

	if r.Host[len(r.Host)-1] == ']' {
		// ipv6 w/o specified port
		return r.Host, nil
	}

	// trim port if it's there. r.Host should never contain a scheme
	hostnameSplit := strings.Split(r.Host, ":")

	if len(hostnameSplit) <= 2 {
		// ipv4 or dns hostname with or without port, first element is hostname
		return hostnameSplit[0], nil
	}

	// if we see >2 colons in the hostname, it's an ipv6 address w/ port
	// unfortunately that means we have to recombine the rest..
	return fmt.Sprintf("%s", strings.Join(hostnameSplit[:len(hostnameSplit)-1], ":")), nil
}

// WrapHandler satisfies the Docker middleware interface for HostCheckMiddleware to reject http requests that do not specify a known DNS name for this endpoint in the Host: field of the request
func (h HostCheckMiddleware) WrapHandler(f func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error) func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (err error) {
		var hostname string
		if hostname, err = validateHostname(r); err != nil {
			return err
		}

		if h.ValidDomains[hostname] {
			return f(ctx, w, r, vars)
		}

		return fmt.Errorf("invalid host header from %s to requested host %s", r.RemoteAddr, r.Host)
	}
}
