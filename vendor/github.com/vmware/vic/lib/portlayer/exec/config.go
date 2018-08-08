// Copyright 2016-2018 VMware, Inc. All Rights Reserved.
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

package exec

import (
	"net/url"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/vic/lib/config"
	"github.com/vmware/vic/lib/config/executor"
	"github.com/vmware/vic/lib/portlayer/event"
	"github.com/vmware/vic/pkg/trace"
)

var Config Configuration

// Configuration is a slice of the VCH config that is relevant to the exec part of the port layer
type Configuration struct {
	// Turn on debug logging
	DebugLevel int `vic:"0.1" scope:"read-only" key:"init/diagnostics/debug"`

	SysLogConfig *executor.SysLogConfig `vic:"0.1" scope:"read-only" key:"init/diagnostics/syslog"`

	// Port Layer - exec
	config.Container `vic:"0.1" scope:"read-only" key:"container"`

	// Resource pool is the working version of the compute resource config
	ResourcePool *object.ResourcePool

	// Cluster is the working reference to the cluster the VCH is present in
	Cluster *object.ComputeResource

	// SelfReference is a reference to the endpointVM, added for VM group membership
	SelfReference types.ManagedObjectReference

	// Parent resource will be a VirtualApp on VC
	VirtualApp *object.VirtualApp

	// For now throw the Event Manager here
	EventManager event.EventManager

	// Information about the VCH resource pool and about the real host that we want
	// tol retrieve just once.
	VCHMhz          int64
	VCHMemoryLimit  int64
	HostOS          string
	HostOSVersion   string
	HostProductName string //'VMware vCenter Server' or 'VMare ESXi'

	// Datastore URLs for image stores - the top layer is [0], the bottom layer is [len-1]
	ImageStores []url.URL `vic:"0.1" scope:"read-only" key:"storage/image_stores"`

	// addToVMGroup sends signal for batching dispatcher to add container VM to VMGroup
	addToVMGroup func(trace.Operation) error
}
