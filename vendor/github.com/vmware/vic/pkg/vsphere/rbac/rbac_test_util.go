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
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/vmware/govmomi/vim25/types"
)

func DoTestRoles(ctx context.Context, t *testing.T, am *AuthzManager, testRoleNames []string, testRolePrivileges []string) {
	var roleCount = len(am.TargetRoles)
	count := InitRoles(ctx, t, am)

	defer Cleanup(ctx, t, am, true)
	require.Equal(t, roleCount, count, "Incorrect number of roles: expected %d, actual %d", roleCount, count)

	// Test correct role validation, it should return 0
	roleCount = 0
	count, err := am.createOrRepairRoles(ctx)
	require.NoError(t, err, "Failed to create roles")
	require.Equal(t, roleCount, count, "Incorrect number of roles: expected %d, actual %d", roleCount, count)

	// Remove two Privileges from two roles
	roles, err := am.getRoleList(ctx)
	fmt.Println(err)
	fmt.Println(roles)

	for i, name := range testRoleNames {
		testRoleNames[i] = am.RolePrefix + name
	}

	for _, role := range roles {
		if role.Name == testRoleNames[0] {
			removePrivilege(&role, testRolePrivileges[0])
			am.authzManager.UpdateRole(ctx, role.RoleId, role.Name, role.Privilege)
		}
		if role.Name == testRoleNames[1] {
			removePrivilege(&role, testRolePrivileges[1])
			am.authzManager.UpdateRole(ctx, role.RoleId, role.Name, role.Privilege)
		}
	}

	// Test
	roleCount = 2
	count, err = am.createOrRepairRoles(ctx)
	require.NoError(t, err, "Failed to repair roles 1")
	require.Equal(t, roleCount, count, "Incorrect number of roles: expected %d, actual %d", roleCount, count)

	// Test correct role validation, it should return 0
	roleCount = 0
	count, err = am.createOrRepairRoles(ctx)
	require.NoError(t, err, "Failed to repair roles 2")
	require.Equal(t, roleCount, count, "Incorrect number of roles: expected %d, actual %d", roleCount, count)
}

func VerifyResourcePermissions(ctx context.Context, t *testing.T, am *AuthzManager, retPerms []ResourcePermission) {
	for _, retPerm := range retPerms {

		// Validate returned permission against the configured permission
		configPerm := am.getResource(retPerm.RType)
		require.Equal(t, am.Principal, retPerm.Permission.Principal)
		require.Equal(t, configPerm.Propagate, retPerm.Permission.Propagate)

		actPerms, err := am.GetPermissions(ctx, retPerm.Reference)
		require.NoError(t, err)

		for _, actPerm := range actPerms {
			if actPerm.Principal != am.Principal {
				continue
			}
			// RoleId must be the same
			require.Equal(t, retPerm.Permission.RoleId, actPerm.RoleId)
		}
	}
}

func InitRoles(ctx context.Context, t *testing.T, am *AuthzManager) int {
	Cleanup(ctx, t, am, false)

	count, err := am.createOrRepairRoles(ctx)
	require.NoError(t, err, "Failed to initialize Roles")

	return count
}

func Cleanup(ctx context.Context, t *testing.T, am *AuthzManager, checkCount bool) {
	var roleCount = len(am.TargetRoles)
	count, err := am.deleteRoles(ctx)
	require.NoError(t, err, "Failed to delete roles")

	if checkCount && count != roleCount {
		t.Fatalf("Incorrect number of roles: expcted %d, actual %d", roleCount, count)
	}
}

func removePrivilege(role *types.AuthorizationRole, privilege string) {
	for i, priv := range role.Privilege {
		if priv == privilege {
			role.Privilege = append(role.Privilege[:i], role.Privilege[i+1:]...)
			return
		}
	}
}
