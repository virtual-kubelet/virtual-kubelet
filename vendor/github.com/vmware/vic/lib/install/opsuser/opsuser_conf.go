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
		"Global.EnableMethods",
		"Global.DisableMethods",
	},
}

var RoleDataCenter = types.AuthorizationRole{
	Name: "datacenter",
	Privilege: []string{
		"Datastore.Config",
		"Datastore.FileManagement",
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
		"Host.Inventory.EditCluster",
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
		"Resource.ColdMigrate",
		"VirtualMachine.Config.AddExistingDisk",
		"VirtualMachine.Config.AddNewDisk",
		"VirtualMachine.Config.AddRemoveDevice",
		"VirtualMachine.Config.AdvancedConfig",
		"VirtualMachine.Config.EditDevice",
		"VirtualMachine.Config.RemoveDisk",
		"VirtualMachine.Config.Rename",
		"VirtualMachine.GuestOperations.Execute",
		"VirtualMachine.GuestOperations.Modify",
		"VirtualMachine.GuestOperations.Query",
		"VirtualMachine.Interact.DeviceConnection",
		"VirtualMachine.Interact.PowerOff",
		"VirtualMachine.Interact.PowerOn",
		"VirtualMachine.Inventory.Create",
		"VirtualMachine.Inventory.Delete",
		"VirtualMachine.Inventory.Register",
		"VirtualMachine.Inventory.Unregister",
	},
}

// RoleEndpointDatastore combines the privileges of RoleDataStore and RoleEndpoint
// and is applied to the cluster in a non-DRS environment.
var RoleEndpointDatastore = types.AuthorizationRole{
	Name:      "endpoint-datastore",
	Privilege: append(RoleDataStore.Privilege, RoleEndpoint.Privilege...),
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

func buildConfig(clusterRole types.AuthorizationRole) rbac.Config {
	return rbac.Config{
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
				Role:      clusterRole,
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
}

// DRSConf stores the RBAC configuration for the ops-user's roles in a DRS environment.
var DRSConf = buildConfig(RoleDataStore)

// NoDRSConf stores the configuration for the ops-user's roles in a non-DRS environment.
// It is different from DRSConf in that RoleEndpointDatastore is used for the cluster
// instead of RoleDataStore. In a non-DRS environment, we need to apply the Endpoint and
// Datastore roles at the cluster level since there are no resource pools.
var NoDRSConf = buildConfig(RoleEndpointDatastore)

// Configuration for the ops-user with increased cluster-level permissions, required for managing DRS VM Groups
var ClusterConf = buildConfig(RoleCluster)
