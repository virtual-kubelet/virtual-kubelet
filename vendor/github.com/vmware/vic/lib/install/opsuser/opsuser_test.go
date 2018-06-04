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
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/vmware/vic/lib/config"

	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/vic/pkg/vsphere/rbac"
	"github.com/vmware/vic/pkg/vsphere/session"
	"github.com/vmware/vic/pkg/vsphere/test/env"
)

var opsuser = "ops-user@vsphere.local"
var rolePrefix = "vic-vch-"

func TestOpsUserRolesSimulatorVPX(t *testing.T) {
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

	am := rbac.NewAuthzManager(ctx, sess.Vim25())
	am.InitConfig(opsuser, rolePrefix, &OpsuserRBACConf)

	var testRoleNames = []string{
		"datastore",
		"endpoint",
	}

	var testRolePrivileges = []string{
		"Datastore.DeleteFile",
		"VirtualMachine.Config.AddNewDisk",
	}

	rbac.DoTestRoles(ctx, t, am, testRoleNames, testRolePrivileges)
}

func TestOpsUserRolesVCenter(t *testing.T) {
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

	am := rbac.NewAuthzManager(ctx, sess.Vim25())
	am.InitConfig(opsuser, rolePrefix, &OpsuserRBACConf)

	var testRoleNames = []string{
		"datastore",
		"endpoint",
	}

	var testRolePrivileges = []string{
		"Datastore.DeleteFile",
		"VirtualMachine.Config.AddNewDisk",
	}

	rbac.DoTestRoles(ctx, t, am, testRoleNames, testRolePrivileges)
}

func TestOpsUserPermsSimulatorVPX(t *testing.T) {
	ctx := context.Background()
	m := simulator.VPX()

	defer m.Remove()

	err := m.Create()
	require.NoError(t, err)

	s := m.Service.NewServer()
	defer s.Close()

	fmt.Println(s.URL.String())

	sessionConfig := &session.Config{
		Service:   s.URL.String(),
		Insecure:  true,
		Keepalive: time.Duration(5) * time.Minute,
	}

	sess, err := session.NewSession(sessionConfig).Connect(ctx)
	require.NoError(t, err)

	configSpec := &config.VirtualContainerHostConfigSpec{
		Connection: config.Connection{
			Username: "ops-user@vsphere.local",
		},
	}

	mgr := NewRBACManager(ctx, sess.Vim25(), nil, &OpsuserRBACConf, configSpec)
	am := mgr.AuthzManager

	var roleCount = len(am.TargetRoles)
	count := rbac.InitRoles(ctx, t, am)

	defer rbac.Cleanup(ctx, t, am, true)
	require.Equal(t, roleCount, count, "Incorrect number of roles: expected %d, actual %d", roleCount, count)

	c := sess.Client
	// Find the Datacenter
	finder := find.NewFinder(c.Client, false)

	dcList, err := finder.DatacenterList(ctx, "/*")
	require.NoError(t, err)
	require.NotEqual(t, 0, len(dcList))

	dc := dcList[0]
	finder.SetDatacenter(dc)

	resourcePermission, err := am.AddPermission(ctx, dc.Reference(), rbac.Datacenter, false)
	require.NoError(t, err)
	require.NotNil(t, resourcePermission)

	// Get permission back
	permissions, err := am.GetPermissions(ctx, dc.Reference())

	if err != nil || len(permissions) == 0 {
		t.Fatalf("Failed to get permissions for Datacenter")
	}

	foundPermission := permissions[0]

	permission := &resourcePermission.Permission

	if foundPermission.Principal != permission.Principal ||
		foundPermission.RoleId != permission.RoleId ||
		foundPermission.Propagate != permission.Propagate ||
		foundPermission.Group != permission.Group {
		t.Fatalf("Permission mismatch, exp: %v, found: %v", permission, foundPermission)
	}
}
