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

package constants

import (
	"fmt"
	"time"

	"github.com/vmware/vic/pkg/version"
)

/* VCH constants */
const (
	SerialOverLANPort  = 2377
	VchAdminPortalPort = 2378
	AttachServerPort   = 2379
	ManagementHostName = "management.localhost"
	ClientHostName     = "client.localhost"

	// DebugPortLayerPort defines the portlayer port while debug level is greater than 2
	DebugPortLayerPort = 2380

	// BridgeScopeType denotes a scope that is of type bridge
	BridgeScopeType = "bridge"
	// ExternalScopeType denotes a scope that is of type external
	ExternalScopeType = "external"
	// DefaultBridgeRange is the default pool for bridge networks
	DefaultBridgeRange = "172.16.0.0/12"
	// PortsOpenNetwork indicates no port blocking
	PortsOpenNetwork = "0-65535"
	// Constants for assemble the VM display name on vSphere
	MaxVMNameLength = 80
	ShortIDLen      = 12
	// vSphere Display name for the VCH's Guest Name and for VAC support
	defaultAltVCHGuestName       = "Photon - VCH"
	defaultAltContainerGuestName = "Photon - Container"

	PropertyCollectorTimeout = 3 * time.Minute

	// Temporary names until they're altered to actual URLs.
	ContainerStoreName = "container"
	VolumeStoreName    = "volume"

	// volume mode flag
	Mode = "Mode"

	// PCI Slot Number logic
	PCISlotNumberBegin int32 = 0x4A0
	PCISlotNumberEnd   int32 = 1 << 11
	PCISlotNumberInc   int32 = 1 << 5

	// NilSlot is an invalid PCI slot number
	NilSlot int32 = 0

	// All paths on the datastore for images are relative to <datastore>/VIC/
	StorageParentDir = "VIC"

	// Key-value storage directory.
	KVStoreFolder = "kvStores"

	// All volumes are stored in this directory.
	VolumesDir = "volumes"

	// default log directory
	DefaultLogDir = "/var/log/vic"

	// Scratch layer ID
	ScratchLayerID = "scratch"

	// Task States
	TaskRunningState = "running"
	TaskStoppedState = "stopped"
	TaskCreatedState = "created"
	TaskFailedState  = "failed"
	TaskUnknownState = "unknown"
)

func DefaultAltVCHGuestName() string {
	return fmt.Sprintf("%s %s, %s, %7s", defaultAltVCHGuestName, version.Version, version.BuildNumber, version.GitCommit)
}

func DefaultAltContainerGuestName() string {
	return fmt.Sprintf("%s %s, %s, %7s", defaultAltContainerGuestName, version.Version, version.BuildNumber, version.GitCommit)
}
