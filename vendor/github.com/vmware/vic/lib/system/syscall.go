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

package system

import "syscall"

// Syscall provides an interface to make system calls
type Syscall interface {
	Mount(source string, target string, fstype string, flags uintptr, data string) error
	Sethostname(p []byte) error
	Symlink(oldname, newname string) error
	Unmount(path string, flags int) error
}

type syscallImpl struct {
}

func (s syscallImpl) Symlink(oldname, newname string) error {
	return syscall.Symlink(oldname, newname)
}
