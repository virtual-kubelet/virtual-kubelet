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
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
)

const (
	ResolvConfPath       = "/etc/resolv.conf"
	DefaultAttempts uint = 5
	DefaultTimeout       = 15 * time.Second
)

type ResolvConf interface {
	Conf

	AddNameservers(...net.IP)
	RemoveNameservers(...net.IP)
	Nameservers() []net.IP
	Attempts() uint
	Timeout() time.Duration
	SetAttempts(uint)
	SetTimeout(time.Duration)
}

type resolvConf struct {
	sync.Mutex

	EntryConsumer

	dirty       bool
	path        string
	nameservers []net.IP
	timeout     time.Duration
	attempts    uint
}

type resolvConfWalker struct {
	lines []string
	i     int
}

func (w *resolvConfWalker) HasNext() bool {
	return w.i < len(w.lines)
}

func (w *resolvConfWalker) Next() string {
	s := w.lines[w.i]
	w.i++
	return s
}

func NewResolvConf(path string) ResolvConf {
	if path == "" {
		path = ResolvConfPath
	}

	return &resolvConf{
		path:     path,
		timeout:  DefaultTimeout,
		attempts: DefaultAttempts,
	}
}

func (r *resolvConf) ConsumeEntry(t string) error {
	r.Lock()
	defer r.Unlock()

	fs := strings.Fields(t)
	if len(fs) < 2 {
		log.Warnf("skipping invalid line %q", t)
		return nil
	}

	switch fs[0] {
	case "nameserver":
		ip := net.ParseIP(fs[1])
		if ip == nil {
			log.Warnf("skipping invalid line %q: invalid ip address", t)
			return nil
		}

		r.addNameservers(ip)
	case "options":
		parts := strings.Split(fs[1], ":")
		if len(parts) > 2 {
			log.Warnf("skipping invalid line %q", t)
			return nil
		}

		var v uint
		switch parts[0] {
		case "timeout":
			fallthrough
		case "attempts":
			if len(parts) < 2 {
				log.Warnf("skipping invalid line %q", t)
				return nil
			}

			o, err := strconv.ParseUint(parts[1], 10, strconv.IntSize)
			if err != nil {
				log.Warnf("skipping invalid line %q: %s", t, err)
				return nil
			}
			v = uint(o)
		}

		switch parts[0] {
		case "timeout":
			r.timeout = time.Duration(v) * time.Second
		case "attempts":
			r.attempts = v
		}
	}

	return nil
}

func (r *resolvConf) Copy(conf Conf) error {
	existing := conf.(*resolvConf)

	existing.Lock()
	defer existing.Unlock()

	r.nameservers = existing.nameservers
	r.dirty = true

	return r.Save()
}

func (r *resolvConf) Load() error {
	r.Lock()
	defer r.Unlock()

	rc := &resolvConf{}
	if err := load(r.path, rc); err != nil {
		return err
	}

	r.nameservers = rc.nameservers
	return nil
}

func (r *resolvConf) Save() error {
	r.Lock()
	defer r.Unlock()

	log.Debugf("%+v", r)
	if !r.dirty {
		return nil
	}

	walker := &resolvConfWalker{lines: r.lines()}
	log.Debugf("%+v", walker)
	if err := save(r.path, walker); err != nil {
		return err
	}

	// make sure the file is readable
	// #nosec: Expect file permissions to be 0600 or less
	if err := os.Chmod(r.path, 0644); err != nil {
		return err
	}

	r.dirty = false
	return nil
}

func (r *resolvConf) AddNameservers(nss ...net.IP) {
	r.Lock()
	defer r.Unlock()

	r.addNameservers(nss...)
}

func (r *resolvConf) addNameservers(nss ...net.IP) {
	for _, n := range nss {
		if n == nil {
			continue
		}

		found := false
		for _, rn := range r.nameservers {
			if rn.Equal(n) {
				found = true
				break
			}
		}

		if !found {
			r.nameservers = append(r.nameservers, n)
			r.dirty = true
		}
	}
}

func (r *resolvConf) RemoveNameservers(nss ...net.IP) {
	r.Lock()
	defer r.Unlock()

	for _, n := range nss {
		if n == nil {
			continue
		}

		for i, rn := range r.nameservers {
			if n.Equal(rn) {
				r.nameservers = append(r.nameservers[:i], r.nameservers[i+1:]...)
				r.dirty = true
				break
			}
		}
	}
}

func (r *resolvConf) Nameservers() []net.IP {
	r.Lock()
	defer r.Unlock()

	return r.nameservers
}

func (r *resolvConf) Timeout() time.Duration {
	return r.timeout
}

func (r *resolvConf) Attempts() uint {
	return r.attempts
}

func (r *resolvConf) SetTimeout(t time.Duration) {
	r.timeout = t
	r.dirty = true
}

func (r *resolvConf) SetAttempts(attempts uint) {
	if attempts > 0 {
		r.attempts = attempts
		r.dirty = true
	}
}

func (r *resolvConf) Path() string {
	return r.path
}

func (r *resolvConf) lines() []string {
	var l []string
	for _, n := range r.nameservers {
		l = append(l, fmt.Sprintf("nameserver %s", n))
	}

	l = append(l, []string{
		fmt.Sprintf("options timeout:%d", r.timeout/time.Second),
		fmt.Sprintf("options attempts:%d", r.attempts),
	}...)

	return l
}
