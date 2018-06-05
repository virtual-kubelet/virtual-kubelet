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

package compute

import (
	"context"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/vmware/vic/pkg/vsphere/session"
)

// VirtualApp struct defines the VirtualApp which provides additional
// VIC specific methods over object.VirtualApp as well as keeps some state
type VirtualApp struct {
	*object.VirtualApp

	*session.Session
}

// NewResourcePool returns a New ResourcePool object
func NewVirtualApp(ctx context.Context, session *session.Session, moref types.ManagedObjectReference) *VirtualApp {
	return &VirtualApp{
		VirtualApp: object.NewVirtualApp(
			session.Vim25(),
			moref,
		),
		Session: session,
	}
}
