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

package guest

import (
	"fmt"
	"io/ioutil"
	"strings"

	"context"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/vic/pkg/vsphere/session"
	"github.com/vmware/vic/pkg/vsphere/vm"
)

const (
	UUIDPath   = "/sys/class/dmi/id/product_serial"
	UUIDPrefix = "VMware-"
)

// UUID gets the BIOS UUID via the sys interface.  This UUID is known by vphsere
func UUID() (string, error) {
	id, err := ioutil.ReadFile(UUIDPath)
	if err != nil {
		return "", fmt.Errorf("error retrieving vm uuid: %s", err)
	}

	uuidstr := string(id[:])

	// check the uuid starts with "VMware-"
	if !strings.HasPrefix(uuidstr, UUIDPrefix) {
		return "", fmt.Errorf("cannot find this VM's UUID")
	}

	// Strip the prefix, white spaces, and the trailing '\n'
	uuidstr = strings.Replace(uuidstr[len(UUIDPrefix):(len(uuidstr)-1)], " ", "", -1)

	// need to add dashes, e.g. "564d395e-d807-e18a-cb25-b79f65eb2b9f"
	uuidstr = fmt.Sprintf("%s-%s-%s-%s", uuidstr[0:8], uuidstr[8:12], uuidstr[12:21], uuidstr[21:])

	return uuidstr, nil
}

// GetSelf gets VirtualMachine reference for the VM this process is running on
func GetSelf(ctx context.Context, s *session.Session) (*vm.VirtualMachine, error) {
	u, err := UUID()
	if err != nil {
		return nil, err
	}

	search := object.NewSearchIndex(s.Vim25())
	ref, err := search.FindByUuid(ctx, s.Datacenter, u, true, nil)
	if err != nil {
		return nil, err
	}

	if ref == nil {
		return nil, fmt.Errorf("can't find the hosting vm")
	}

	return vm.NewVirtualMachine(ctx, s, ref.Reference()), nil
}
