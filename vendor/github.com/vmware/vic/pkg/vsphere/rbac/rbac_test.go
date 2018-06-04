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

package rbac

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/vic/pkg/vsphere/session"
	"github.com/vmware/vic/pkg/vsphere/test/env"
)

var Role1 = types.AuthorizationRole{
	Name: "vcenter",
	Privilege: []string{
		"Datastore.Config",
	},
}

var Role2 = types.AuthorizationRole{
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

var Role3 = types.AuthorizationRole{
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

// Configuration for the ops-user
var testRBACConfig = Config{
	Resources: []Resource{
		{
			Type:      VCenter,
			Propagate: false,
			Role:      Role1,
		},
		{
			Type:      Datacenter,
			Propagate: true,
			Role:      Role2,
		},
		{
			Type:      Cluster,
			Propagate: true,
			Role:      Role3,
		},
	},
}

var testRolePrefix = "test-role-prefix"
var testUser = "test-user"

func TestRolesSimulatorVPX(t *testing.T) {
	ctx := context.Background()
	m := simulator.VPX()
	defer m.Remove()

	err := m.Create()
	require.NoError(t, err, "Cannot create VPX Simulator")

	s := m.Service.NewServer()
	defer s.Close()

	config := &session.Config{
		Service:   s.URL.String(),
		Insecure:  true,
		Keepalive: time.Duration(5) * time.Minute,
	}

	sess, err := session.NewSession(config).Connect(ctx)
	require.NoError(t, err, "Cannot connect to VPX Simulator")

	am := NewAuthzManager(ctx, sess.Vim25())
	am.InitConfig(testUser, testRolePrefix, &testRBACConfig)

	var testRoleNames = []string{
		"datacenter",
		"cluster",
	}

	var testRolePrivileges = []string{
		"VirtualMachine.Config.AddNewDisk",
		"Host.Config.SystemManagement",
	}

	DoTestRoles(ctx, t, am, testRoleNames, testRolePrivileges)
}

func TestRolesVCenter(t *testing.T) {
	ctx := context.Background()

	config := &session.Config{
		Service:   env.URL(t),
		Insecure:  true,
		Keepalive: time.Duration(5) * time.Minute,
	}

	sess, err := session.NewSession(config).Connect(ctx)
	if err != nil {
		t.SkipNow()
	}

	am := NewAuthzManager(ctx, sess.Vim25())
	am.InitConfig(testUser, testRolePrefix, &testRBACConfig)

	var testRoleNames = []string{
		"datacenter",
		"cluster",
	}

	var testRolePrivileges = []string{
		"VirtualMachine.Config.AddNewDisk",
		"Host.Config.SystemManagement",
	}

	DoTestRoles(ctx, t, am, testRoleNames, testRolePrivileges)
}

func TestAdminSimulatorVPX(t *testing.T) {
	ctx := context.Background()
	m := simulator.VPX()
	defer m.Remove()

	err := m.Create()
	require.NoError(t, err, "Cannot create VPX Simulator")

	s := m.Service.NewServer()
	defer s.Close()

	config := &session.Config{
		Service:   s.URL.String(),
		Insecure:  true,
		Keepalive: time.Duration(5) * time.Minute,
	}

	sess, err := session.NewSession(config).Connect(ctx)
	require.NoError(t, err, "Cannot connect to VPX Simulator")

	am := NewAuthzManager(ctx, sess.Vim25())
	am.InitConfig("admin", "test-role-prefix", &testRBACConfig)

	// Unfortunately the Sim does not have support for looking up group membership
	// therefore we can only test the presence of the Admin role

	res, err := am.PrincipalHasRole(ctx, "Admin")
	require.NoError(t, err, "Failed to verify Admin Privileges")
	require.True(t, res, "User Administrator@vsphere.local should have an Admin role")

	// Negative test, principal does not have that role
	res, err = am.PrincipalHasRole(ctx, "NoAccess")
	require.NoError(t, err, "Failed to verify Admin Privileges")
	require.False(t, res, "User Administrator@vsphere.local should have an NoAccess role")

	// Check regular user
	am.Principal = "nouser@vshpere.local"
	res, err = am.PrincipalHasRole(ctx, "Admin")
	require.NoError(t, err, "Failed to verify Admin Privileges")
	require.False(t, res, "User nouser@vsphere.local should not have an Admin role")
}

func TestAdminVCenter(t *testing.T) {
	ctx := context.Background()

	config := &session.Config{
		Service:   env.URL(t),
		Insecure:  true,
		Keepalive: time.Duration(5) * time.Minute,
	}

	sess, err := session.NewSession(config).Connect(ctx)
	if err != nil {
		t.SkipNow()
	}

	am := NewAuthzManager(ctx, sess.Vim25())
	am.InitConfig("Administrator@vsphere.local", "test-role-prefix", &testRBACConfig)

	res, err := am.PrincipalBelongsToGroup(ctx, "Administrators")
	require.NoError(t, err, "Failed to verify Admin Privileges")
	require.True(t, res, "User Administrator@vsphere.local should be a member of Administrators")

	res, err = am.PrincipalHasRole(ctx, "Admin")
	require.NoError(t, err, "Failed to verify Admin Privileges")
	require.True(t, res, "User Administrator@vsphere.local should have an Admin role")

	// Negative test, principal does not belong
	res, err = am.PrincipalBelongsToGroup(ctx, "TestUsers")
	require.NoError(t, err, "Failed to verify Admin Privileges")
	require.False(t, res, "User Administrator@vsphere.local should not be a member of TestUsers")

	// Negative test, principal does not have that role
	res, err = am.PrincipalHasRole(ctx, "NoAccess")
	require.NoError(t, err, "Failed to verify Admin Privileges")
	require.False(t, res, "User Administrator@vsphere.local should have an NoAccess role")

	// Check regular user
	am.Principal = "nouser@vshpere.local"
	res, err = am.PrincipalHasRole(ctx, "Admin")
	require.NoError(t, err, "Failed to verify Admin Privileges")
	require.False(t, res, "User nouser@vsphere.local should not have an Admin role")

	// Check regular user
	am.Principal = "nouser"
	res, err = am.PrincipalHasRole(ctx, "Admin")
	require.NoError(t, err, "Failed to verify Admin Privileges")
	require.False(t, res, "User nouser@vsphere.local should not have an Admin role")
}
