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
	"context"
	"net/url"

	log "github.com/Sirupsen/logrus"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	gvsession "github.com/vmware/govmomi/session"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/vic/lib/config"
	"github.com/vmware/vic/pkg/errors"
	"github.com/vmware/vic/pkg/vsphere/compute"
	"github.com/vmware/vic/pkg/vsphere/rbac"
	"github.com/vmware/vic/pkg/vsphere/session"
)

var opsuserRolePrefix = "vic-vch-"

type RBACManager struct {
	AuthzManager *rbac.AuthzManager
	configSpec   *config.VirtualContainerHostConfigSpec
	session      *session.Session
	client       *vim25.Client
}

// resourceDesc encapsulates a resource's type and reference.
type resourceDesc struct {
	rType int8
	ref   types.ManagedObjectReference
}

func NewRBACManager(ctx context.Context, session *session.Session, rbacConfig *rbac.Config, configSpec *config.VirtualContainerHostConfigSpec) (*RBACManager, error) {
	if session == nil {
		return nil, errors.New("unable to create an RBACManager - session not set")
	}

	rbacMgr := &RBACManager{
		configSpec: configSpec,
		session:    session,
		client:     session.Vim25(),
	}

	am := rbac.NewAuthzManager(ctx, rbacMgr.client)
	am.InitConfig(configSpec.Connection.Username, opsuserRolePrefix, rbacConfig)
	rbacMgr.AuthzManager = am

	return rbacMgr, nil
}

func GrantDCReadOnlyPerms(ctx context.Context, session *session.Session, configSpec *config.VirtualContainerHostConfigSpec) error {
	mgr, err := NewRBACManager(ctx, session, &DCReadOnlyConf, configSpec)
	if err != nil {
		return err
	}

	_, err = mgr.SetupDCReadOnlyPermissions(ctx)
	return err
}

func GrantOpsUserPerms(ctx context.Context, session *session.Session, configSpec *config.VirtualContainerHostConfigSpec) error {
	var rbacConfig *rbac.Config

	// Use a separate RBAC configuration depending on whether DRS is enabled.
	if session.DRSEnabled == nil || !*session.DRSEnabled {
		rbacConfig = &NoDRSConf
	} else {
		if configSpec.UseVMGroup {
			rbacConfig = &ClusterConf
		} else {
			rbacConfig = &DRSConf
		}
	}

	mgr, err := NewRBACManager(ctx, session, rbacConfig, configSpec)
	if err != nil {
		return err
	}

	_, err = mgr.SetupRolesAndPermissions(ctx)
	return err
}

func (mgr *RBACManager) SetupRolesAndPermissions(ctx context.Context) ([]rbac.ResourcePermission, error) {
	am := mgr.AuthzManager
	res, err := am.IsPrincipalAnAdministrator(ctx)
	if err != nil {
		return nil, err
	}
	if res {
		log.Warnf("Skipping ops-user Role/Permissions initialization. The current ops-user (%s) has administrative privileges.", am.Principal)
		log.Warnf("This occurs when \"%s\" is a member of the \"Administrators\" group or has been granted \"Admin\" role to any of the resources in the system.", am.Principal)
		return nil, nil
	}
	if _, err = am.CreateRoles(ctx); err != nil {
		return nil, err
	}
	return mgr.SetupPermissions(ctx)
}

func (mgr *RBACManager) SetupPermissions(ctx context.Context) ([]rbac.ResourcePermission, error) {
	return mgr.setupPermissions(ctx)
}

func (mgr *RBACManager) SetupDCReadOnlyPermissions(ctx context.Context) (*rbac.ResourcePermission, error) {
	am := mgr.AuthzManager
	res, err := am.IsPrincipalAnAdministrator(ctx)
	if err != nil {
		return nil, err
	}

	// If administrator, skip setting the root permissions
	if res {
		log.Warnf("Cannot perform ops-user Role/Permissions initialization. The current ops-user (%s) has administrative privileges.", am.Principal)
		log.Warnf("This occurs when \"%s\" is a member of the \"Administrators\" group or has been granted \"Admin\" role to any of the resources in the system.", am.Principal)
		return nil, errors.Errorf("Cannot grant ops-user permissions as %s has administrative privileges", am.Principal)
	}

	dcRef := mgr.session.Datacenter.Reference()
	hasPrivs, err := am.ReadPermsOnDC(ctx, dcRef)
	if err != nil {
		return nil, err
	}

	// If the ops-user already has enough privileges on the datacenter to at least
	// satisfy the read-only role, skip setting read-only permissions.
	if hasPrivs {
		log.Debugf("ops-user already has enough privileges for read-only access to datacenter, skipping setting read-only permissions")
		return nil, nil
	}

	return mgr.setupDcReadOnlyPermissions(ctx)
}

func (mgr *RBACManager) setupDcReadOnlyPermissions(ctx context.Context) (*rbac.ResourcePermission, error) {
	session := mgr.session
	am := mgr.AuthzManager
	dcRef := session.Datacenter.Reference()

	// Apply permissions
	resourcePermission, err := am.AddPermission(ctx, dcRef, rbac.DatacenterReadOnly, false)
	if err != nil {
		return nil, errors.Errorf("Ops-User: RBACManager, Unable to set top read only permissions on %s, error: %s",
			dcRef.String(), err.Error())
	}

	return resourcePermission, nil
}

func (mgr *RBACManager) setupPermissions(ctx context.Context) ([]rbac.ResourcePermission, error) {
	am := mgr.AuthzManager
	resourceDescs := make([]resourceDesc, 0, len(am.Config.Resources))

	// Get a reference to the top object
	finder := find.NewFinder(mgr.client, false)

	root, err := finder.Folder(ctx, "/")
	if err != nil {
		return nil, errors.Errorf("Ops-User: RBACManager, Unable to find top object: %s", err.Error())
	}

	resourceDescs = append(resourceDescs, resourceDesc{rbac.VCenter, root.Reference()})

	session := session.NewSession(&session.Config{})
	// Set client
	session.Client = &govmomi.Client{
		Client:         mgr.client,
		SessionManager: gvsession.NewManager(mgr.client),
	}

	// Use the VirtualContainerHostConfigSpec to find the various resources
	// Start with Resource Pool, Cluster and Datacenter
	rpRef := mgr.configSpec.ComputeResources[0]
	rp := compute.NewResourcePool(ctx, session, rpRef)

	datacenter, err := rp.GetDatacenter(ctx)
	if err != nil {
		return nil, errors.Errorf("Ops-User: RBACManager, Unable to find Datacenter: %s", err.Error())
	}
	resourceDescs = append(resourceDescs, resourceDesc{rbac.Datacenter, datacenter.Reference()})

	finder.SetDatacenter(datacenter)

	cluster, err := rp.GetCluster(ctx)
	if err != nil {
		return nil, errors.Errorf("Ops-User: RBACManager, Unable to find Cluster: %s", err.Error())
	}

	// In a DRS environment, this will add RoleDataStore to the cluster. In a non-DRS environment,
	// this will add RoleEndpointDatastore to the cluster per the no-DRS RBAC configuration.
	resourceDescs = append(resourceDescs, resourceDesc{rbac.Cluster, cluster.Reference()})

	// Find image and volume datastores
	dsNameToRef := make(rbac.NameToRef)
	err = mgr.collectDatastores(ctx, finder, dsNameToRef)
	if err != nil {
		return nil, errors.Errorf("Ops-User: RBACManager, Unable to find Datastores: %s", err.Error())
	}

	// Add datastores
	for _, ref := range dsNameToRef {
		resourceDescs = append(resourceDescs, resourceDesc{rbac.Datastore, ref})
	}

	// Add networks
	for _, network := range mgr.configSpec.Network.ContainerNetworks {
		netRef := &types.ManagedObjectReference{}
		netRef.FromString(network.ID)
		if netRef.Type == "" || netRef.Value == "" {
			return nil, errors.Errorf("Ops-User: RBACManager, Unable to build Bridged Network MoRef: %s", network.ID)
		}
		resourceDescs = append(resourceDescs, resourceDesc{rbac.Network, *netRef})
	}

	// In a DRS environment, add the endpoint role to the resource pool(s). In a
	// non-DRS environment, this role's privileges are applied at the cluster
	// level by using RoleEndpointDatastore.
	if mgr.session.DRSEnabled != nil && *mgr.session.DRSEnabled {
		for _, rPoolRef := range mgr.configSpec.ComputeResources {
			resourceDescs = append(resourceDescs, resourceDesc{rbac.Endpoint, rPoolRef})
		}
	}

	// For vCenter, apply the endpoint role to the VCH inventory folder as well.
	if mgr.session.IsVC() {
		vchFolder := mgr.session.VCHFolder
		if vchFolder == nil {
			return nil, errors.Errorf("VCH folder not set, unable to set permissions")
		}

		resourceDescs = append(resourceDescs, resourceDesc{rbac.Endpoint, vchFolder.Reference()})
	}

	resourcePermissions := make([]rbac.ResourcePermission, 0, len(am.Config.Resources))
	// Apply permissions
	for _, desc := range resourceDescs {
		resourcePermission, err := am.AddPermission(ctx, desc.ref, desc.rType, false)
		if err != nil {
			return nil, errors.Errorf("Ops-User: RBACManager, Unable to set permissions on %s, error: %s",
				desc.ref.String(), err.Error())
		}
		if resourcePermission != nil {
			resourcePermissions = append(resourcePermissions, *resourcePermission)
		}
	}

	return resourcePermissions, nil
}

func (mgr *RBACManager) collectDatastores(ctx context.Context, finder *find.Finder, dsNameToRef rbac.NameToRef) error {
	err := mgr.findDatastores(ctx, finder, mgr.configSpec.Storage.ImageStores, dsNameToRef)
	if err != nil {
		return err
	}
	volumeLocations := make([]url.URL, 0, len(mgr.configSpec.Storage.VolumeLocations))
	for _, volumeLocation := range mgr.configSpec.Storage.VolumeLocations {
		// Only apply changes to datastores managed by vSphere
		if volumeLocation.Scheme != "ds" {
			continue
		}
		volumeLocations = append(volumeLocations, *volumeLocation)
	}
	if err = mgr.findDatastores(ctx, finder, volumeLocations, dsNameToRef); err != nil {
		return err
	}
	return nil
}

func (mgr *RBACManager) findDatastores(ctx context.Context, finder *find.Finder,
	storeURLs []url.URL, dsNameToRef rbac.NameToRef) error {
	for _, storeURL := range storeURLs {
		dsName := storeURL.Host
		// Skip if we already have one
		if _, ok := dsNameToRef[dsName]; ok {
			continue
		}
		ds, err := finder.Datastore(ctx, dsName)
		if err != nil {
			return err
		}
		dsNameToRef[dsName] = ds.Reference()
	}
	return nil
}
