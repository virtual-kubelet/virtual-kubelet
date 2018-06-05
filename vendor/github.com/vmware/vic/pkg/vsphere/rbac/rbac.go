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
	"fmt"
	"reflect"
	"strings"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/methods"
	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/vic/pkg/errors"
	"github.com/vmware/vic/pkg/trace"
)

const (
	VCenter = iota
	DatacenterReadOnly
	Datacenter
	Cluster
	DatastoreFolder
	Datastore
	VSANDatastore
	Network
	Endpoint
)

const (
	sysAnonPriv = "System.Anonymous"
	sysReadPriv = "System.Read"
	sysViewPriv = "System.View"
)

type NameToRef map[string]types.ManagedObjectReference

type AuthzManager struct {
	authzManager *object.AuthorizationManager
	client       *vim25.Client
	resources    map[int8]*Resource
	TargetRoles  []types.AuthorizationRole
	RolePrefix   string
	Principal    string
	Config       *Config
}

type Resource struct {
	Type      int8
	Propagate bool
	Role      types.AuthorizationRole
}

type Config struct {
	Resources []Resource
}

type ResourcePermission struct {
	RType      int8
	Reference  types.ManagedObjectReference
	Permission types.Permission
}

func NewAuthzManager(ctx context.Context, client *vim25.Client) *AuthzManager {
	authManager := object.NewAuthorizationManager(client)
	mgr := &AuthzManager{
		client:       client,
		authzManager: authManager,
	}
	return mgr
}

func (am *AuthzManager) InitConfig(principal string, rolePrefix string, config *Config) {
	am.Principal = principal
	am.RolePrefix = rolePrefix
	am.Config = config
	am.initTargetRoles()
	am.initResourceMap()
}

func (am *AuthzManager) CreateRoles(ctx context.Context) (int, error) {
	return am.createOrRepairRoles(ctx)
}

func (am *AuthzManager) DeleteRoles(ctx context.Context) (int, error) {
	return am.deleteRoles(ctx)
}

func (am *AuthzManager) RoleList(ctx context.Context) (object.AuthorizationRoleList, error) {
	return am.getRoleList(ctx)
}

func (am *AuthzManager) IsPrincipalAnAdministrator(ctx context.Context) (bool, error) {
	// Check if the principal belongs to the Administrators group
	res, err := am.PrincipalBelongsToGroup(ctx, "Administrators")
	if err != nil {
		return false, err
	}

	if res {
		return res, nil
	}

	// Check if the principal has an Admin Role
	res, err = am.PrincipalHasRole(ctx, "Admin")
	if err != nil {
		return false, err
	}

	return res, nil
}

func (am *AuthzManager) PrincipalBelongsToGroup(ctx context.Context, group string) (bool, error) {
	op := trace.FromContext(ctx, "PrincipalBelongsToGroup")

	ref := *am.client.ServiceContent.UserDirectory

	components := strings.Split(am.Principal, "@")
	name := components[0]
	var domain string
	if len(components) >= 2 {
		domain = components[1]
	}

	req := types.RetrieveUserGroups{
		This:           ref,
		Domain:         domain,
		SearchStr:      name,
		ExactMatch:     true,
		BelongsToGroup: group,
		FindUsers:      true,
		FindGroups:     false,
	}

	results, err := methods.RetrieveUserGroups(ctx, am.client, &req)

	// This is to work around a bug in vSphere, when AD is added to
	// the identity source list, the API returns Object Not Found,
	// In this case, we ignore the error and return false (BUG: 2037706)
	if err != nil && (isNotSupportedError(ctx, err) || isNotFoundError(ctx, err)) {
		op.Debugf("Received Error (%s) from PrincipalBelongsToGroup(), could not verify user %s is not a member of the Administrators group", err.Error(), am.Principal)
		op.Warnf("If ops-user (%s) belongs to the Administrators group, permissions on some resources might have been restricted", am.Principal)
		return false, nil
	}

	if err != nil {
		op.Debugf("Error from PrincipalBelongsToGroup: %s", err.Error())
		return false, err
	}

	if len(results.Returnval) > 0 {
		return true, nil
	}

	return false, nil
}

// ReadPermsOnDC returns true if the user (principal) in the AuthzManager has at least
// read permissions on the input datacenter ref, false otherwise.
func (am *AuthzManager) ReadPermsOnDC(ctx context.Context, dcRef types.ManagedObjectReference) (bool, error) {
	perms, err := am.GetPermissions(ctx, dcRef)
	if err != nil {
		return false, err
	}

	roles, err := am.RoleList(ctx)
	if err != nil {
		return false, err
	}

	opsUser := strings.ToLower(am.Principal)

	// These vars keep track of whether the corresponding privileges have been found.
	var anon, read, view bool

	for _, perm := range perms {
		// Skip permission entries assigned to other users.
		user := am.formatPrincipal(perm.Principal)
		if user != opsUser {
			continue
		}

		role := roles.ById(perm.RoleId)
		if role == nil {
			return false, fmt.Errorf("unable to find role %d by ID", perm.RoleId)
		}

		// Check the role for privileges that satisfy the read-only role and set their flags.
		privs := role.Privilege
		for i := range privs {
			anon = anon || privs[i] == sysAnonPriv
			read = read || privs[i] == sysReadPriv
			view = view || privs[i] == sysViewPriv
		}

		if anon && read && view {
			// The ops-user has enough privileges for the read-only role.
			return true, nil
		}
	}

	return false, nil
}

func (am *AuthzManager) PrincipalHasRole(ctx context.Context, roleName string) (bool, error) {
	// Build expected representation of the ops-user
	principal := strings.ToLower(am.Principal)

	// Get role id for admin Role
	roleList, err := am.RoleList(ctx)
	if err != nil {
		return false, err
	}

	role := roleList.ByName(roleName)

	allPerms, err := am.authzManager.RetrieveAllPermissions(ctx)
	if err != nil {
		return false, err
	}

	for _, perm := range allPerms {
		if perm.RoleId != role.RoleId {
			continue
		}

		fPrincipal := am.formatPrincipal(perm.Principal)
		if fPrincipal == principal {
			return true, nil
		}
	}

	return false, nil
}

func (am *AuthzManager) GetPermissions(ctx context.Context,
	ref types.ManagedObjectReference) ([]types.Permission, error) {
	// Get current Permissions
	return am.authzManager.RetrieveEntityPermissions(ctx, ref, false)
}

func (am *AuthzManager) AddPermission(ctx context.Context, ref types.ManagedObjectReference, resourceType int8, isGroup bool) (*ResourcePermission, error) {

	resource := am.getResource(resourceType)
	if resource == nil {
		return nil, fmt.Errorf("cannot find resource of type %d", resourceType)
	}

	// Collect the new roles, possibly cache the result in the Authz manager
	roleList, err := am.getRoleList(ctx)
	if err != nil {
		return nil, err
	}

	// Locate target role
	role := roleList.ByName(am.getRoleName(resource))
	if role == nil {
		return nil, fmt.Errorf("cannot find role: %s", resource.Role.Name)
	}

	// Get current permissions
	permissions, err := am.authzManager.RetrieveEntityPermissions(ctx, ref, false)
	if err != nil {
		return nil, err
	}

	for _, permission := range permissions {
		if permission.Principal == am.Principal &&
			permission.RoleId == role.RoleId &&
			permission.Propagate == resource.Propagate {
			return nil, nil
		}
	}

	// No match found, create new permission
	permission := types.Permission{
		Principal: am.Principal,
		RoleId:    role.RoleId,
		Propagate: resource.Propagate,
		Group:     isGroup,
	}
	permissions = append(permissions, permission)

	if err = am.authzManager.SetEntityPermissions(ctx, ref, permissions); err != nil {
		return nil, err
	}

	resourcePermission := &ResourcePermission{
		RType:      resourceType,
		Reference:  ref,
		Permission: permission,
	}

	return resourcePermission, nil
}

func (am *AuthzManager) createOrRepairRoles(ctx context.Context) (int, error) {
	// Get all the existing roles
	mgr := am.authzManager
	roleList, err := mgr.RoleList(ctx)
	if err != nil {
		return 0, err
	}

	var count int
	for _, targetRole := range am.TargetRoles {
		foundRole := roleList.ByName(targetRole.Name)
		if foundRole != nil {
			isMod, err := am.checkAndRepairRole(ctx, &targetRole, foundRole)
			if isMod && err == nil {
				count++
			}
		} else {
			_, err = mgr.AddRole(ctx, targetRole.Name, targetRole.Privilege)
			if err == nil {
				count++
			}
		}
		if err != nil {
			return count, err
		}
	}
	return count, nil
}

func (am *AuthzManager) deleteRoles(ctx context.Context) (int, error) {
	mgr := am.authzManager
	// Get all the existing roles
	roleList, err := mgr.RoleList(ctx)
	if err != nil {
		return 0, err
	}

	var count int
	for _, targetRole := range am.TargetRoles {
		foundRole := roleList.ByName(targetRole.Name)
		if foundRole != nil {
			err = mgr.RemoveRole(ctx, foundRole.RoleId, true)
			if err == nil {
				count++
			}
		}
	}
	return count, nil
}

func (am *AuthzManager) getRoleList(ctx context.Context) (object.AuthorizationRoleList, error) {
	return am.authzManager.RoleList(ctx)
}

func (am *AuthzManager) checkAndRepairRole(ctx context.Context, tRole *types.AuthorizationRole, fRole *types.AuthorizationRole) (bool, error) {
	mgr := am.authzManager
	// Check that the privileges list in Target Role is a subset of the list in Found role
	fSet := make(map[string]bool)
	for _, p := range fRole.Privilege {
		fSet[p] = true
	}

	var isModified bool
	for _, p := range tRole.Privilege {
		if _, found := fSet[p]; !found {
			// Privilege not found
			// Add it to the found Role
			fRole.Privilege = append(fRole.Privilege, p)
			isModified = true
		}
	}

	if !isModified {
		return false, nil
	}

	// Not a subset, need to call govmomi to set the new privileges
	err := mgr.UpdateRole(ctx, fRole.RoleId, fRole.Name, fRole.Privilege)

	return true, err
}

func (am *AuthzManager) initTargetRoles() {
	count := len(am.Config.Resources)
	roles := make([]types.AuthorizationRole, 0, count)
	dSet := make(map[string]bool)
	for index, resource := range am.Config.Resources {
		name := am.getRoleName(&am.Config.Resources[index])
		// Discard duplicates
		if _, found := dSet[name]; !found {
			role := new(types.AuthorizationRole)
			*role = resource.Role
			role.Name = name
			dSet[name] = true
			roles = append(roles, *role)
		}
	}
	am.TargetRoles = roles
}

func (am *AuthzManager) initResourceMap() {
	am.resources = make(map[int8]*Resource)
	for i, resource := range am.Config.Resources {
		am.resources[resource.Type] = &am.Config.Resources[i]
	}
}

func (am *AuthzManager) getResource(resourceType int8) *Resource {
	resource, ok := am.resources[resourceType]
	if !ok {
		panic(errors.Errorf("Cannot find RBAC resource type: %d", resourceType))
	}
	return resource
}

func (am *AuthzManager) formatPrincipal(principal string) string {
	components := strings.Split(principal, "\\")
	if len(components) != 2 {
		return strings.ToLower(principal)
	}
	ret := strings.ToLower(components[1]) + "@" + strings.ToLower(components[0])
	return ret
}

func (am *AuthzManager) getRoleName(resource *Resource) string {
	switch resource.Type {
	case DatacenterReadOnly:
		return resource.Role.Name
	default:
		return am.RolePrefix + resource.Role.Name
	}
}

func isNotSupportedError(ctx context.Context, err error) bool {
	op := trace.FromContext(ctx, "isNotSupportedError")

	if soap.IsSoapFault(err) {
		vimFault := soap.ToSoapFault(err).VimFault()
		op.Debugf("Error type: %s", reflect.TypeOf(vimFault))

		_, ok := soap.ToSoapFault(err).VimFault().(types.NotSupported)
		return ok
	}

	return false
}

func isNotFoundError(ctx context.Context, err error) bool {
	op := trace.FromContext(ctx, "isNotFoundError")

	if soap.IsSoapFault(err) {
		vimFault := soap.ToSoapFault(err).VimFault()
		op.Debugf("Error type: %s", reflect.TypeOf(vimFault))

		_, ok := soap.ToSoapFault(err).VimFault().(types.NotFound)
		return ok
	}

	return false
}
