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

package rbac

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/vic/pkg/vsphere/session"
	"github.com/vmware/vic/pkg/vsphere/test"
	"github.com/vmware/vic/pkg/vsphere/test/env"
)

var role1 = types.AuthorizationRole{
	Name: "vcenter",
	Privilege: []string{
		"Datastore.Config",
	},
}

var role2 = types.AuthorizationRole{
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

var role3 = types.AuthorizationRole{
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

var readOnlyRole = types.AuthorizationRole{
	Name: "readonly",
	Privilege: []string{
		sysAnonPriv,
		sysReadPriv,
		sysViewPriv,
	},
}

var dcReadOnlyConfig = Config{
	Resources: []Resource{
		{
			Type:      Datacenter,
			Propagate: true,
			Role:      readOnlyRole,
		},
	},
}

// Configuration for the ops-user
var testRBACConfig = Config{
	Resources: []Resource{
		{
			Type:      VCenter,
			Propagate: false,
			Role:      role1,
		},
		{
			Type:      Datacenter,
			Propagate: true,
			Role:      role2,
		},
		{
			Type:      Cluster,
			Propagate: true,
			Role:      role3,
		},
	},
}

var (
	testRolePrefix = "test-role-prefix"
	testUser       = "test-user"
)

func TestReadPermsOnDC(t *testing.T) {
	ctx := context.Background()

	// Create the VPX model and server.
	m := simulator.VPX()
	defer m.Remove()
	err := m.Create()
	require.NoError(t, err, "Cannot create VPX simulator")

	server := m.Service.NewServer()
	defer server.Close()

	s, err := test.SessionWithVPX(ctx, server.URL.String())
	require.NoError(t, err, "Cannot initialize the VPX session")

	// Initialize the AuthzManager.
	am := NewAuthzManager(ctx, s.Vim25())
	am.InitConfig(testUser, testRolePrefix, &dcReadOnlyConfig)

	_, err = am.createOrRepairRoles(ctx)
	require.NoError(t, err, "Cannot create the read-only role")

	// Test that ReadPermsOnDC returns an error when a non-existent entity ref is supplied.
	// TODO(anchal): govmomi simulator's RetrieveEntityPermissions func assumes a validated
	// moref. This test case requires an update to govmomi.
	// fakeRef := types.ManagedObjectReference{
	// 	Type:  "VirtualMachine",
	// 	Value: "foo",
	// }
	// hasPrivs, err := am.ReadPermsOnDC(ctx, fakeRef)
	// require.Error(t, err, "Received no error from ReadPermsOnDC")

	// Test that ReadPermsOnDC returns false when no permissions have been set on an object.
	dcRef := s.Datacenter.Reference()
	hasPrivs, err := am.ReadPermsOnDC(ctx, dcRef)
	require.NoError(t, err, "Received unexpected error from ReadPermsOnDC")
	require.False(t, hasPrivs, "Expected ReadPermsOnDC to return false")

	// Test that ReadPermsOnDC returns false when a subset of read-only privileges is
	// assigned to an entity.
	clusterRef := s.Cluster.Reference()
	clusterPerms := []types.Permission{
		{
			Principal: am.Principal,
			// RoleId -3 is for the View role, which has only System.Anonymous
			// and System.View privileges.
			RoleId: int32(-3),
		},
	}
	err = am.authzManager.SetEntityPermissions(ctx, clusterRef, clusterPerms)
	require.NoError(t, err, "Cannot set permissions on cluster ref")

	hasPrivs, err = am.ReadPermsOnDC(ctx, clusterRef)
	require.NoError(t, err, "Received unexpected error from ReadPermsOnDC")
	require.False(t, hasPrivs, "Expected ReadPermsOnDC to return false")

	// Test that ReadPermsOnDC returns false when the permissions are assigned to a
	// user who does not match the ops-user (am.Principal).
	fakePrincipal := "foo@vsphere.local"
	dcPerms := []types.Permission{
		{
			Principal: fakePrincipal,
			RoleId:    readOnlyRole.RoleId,
		},
	}
	err = am.authzManager.SetEntityPermissions(ctx, dcRef, dcPerms)
	require.NoError(t, err, "Cannot set permissions on dc ref")

	hasPrivs, err = am.ReadPermsOnDC(ctx, dcRef)
	require.NoError(t, err, "Received unexpected error from ReadPermsOnDC")
	require.False(t, hasPrivs, "Expected ReadPermsOnDC to return false")

	// Test that ReadPermsOnDC returns true when read-only permissions are assigned to
	// the ops-user.
	dcPerms = []types.Permission{
		{
			Principal: am.Principal,
			RoleId:    readOnlyRole.RoleId,
		},
	}
	err = am.authzManager.SetEntityPermissions(ctx, dcRef, dcPerms)
	require.NoError(t, err, "Cannot set permissions on dc ref")

	hasPrivs, err = am.ReadPermsOnDC(ctx, dcRef)
	require.NoError(t, err, "Received unexpected error from ReadPermsOnDC")
	require.True(t, hasPrivs, "Expected ReadPermsOnDC to return true")
}

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
