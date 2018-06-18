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
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	mdns "github.com/miekg/dns"
)

// Item represents an item in the cache
type Item struct {
	Expiration time.Time
	Msg        *mdns.Msg
}

// CacheOptions represents the cache options
type CacheOptions struct {
	// Max capacity of cache, after this limit cache starts to evict random elements
	capacity int
	// Default ttl used by items
	ttl time.Duration
}

// Cache stores dns.Msgs and their expiration time
type Cache struct {
	CacheOptions

	// Protects following map
	sync.RWMutex
	m map[string]*Item

	// atomic cache hits & misses counters
	// ^ cause we update them while holding the read lock
	hits   uint64
	misses uint64
}

// NewCache returns a new cache
func NewCache(options CacheOptions) *Cache {
	return &Cache{
		CacheOptions: options,
		m:            make(map[string]*Item, options.capacity),
	}
}

// Capacity returns the capacity of the cache
func (c *Cache) Capacity() int {
	return c.capacity
}

// Count returns the element count of the cache
func (c *Cache) Count() int {
	c.RLock()
	defer c.RUnlock()
	return len(c.m)
}

func generateKey(q mdns.Question) string {
	return fmt.Sprintf("%s:%s", q.Name, mdns.TypeToString[q.Qtype])
}

// Add adds dns.Msg to the cache
func (c *Cache) Add(msg *mdns.Msg) {
	c.Lock()
	defer c.Unlock()

	if len(c.m) >= c.capacity {
		// pick a random key and remove it
		for k := range c.m {
			delete(c.m, k)
			break
		}
	}

	key := generateKey(msg.Question[0])
	if _, ok := c.m[key]; !ok {
		c.m[key] = &Item{
			Expiration: time.Now().UTC().Add(c.ttl),
			Msg:        msg.Copy(),
		}
	}
}

// Remove removes the dns.Msg from the cache
func (c *Cache) Remove(msg *mdns.Msg) {
	c.Lock()
	defer c.Unlock()

	if len(c.m) <= 0 {
		return
	}

	key := generateKey(msg.Question[0])
	delete(c.m, key)
}

// Get returns the dns.Msg from the cache
func (c *Cache) Get(msg *mdns.Msg) *mdns.Msg {
	key := generateKey(msg.Question[0])

	c.RLock()
	e, ok := c.m[key]
	c.RUnlock()

	if ok {
		atomic.AddUint64(&c.hits, 1)

		if time.Since(e.Expiration) < 0 {
			return e.Msg.Copy()
		}
		// Expired msg, remove it from the cache
		c.Remove(msg)
	} else {
		atomic.AddUint64(&c.misses, 1)
	}
	return nil
}

// Hits returns the number of cache hits
func (c *Cache) Hits() uint64 {
	return atomic.LoadUint64(&c.hits)
}

// Misses returns the number of cache misses
func (c *Cache) Misses() uint64 {
	return atomic.LoadUint64(&c.misses)
}

// Reset resets the cache
func (c *Cache) Reset() {
	c.Lock()
	defer c.Unlock()

	// drop the old map for GC and reset counters
	c.m = make(map[string]*Item, c.capacity)
	atomic.StoreUint64(&c.hits, 0)
	atomic.StoreUint64(&c.misses, 0)

}
