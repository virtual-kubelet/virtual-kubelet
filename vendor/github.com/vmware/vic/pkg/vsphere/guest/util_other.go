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

// +build !linux

package guest

import (
	"fmt"
	"runtime"

	"context"

	"github.com/vmware/vic/pkg/vsphere/session"
	"github.com/vmware/vic/pkg/vsphere/vm"
)

// UUID gets the BIOS UUID via the sys interface.  This UUID is known by vphsere
func UUID() (string, error) {
	return "", fmt.Errorf("unimplemented on %s", runtime.GOOS)
}

// GetSelf gets VirtualMachine reference for the VM this process is running on
func GetSelf(ctx context.Context, s *session.Session) (*vm.VirtualMachine, error) {
	return nil, fmt.Errorf("unimplemented on %s", runtime.GOOS)
}
