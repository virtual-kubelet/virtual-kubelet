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
	"net"
	"net/url"
	"strings"

	glob "github.com/ryanuber/go-glob"
)

type Entry interface {
	Contains(e Entry) bool
	Match(e string) bool
	Equal(other Entry) bool
	String() string

	IsCIDR() bool
	IsURL() bool
}

type URLEntry interface {
	Entry
	URL() *url.URL
}

func ParseEntry(s string) Entry {
	_, ipnet, err := net.ParseCIDR(s)
	if err == nil {
		return &cidrEntry{ipnet: ipnet}
	}

	if u := parseURL(s); u != nil {
		return &urlEntry{u: u}
	}

	return nil
}

type cidrEntry struct {
	ipnet *net.IPNet
}

func (c *cidrEntry) IsCIDR() bool {
	return true
}

func (c *cidrEntry) IsURL() bool {
	return false
}

func (c *cidrEntry) Contains(e Entry) bool {
	switch e := e.(type) {
	case *urlEntry:
		if ip := net.ParseIP(e.u.Hostname()); ip != nil {
			return c.ipnet.Contains(ip)
		}
	case *cidrEntry:
		return c.ipnet.Contains(e.ipnet.IP.Mask(e.ipnet.Mask))
	}

	return false
}

func (c *cidrEntry) Match(s string) bool {
	return c.Contains(ParseEntry(s))
}

func (c *cidrEntry) Equal(other Entry) bool {
	return other.String() == c.ipnet.String()
}

func (c *cidrEntry) String() string {
	return c.ipnet.String()
}

type urlEntry struct {
	u *url.URL
}

func (u *urlEntry) IsCIDR() bool {
	return false
}

func (u *urlEntry) IsURL() bool {
	return true
}

func ensurePort(u *url.URL) *url.URL {
	_, _, err := net.SplitHostPort(u.Host)
	if err == nil {
		return u // port already present
	}

	res := *u
	switch u.Scheme {
	case "http":
		res.Host = u.Host + ":80"
	case "https":
		res.Host = u.Host + ":443"
	}

	return &res
}

func (u *urlEntry) Contains(e Entry) bool {
	switch e := e.(type) {
	case *urlEntry:
		up := ensurePort(u.u)
		ep := ensurePort(e.u)
		if up.Port() == "" && ep.Port() != "" {
			ep.Host = ep.Hostname()
		}
		return (u.u.Scheme == "" || u.u.Scheme == e.u.Scheme) &&
			strings.HasPrefix(e.u.Path, u.u.Path) &&
			glob.Glob(up.Host, ep.Host)
	}

	return false
}

func (u *urlEntry) Match(s string) bool {
	q := ParseEntry(s)
	query, ok := q.(URLEntry)
	if !ok {
		return false
	}

	// copy u to nu ("new u") so we don't modify the record
	nu, _ := ParseEntry(u.String()).(URLEntry)

	if query.URL().Scheme != "" && nu.URL().Scheme == "" {
		query.URL().Scheme = nu.URL().Scheme
	} else if nu.URL().Scheme != "" && query.URL().Scheme == "" {
		nu.URL().Scheme = query.URL().Scheme
	}

	return nu.Contains(query)
}

func (u *urlEntry) String() string {
	if u.u.Scheme == "" {
		return strings.TrimPrefix(u.u.String(), "//")
	}

	return u.u.String()
}

func (u *urlEntry) Equal(other Entry) bool {
	if other, ok := other.(*urlEntry); ok {
		up := ensurePort(u.u)
		otherp := ensurePort(other.u)
		return up.String() == otherp.String()
	}

	return other.String() == u.u.String()
}

func (u *urlEntry) URL() *url.URL {
	return u.u
}

func parseURL(s string) *url.URL {
	for _, p := range []string{"", "https://"} {
		u, err := url.Parse(p + s)
		if err == nil && len(u.Host) > 0 {
			if p != "" {
				u.Scheme = "" // ignore the scheme
			}

			return u
		}
	}

	return nil
}
