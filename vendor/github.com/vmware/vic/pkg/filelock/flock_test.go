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

package filelock

import (
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFlock(t *testing.T) {
	lockName := "test_lock" + strconv.FormatInt(time.Now().UnixNano(), 10)
	fl1 := NewFileLock(lockName)
	fl2 := NewFileLock(lockName)
	st := time.Now()
	et := st
	var wg sync.WaitGroup

	wg.Add(1)
	fl1.Acquire()

	go func() {

		if err := fl2.Acquire(); err != nil {
			t.Fatal(err)
		}
		et = time.Now()

		if err := fl2.Release(); err != nil {
			t.Fatal(err)
		}
		wg.Done()
	}()

	time.Sleep(time.Millisecond * 100)
	fl1.Release()
	wg.Wait()

	delta := (et.UnixNano() - st.UnixNano()) / 1000000
	if delta < 50 {
		t.Errorf("Wait time is less than 50: %d", delta)
	}
}

func TestManyLocks(t *testing.T) {
	lockName := "test_lock" + strconv.FormatInt(time.Now().UnixNano(), 10)
	baseLock := NewFileLock(lockName)

	var wg sync.WaitGroup
	if err := baseLock.Acquire(); err != nil {
		t.Fatal(err)
	}

	locksCount := 200
	cnt := 0

	for i := 0; i < locksCount; i++ {
		wg.Add(1)
		go func() {
			time.Sleep(time.Millisecond)
			defer wg.Done()
			l := NewFileLock(lockName)
			if err := l.Acquire(); err != nil {
				t.Error(err)
			} else {
				cnt++
				l.Release()
			}
		}()
	}
	baseLock.Release()
	wg.Wait()
	assert.Equal(t, locksCount, cnt)
}

func TestManyLocksWithNoBaseLock(t *testing.T) {
	lockName := "test_lock" + strconv.FormatInt(time.Now().UnixNano(), 10)

	var wg sync.WaitGroup

	locksCount := 200
	cnt := 0
	for i := 0; i < locksCount; i++ {
		wg.Add(1)
		go func() {
			time.Sleep(time.Millisecond)
			defer wg.Done()
			l := NewFileLock(lockName)
			if err := l.Acquire(); err != nil {
				t.Error(err)
			} else {
				cnt++
				l.Release()
			}
		}()
	}
	wg.Wait()
	assert.Equal(t, locksCount, cnt)
}
