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

package filelock

import (
	"os"
	"path/filepath"
	"sync"
	"syscall"
)

// FileLock is a cross-process lock designed to work over FS that supports locking.
type FileLock struct {
	LockFile string
	LockName string

	mu sync.Mutex
	fh *os.File
}

// NewFileLock returns a new instance of the file based lock.
// it is a user responsibility to ensure lock name is unique and doesn't collide
// with any other file names in the TEMP directory.
func NewFileLock(lockName string) *FileLock {
	return &FileLock{
		LockName: lockName,
		LockFile: filepath.Join("/var/run/lock", lockName),
	}
}

// Acquire grabs the lock. If lock is already acquired, it will block.
// User should check for errors if lock is actually acquired, if lock is not acquired
// it will panic on Release.
func (fl *FileLock) Acquire() error {
	fl.mu.Lock()
	fh, err := os.Create(fl.LockFile)
	if err != nil {
		fl.mu.Unlock()
		return err
	}
	fl.fh = fh
	err = syscall.Flock(int(fh.Fd()), syscall.LOCK_EX)
	if err != nil {
		// #nosec: Errors unhandled
		fh.Close()
		fh = nil
		fl.mu.Unlock()
	}
	return err
}

// Release lock. If lock is not acquired, it will panic.
func (fl *FileLock) Release() error {
	if fl.fh == nil {
		panic("Attempt to release not acquired lock!")
	}
	// #nosec: Errors unhandled
	syscall.Flock(int(fl.fh.Fd()), syscall.LOCK_UN)
	err := fl.fh.Close()
	fl.fh = nil
	fl.mu.Unlock()
	return err
}
