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

package etcconf

import (
	"io/ioutil"
	"net"
	"os"
	"testing"

	log "github.com/Sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type entry struct {
	addr      net.IP
	hostnames []string
}

var (
	localv4 = &entry{
		addr:      net.ParseIP("127.0.0.1"),
		hostnames: []string{"localhost", "localhost.localdomain", "localhost4"},
	}
	localv6 = &entry{
		addr:      net.ParseIP("::1"),
		hostnames: []string{"localhost", "localhost.localdomain", "localhost6"},
	}
	independent = &entry{
		addr:      net.ParseIP("8.8.8.8"),
		hostnames: []string{"dns.google.com"},
	}
	name = &entry{
		addr:      net.ParseIP("192.168.0.2"),
		hostnames: []string{"system.host.name"},
	}
	namev6 = &entry{
		addr:      net.ParseIP("::2"),
		hostnames: []string{"system.host.name"},
	}
	alias = &entry{
		addr:      net.ParseIP("192.168.0.2"),
		hostnames: []string{"other.host.name"},
	}
)

// initializes a hosts file from entry, adds name and alias, and saves the file
func initializeHostsFile(t *testing.T) *hosts {
	log.SetLevel(log.DebugLevel)

	f, err := ioutil.TempFile("", "hosts-test")
	require.NoError(t, err, "Unable to create tmpfile")
	// defer os.Remove(f.Name())

	hosts := newHosts(f.Name())
	hosts.Load()
	require.Equal(t, 0, len(hosts.entries), "New hosts file contains entries")

	hosts.SetHost(name.hostnames[0], name.addr)
	hosts.SetHost(alias.hostnames[0], alias.addr)

	require.Equal(t, 1, len(hosts.entries), "Hosts was expected to have only one entry after non-local entry and alias added")

	err = hosts.Save()
	require.NoError(t, err, "Failed to save hosts file")

	return hosts
}

// TestAddressTypeIsolation checks that hostnames can be assigned to multiple
// addresses so long as those addresses are of different types. Types are limited
// to IPv4 and IPv6 at this time.
func TestAddressTypeIsolation(t *testing.T) {
	log.SetLevel(log.DebugLevel)

	f, err := ioutil.TempFile("", "hosts-test")
	require.NoError(t, err, "Unable to create tmpfile")
	defer os.Remove(f.Name())

	hosts := newHosts(f.Name())
	hosts.Load()
	require.Equal(t, 0, len(hosts.entries), "New hosts file contains entries")

	hosts.SetHost(localv4.hostnames[0], localv4.addr)
	hosts.SetHost(localv4.hostnames[1], localv4.addr)
	hosts.SetHost(localv4.hostnames[2], localv4.addr)

	require.Equal(t, 1, len(hosts.entries), "Hosts was expected to have only one entry after IPv4 local aliases added")

	hosts.SetHost(localv6.hostnames[0], localv6.addr)
	hosts.SetHost(localv6.hostnames[1], localv6.addr)
	hosts.SetHost(localv6.hostnames[2], localv6.addr)

	require.Equal(t, 2, len(hosts.entries), "Hosts was expected to have two entries after IPv6 local aliases added")

	localhostIPs := hosts.HostIP("localhost")
	require.Equal(t, 2, len(localhostIPs), "Expected two results for localhost lookup")
	ipv4 := localhostIPs[0].To4() != nil
	require.Equal(t, !ipv4, localhostIPs[1].To4() != nil, "Expected one address for localhost to be IPv6")

	require.True(t, localhostIPs[0].IsLoopback(), "Expected localhost IP to be loopback: %s", localhostIPs[0].String())
	require.True(t, localhostIPs[1].IsLoopback(), "Expected localhost IP to be loopback: %s", localhostIPs[1].String())

	err = hosts.Save()
	require.NoError(t, err, "Failed to save hosts file")

	// confirm preservation across load/save - overwrite previous hosts to avoid cross referencing accidents
	hosts = newHosts(f.Name())
	hosts.Load()

	localhostIPs = hosts.HostIP("localhost")
	require.Equal(t, 2, len(localhostIPs), "Expected two results for localhost lookup")
	ipv4 = localhostIPs[0].To4() != nil
	require.Equal(t, !ipv4, localhostIPs[1].To4() != nil, "Expected one address for localhost to be IPv6")

	require.True(t, localhostIPs[0].IsLoopback(), "Expected localhost IP to be loopback: %s", localhostIPs[0].String())
	require.True(t, localhostIPs[1].IsLoopback(), "Expected localhost IP to be loopback: %s", localhostIPs[1].String())

}

// TestAddressReassignment confirms that if the IP address is updated for a given
// hostname then it's updated for all of the current aliases (i.e. first field in hosts
// entry is the only thing that changes)
func TestAddressReassignment(t *testing.T) {
	log.SetLevel(log.DebugLevel)

	hosts := initializeHostsFile(t)
	f := hosts.Path()

	// confirm preservation across load/save - overwrite previous hosts to avoid cross referencing accidents
	hosts = newHosts(f)
	hosts.Load()

	// changes the IP address for the entire entry
	hosts.SetHost(name.hostnames[0], net.ParseIP("192.168.0.3"))
	ips := hosts.HostIP(name.hostnames[0])
	require.Equal(t, 1, len(ips), "Expected only one results for name lookup")
	require.Equal(t, "192.168.0.3", ips[0].String(), "Expected update of address to take effect")

	// and check the alias is updated as well
	ips = hosts.HostIP(alias.hostnames[0])
	require.Equal(t, 1, len(ips), "Expected only one results for name lookup")
	require.Equal(t, "192.168.0.3", ips[0].String(), "Expected update of address to take effect")
}

// TestNameReassignment confirms that if a name is Removed then set again with a different
// address it does not cause updates to aliases prior to the Removal.
func TestNameReassignment(t *testing.T) {
	log.SetLevel(log.DebugLevel)

	hosts := initializeHostsFile(t)
	f := hosts.Path()

	// confirm preservation across load/save - overwrite previous hosts to avoid cross referencing accidents
	hosts = newHosts(f)
	hosts.Load()

	// This is the primary different from TestAddressReassignment test
	hosts.RemoveHost(name.hostnames[0])

	// changes the IP address for the entire entry
	hosts.SetHost(name.hostnames[0], net.ParseIP("192.168.0.3"))
	ips := hosts.HostIP(name.hostnames[0])
	require.Equal(t, 1, len(ips), "Expected only one results for name lookup")
	require.Equal(t, "192.168.0.3", ips[0].String(), "Expected update of address to take effect")

	// and check the alias is NOT updated as well
	ips = hosts.HostIP(alias.hostnames[0])
	require.Equal(t, 1, len(ips), "Expected only one results for name lookup")
	require.Equal(t, "192.168.0.2", ips[0].String(), "Expected alias entry not to have been updated to take effect")
}

// TestNoChange confirms that performing an operation that should not cause a change causes
// no update
func TestNoChange(t *testing.T) {
	hosts := initializeHostsFile(t)
	f := hosts.Path()

	finfo, err := os.Stat(f)
	require.NoError(t, err, "Unable to stat hosts file")
	modTime := finfo.ModTime()

	// perform the no-op update
	ip := hosts.HostIP(name.hostnames[0])
	hosts.SetHost(name.hostnames[0], ip[0])

	// save the file - this should be a a no-op
	err = hosts.Save()
	require.NoError(t, err, "Unable to save hosts file")

	finfo, err = os.Stat(f)
	require.NoError(t, err, "Unable to stat hosts file for verification")

	assert.Equal(t, modTime, finfo.ModTime(), "Expected modification time to be unchanged")
}

// TestLocalToPublic checks the behaviour of reassignment for a hostname that is currently
// assigned to a local address with the new address being public.
// This is a distinct case as we should NOT be moving the standard localhost aliases.
func TestLocalToPublic(t *testing.T) {
	t.Skipf("Unimplemented - current implementation behaviour does not reflect this requirement")
}

// TestPublicToLocal checks the behaviour of reassignment for a hostname that is currently
// assigned to a public address with the new address being private.
// This is a distinct case as we should NOT be moving any aliases
func TestPublicToLocal(t *testing.T) {
	t.Skipf("Unimplemented - current implementation behaviour does not reflect this requirement")
}
