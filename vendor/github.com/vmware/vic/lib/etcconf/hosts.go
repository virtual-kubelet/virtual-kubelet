// Copyright 2016-2017 VMware, Inc. All Rights Reserved.
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
	"bytes"
	"fmt"
	"net"
	"os"
	"sort"
	"strings"
	"sync"

	log "github.com/Sirupsen/logrus"
)

type Hosts interface {
	Conf

	SetHost(hostname string, ip net.IP)
	RemoveHost(hostname string)
	RemoveAll()

	HostIP(hostname string) []net.IP
}

type hostEntry struct {
	IP        net.IP
	Hostnames []string
	newAddr   bool
}

func (e *hostEntry) String() string {
	return fmt.Sprintf("%s %s", e.IP, strings.Join(e.Hostnames, " "))
}

func (e *hostEntry) addNames(names ...string) string {
	e.Hostnames = append(e.Hostnames, names...)
	sort.Strings(e.Hostnames)

	return e.IP.String() + " " + strings.Join(e.Hostnames, " ")
}

func (e *hostEntry) setAddress(ip net.IP) {
	if e.IP != nil {
		hostnames := strings.Join(e.Hostnames, " ")

		log.Infof("Changing IP address: %s -> %s", e.IP.String(), ip.String())
		log.Infof("IP change impacts the following hostnames: %s", hostnames)
		if e.newAddr {
			log.Warn("Address has changed more than once since last load, implying a configuration race")
		}
		e.newAddr = true
	}

	e.IP = ip
}

type hosts struct {
	sync.Mutex

	EntryConsumer

	hostsIPv4 map[string]*hostEntry
	hostsIPv6 map[string]*hostEntry
	entries   map[string]*hostEntry
	dirty     bool
	path      string
}

type hostsWalker struct {
	entries []*hostEntry
	i       int
}

func (w *hostsWalker) HasNext() bool {
	return w.i < len(w.entries)
}

func (w *hostsWalker) Next() string {
	s := w.entries[w.i].String()
	w.i++
	return s
}

func NewHosts(path string) Hosts {
	return newHosts(path)
}

func newHosts(path string) *hosts {
	if path == "" {
		path = HostsPath
	}

	return &hosts{
		path:      path,
		hostsIPv4: make(map[string]*hostEntry),
		hostsIPv6: make(map[string]*hostEntry),
		entries:   make(map[string]*hostEntry),
	}
}

func (h *hosts) ConsumeEntry(t string) error {
	h.Lock()
	defer h.Unlock()

	fs := strings.Fields(t)
	if len(fs) < 2 {
		log.Warnf("ignoring incomplete line %q", t)
		return nil
	}

	ip := net.ParseIP(fs[0])
	if ip == nil {
		log.Warnf("ignoring line %q due to invalid ip address", t)
		return nil
	}

	for _, hs := range fs[1:] {
		h.setHost(hs, ip)
	}

	return nil
}

// Needs to ensure that host entries don't occur twice, and that we have stable reconsiliation if they do
func (h *hosts) Load() error {
	h.Lock()
	defer h.Unlock()

	newHosts := newHosts(h.path)

	if err := load(h.path, newHosts); err != nil {
		return err
	}

	h.hostsIPv4 = newHosts.hostsIPv4
	h.hostsIPv6 = newHosts.hostsIPv6
	h.entries = newHosts.entries
	h.dirty = false
	return nil
}

// Seed pulls replaces the current state with that from the provided hosts
// It _does not_ perform a deep copy so performs a Save immediately.
func (h *hosts) Copy(conf Conf) error {
	// straight up panic if not the appropriate type
	existing := conf.(*hosts)

	existing.Lock()
	defer existing.Unlock()

	h.hostsIPv4 = existing.hostsIPv4
	h.hostsIPv6 = existing.hostsIPv6
	h.entries = existing.entries
	h.dirty = true

	return h.Save()
}

// ensure hostname is associated with localhost

func (h *hosts) Save() error {
	h.Lock()
	defer h.Unlock()

	if !h.dirty {
		log.Debugf("skipping writing file since there are no new entries")
		return nil
	}

	var entries []*hostEntry
	for _, v := range h.entries {
		entries = append(entries, v)
	}

	if err := save(h.path, &hostsWalker{entries: entries}); err != nil {
		return err
	}

	// make sure the file is readable
	// #nosec: Expect file permissions to be 0600 or less
	if err := os.Chmod(h.path, 0644); err != nil {
		return err
	}

	h.dirty = false
	return nil
}

func (h *hosts) SetHost(hostname string, ip net.IP) {
	h.Lock()
	defer h.Unlock()

	h.setHost(hostname, ip)
}

func (h *hosts) setHost(hostname string, ip net.IP) {
	h.dirty = true

	if ip == nil {
		return
	}

	// what type of address is it?
	hostmap := h.hostsIPv6
	ipv4 := ip.To4()
	if ipv4 != nil {
		// this drops any leading garbage in the array so that byte comparisons work as
		// expected
		ip = ipv4
		hostmap = h.hostsIPv4
	}

	hentry := hostmap[hostname]
	ientry := h.entries[ip.String()]

	if hentry != nil {
		// existing entry with no changes
		if bytes.Equal(hentry.IP, ip) {
			h.dirty = false
			return
		}

		// existing hostname with a new address - change address for all assocated hostnames
		hentry.setAddress(ip)
		return
	}

	// completely new entry for this ip address type
	if ientry == nil {
		entry := &hostEntry{
			IP:        ip,
			Hostnames: []string{hostname},
		}

		h.entries[ip.String()] = entry
		hostmap[hostname] = entry
		return
	}

	// add the hostname indexed IP record
	ientry.Hostnames = append(ientry.Hostnames, hostname)
	hostmap[hostname] = ientry

	return
}

func (h *hosts) RemoveHost(hostname string) {
	h.Lock()
	defer h.Unlock()

	for _, hostmap := range []map[string]*hostEntry{h.hostsIPv4, h.hostsIPv6} {
		entry := hostmap[hostname]
		if entry == nil {
			continue
		}

		h.dirty = true
		delete(hostmap, hostname)

		if len(entry.Hostnames) < 2 {
			log.Infof("Removing hostname and address: %s (%s)", hostname, entry.IP.String())
			delete(h.entries, entry.IP.String())
			continue
		}

		var remaining []string
		for i := range entry.Hostnames {
			if entry.Hostnames[i] == hostname {
				remaining = entry.Hostnames[:i]
				remaining = append(remaining, entry.Hostnames[i+1:]...)
			}
		}
		entry.Hostnames = remaining
	}
}

func (h *hosts) RemoveAll() {
	h.Lock()
	defer h.Unlock()

	h.dirty = len(h.hostsIPv4) > 0 || len(h.hostsIPv6) > 0

	h.hostsIPv4 = make(map[string]*hostEntry)
	h.hostsIPv6 = make(map[string]*hostEntry)
	h.entries = make(map[string]*hostEntry)
}

func (h *hosts) HostIP(hostname string) []net.IP {
	h.Lock()
	defer h.Unlock()

	var ips []net.IP
	if ipv4, ok := h.hostsIPv4[hostname]; ok {
		ips = append(ips, ipv4.IP)
	}
	if ipv6, ok := h.hostsIPv6[hostname]; ok {
		ips = append(ips, ipv6.IP)
	}

	return ips
}

func (h *hosts) Path() string {
	return h.path
}
