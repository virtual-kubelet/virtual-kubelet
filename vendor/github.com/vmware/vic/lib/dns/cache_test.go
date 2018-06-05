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

package dns

import (
	"math/rand"
	"sync"
	"testing"
	"time"

	mdns "github.com/miekg/dns"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func RandStringRunes(n int) string {
	runes := []rune("abcdefghijklmnopqrstuvwxyz")

	b := make([]rune, n)
	for i := range b {
		b[i] = runes[rand.Intn(len(runes))]
	}
	return string(b)
}

func RandDNSType() uint16 {
	types := []uint16{
		mdns.TypeA,
		mdns.TypeAAAA,
	}
	return types[rand.Intn(len(types))]
}

func NewMsg() *mdns.Msg {
	m := &mdns.Msg{}
	name := RandStringRunes(16) + ".vmware.local."
	qtype := RandDNSType()
	m.SetQuestion(name, qtype)
	return m
}

func TestCache(t *testing.T) {
	o := CacheOptions{
		capacity: 10,
		ttl:      10 * time.Second,
	}
	c := NewCache(o)

	var msgs []*mdns.Msg
	for i := 0; i < 10; i++ {
		msgs = append(msgs, NewMsg())
	}

	for _, msg := range msgs {
		c.Add(msg)
	}
	if c.Count() != len(msgs) {
		t.Fatalf("Add failed")
	}

	for _, msg := range msgs {
		m := c.Get(msg)
		if m.Question[0] != msg.Question[0] {
			t.Fatalf("Get failed")
		}
	}

	for _, msg := range msgs {
		c.Remove(msg)
	}
	if c.Count() != 0 {
		t.Fatalf("Remove failed")
	}
}

func TestCapacity(t *testing.T) {
	o := CacheOptions{
		capacity: 10,
		ttl:      10 * time.Second,
	}
	c := NewCache(o)

	var msgs []*mdns.Msg
	for i := 0; i < 100; i++ {
		msgs = append(msgs, NewMsg())
	}

	for _, msg := range msgs {
		c.Add(msg)
	}

	if c.Count() != c.Capacity() {
		t.Fatalf("Add failed")
	}
}

func TestReset(t *testing.T) {
	o := CacheOptions{
		capacity: 10,
		ttl:      10 * time.Second,
	}
	c := NewCache(o)

	var msgs []*mdns.Msg
	for i := 0; i < 10; i++ {
		msgs = append(msgs, NewMsg())
	}

	for _, msg := range msgs {
		c.Add(msg)
	}
	if c.Count() != len(msgs) {
		t.Fatalf("Add failed")
	}

	c.Reset()

	if c.Count() != 0 {
		t.Fatalf("Reset failed")
	}
}

func TestExpiration(t *testing.T) {
	o := CacheOptions{
		capacity: 100,
		ttl:      time.Nanosecond,
	}
	c := NewCache(o)

	var msgs []*mdns.Msg
	for i := 0; i < 100; i++ {
		msgs = append(msgs, NewMsg())
	}

	for _, msg := range msgs {
		c.Add(msg)
	}
	for _, msg := range msgs {
		// All of them should be expired
		if c.Get(msg) != nil {
			t.Fatalf("Get failed")
		}
	}
}

// For best result run with -race
func TestConcurrency(t *testing.T) {
	var wg sync.WaitGroup

	samplesize := 1 << 10
	o := CacheOptions{
		capacity: samplesize,
		ttl:      10 * time.Minute,
	}
	c := NewCache(o)

	// create a map so that we can iterate on it randomly
	msgs := make(map[int]*mdns.Msg)
	for i := 0; i < samplesize; i++ {
		msgs[i] = NewMsg()
	}

	writer := func() {
		for _, msg := range msgs {
			c.Add(msg)
		}
		wg.Done()
	}

	reader := func() {
		for _, msg := range msgs {
			c.Get(msg)
		}
		wg.Done()
	}

	remover := func() {
		for _, msg := range msgs {
			c.Remove(msg)
		}
		wg.Done()
	}

	// 3 writer
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go writer()
	}

	// 5 reader
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go reader()
	}

	// 2 remover
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go remover()
	}

	wg.Wait()
}
