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

package opsuser

import (
	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/vic/pkg/vsphere/rbac"
)

var vchRolePrefix = "vic-vch-"

// Pre-existing ReadOnly Role, no need to specify the privileges
var RoleReadOnly = types.AuthorizationRole{
	Name:      "ReadOnly",
	Privilege: []string{},
}

var RoleVCenter = types.AuthorizationRole{
	Name: "vcenter",
	Privilege: []string{
		"Datastore.Config",
	},
}

var RoleDataCenter = types.AuthorizationRole{
	Name: "datacenter",
	Privilege: []string{
		"Datastore.Config",
		"Datastore.FileManagement",
		"VirtualMachine.Config.AddNewDisk",
		"VirtualMachine.Config.AdvancedConfig",
		"VirtualMachine.Config.RemoveDisk",
		"VirtualMachine.Inventory.Create",
		"VirtualMachine.Inventory.Delete",
	},
}

var RoleCluster = types.AuthorizationRole{
	Name: "cluster",
	Privilege: []string{
		"Datastore.AllocateSpace",
		"Datastore.Browse",
		"Datastore.Config",
		"Datastore.DeleteFile",
		"Datastore.FileManagement",
		"Host.Config.SystemManagement",
	},
}

var RoleDataStore = types.AuthorizationRole{
	Name: "datastore",
	Privilege: []string{
		"Datastore.AllocateSpace",
		"Datastore.Browse",
		"Datastore.Config",
		"Datastore.DeleteFile",
		"Datastore.FileManagement",
		"Host.Config.SystemManagement",
	},
}

var RoleNetwork = types.AuthorizationRole{
	Name: "network",
	Privilege: []string{
		"Network.Assign",
	},
}

var RoleEndpoint = types.AuthorizationRole{
	Name: "endpoint",
	Privilege: []string{
		"DVPortgroup.Modify",
		"DVPortgroup.PolicyOp",
		"DVPortgroup.ScopeOp",
		"Resource.AssignVMToPool",
		"VirtualMachine.Config.AddExistingDisk",
		"VirtualMachine.Config.AddNewDisk",
		"VirtualMachine.Config.AddRemoveDevice",
		"VirtualMachine.Config.AdvancedConfig",
		"VirtualMachine.Config.EditDevice",
		"VirtualMachine.Config.RemoveDisk",
		"VirtualMachine.Config.Rename",
		"VirtualMachine.GuestOperations.Execute",
		"VirtualMachine.Interact.DeviceConnection",
		"VirtualMachine.Interact.PowerOff",
		"VirtualMachine.Interact.PowerOn",
		"VirtualMachine.Inventory.Create",
		"VirtualMachine.Inventory.Delete",
		"VirtualMachine.Inventory.Register",
		"VirtualMachine.Inventory.Unregister",
	},
}

var DCReadOnlyConf = rbac.Config{
	Resources: []rbac.Resource{
		{
			Type:      rbac.DatacenterReadOnly,
			Propagate: false,
			Role:      RoleReadOnly,
		},
	},
}

// Configuration for the ops-user
var OpsuserRBACConf = rbac.Config{
	Resources: []rbac.Resource{
		{
			Type:      rbac.VCenter,
			Propagate: false,
			Role:      RoleVCenter,
		},
		{
			Type:      rbac.Datacenter,
			Propagate: true,
			Role:      RoleDataCenter,
		},
		{
			Type:      rbac.Cluster,
			Propagate: true,
			Role:      RoleDataStore,
		},
		{
			Type:      rbac.DatastoreFolder,
			Propagate: true,
			Role:      RoleDataStore,
		},
		{
			Type:      rbac.Datastore,
			Propagate: false,
			Role:      RoleDataStore,
		},
		{
			Type:      rbac.VSANDatastore,
			Propagate: false,
			Role:      RoleDataStore,
		},
		{
			Type:      rbac.Network,
			Propagate: true,
			Role:      RoleNetwork,
		},
		{
			Type:      rbac.Endpoint,
			Propagate: true,
			Role:      RoleEndpoint,
		},
	},
}
