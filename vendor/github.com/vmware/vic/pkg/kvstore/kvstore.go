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

package kvstore

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"sync"

	log "github.com/Sirupsen/logrus"
)

var (
	ErrKeyNotFound = errors.New("key not found")
)

// This package implements a very basic key/value store.  It is up to the
// caller to provision the namespace.

type KeyValueStore interface {
	// Set adds a new key or modifies an existing key in the key-value store
	Put(ctx context.Context, key string, value []byte) error

	// Get gets an existing key in the key-value store. Returns ErrKeyNotFound
	// if key does not exist the key-value store.
	Get(key string) ([]byte, error)

	// List lists the key-value pairs whose keys match the regular expression
	// passed in.
	List(re string) (map[string][]byte, error)

	// Delete deletes existing keys from the key-value store. Returns ErrKeyNotFound
	// if key does not exist the key-value store.
	Delete(ctx context.Context, key string) error

	// Save saves the key-value store data to the backend.
	Save(ctx context.Context) error

	// Name returns the unique identifier/name for the key-value store. This is
	// used to determine the path that is passed to the backend operations.
	Name() string
}

type kv struct {
	b    Backend
	kv   map[string][]byte
	name string
	l    sync.RWMutex
}

func fileName(name string) string {
	return fmt.Sprintf("%s.dat", name)
}

// Create a new KeyValueStore instance using the given Backend with the given
// file.  If the file exists on the Backend, it is restored.
func NewKeyValueStore(ctx context.Context, store Backend, name string) (KeyValueStore, error) {
	p := &kv{
		b:    store,
		kv:   make(map[string][]byte),
		name: name,
	}

	if err := p.restore(ctx); err != nil {
		return nil, err
	}

	log.Infof("NewKeyValueStore(%s) restored %d keys", name, len(p.kv))

	return p, nil
}

func (p *kv) Name() string {
	return p.name
}

func (p *kv) restore(ctx context.Context) error {
	p.l.Lock()
	defer p.l.Unlock()

	rc, err := p.b.Load(ctx, fileName(p.name))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return err
	}
	defer rc.Close()

	if err = json.NewDecoder(rc).Decode(&p.kv); err != nil {
		return err
	}

	return nil
}

// Set a key to the KeyValueStore with the given value.  If they key already
// exists, the value is overwritten.
func (p *kv) Put(ctx context.Context, key string, value []byte) error {
	p.l.Lock()
	defer p.l.Unlock()

	// get the old value in case we need to roll back
	oldvalue, ok := p.kv[key]

	if ok && bytes.Compare(oldvalue, value) == 0 {
		// NOOP
		return nil
	}

	p.kv[key] = value

	if err := p.save(ctx); err != nil && ok {
		// revert if failure
		p.kv[key] = oldvalue
		return err
	}

	return nil
}

// Get retrieves a key from the KeyValueStore.
func (p *kv) Get(key string) ([]byte, error) {
	p.l.RLock()
	defer p.l.RUnlock()

	v, ok := p.kv[key]
	if !ok {
		return []byte{}, ErrKeyNotFound
	}

	return v, nil
}

func (p *kv) List(re string) (map[string][]byte, error) {
	p.l.RLock()
	defer p.l.RUnlock()

	regex, err := regexp.Compile(re)
	if err != nil {
		return nil, err
	}

	kv := make(map[string][]byte)
	for k, v := range p.kv {
		if regex.MatchString(k) {
			kv[k] = v
		}
	}

	if len(kv) == 0 {
		return nil, ErrKeyNotFound
	}

	return kv, nil
}

// Delete removes a key from the KeyValueStore.
func (p *kv) Delete(ctx context.Context, key string) error {
	p.l.Lock()
	defer p.l.Unlock()

	oldvalue, ok := p.kv[key]
	if !ok {
		return ErrKeyNotFound
	}

	delete(p.kv, key)

	if err := p.save(ctx); err != nil {
		// restore the key
		p.kv[key] = oldvalue
		return err
	}

	return nil
}

// Save persists the KeyValueStore to the Backend.
func (p *kv) Save(ctx context.Context) error {
	p.l.Lock()
	defer p.l.Unlock()
	return p.save(ctx)
}

func (p *kv) save(ctx context.Context) error {
	buf, err := json.Marshal(p.kv)
	if err != nil {
		return err
	}

	r := bytes.NewReader(buf)
	if err = p.b.Save(ctx, r, fileName(p.name)); err != nil {
		return fmt.Errorf("Error uploading %s: %s", fileName(p.name), err)
	}

	return nil
}
