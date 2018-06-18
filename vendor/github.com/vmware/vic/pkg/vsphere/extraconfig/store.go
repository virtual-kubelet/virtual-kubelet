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

package extraconfig

import (
	"sync"
)

// Store provides combined DataSource and DataSink.
type Store interface {
	Get(string) (string, error)
	Put(string, string) error
}

type MapStore struct {
	mutex sync.Mutex

	store map[string]string
}

func New() *MapStore {
	return &MapStore{
		store: make(map[string]string),
	}
}

func (t *MapStore) Get(key string) (string, error) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	val, ok := t.store[key]
	if !ok {
		return "", ErrKeyNotFound
	}
	return val, nil
}

func (t *MapStore) Put(key, value string) error {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	t.store[key] = value
	return nil
}
