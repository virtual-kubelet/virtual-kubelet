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

package communication

import (
	"sync"
)

// LazyInitializer defines the function that returns SessionInteractor
type LazyInitializer func() (SessionInteractor, error)

// LazySessionInteractor holds lazily initialized SessionInteractor
type LazySessionInteractor struct {
	mu sync.Mutex
	si SessionInteractor
	fn LazyInitializer
}

// Initialize either returns either already initialized connection or returns the connection after initializing it
func (l *LazySessionInteractor) Initialize() (SessionInteractor, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.si != nil {
		return l.si, nil
	}

	if l.si == nil && l.fn == nil {
		panic("both si and fn are nil")
	}

	var err error

	// l.si is nil but l.fn is not
	l.si, err = l.fn()
	if err != nil {
		return nil, err
	}
	return l.si, nil
}

// SessionInteractor returns either an initialized connection, or nil if it was never initialized
func (l *LazySessionInteractor) SessionInteractor() SessionInteractor {
	l.mu.Lock()
	defer l.mu.Unlock()

	return l.si
}
