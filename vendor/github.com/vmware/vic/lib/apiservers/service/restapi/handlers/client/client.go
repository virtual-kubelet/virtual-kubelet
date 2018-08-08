// Copyright 2018 VMware, Inc. All Rights Reserved.
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

// Package client enables handler code to easily and consistently validate and dispatch calls to lib/install.
package client

import (
	"context"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"

	"github.com/docker/docker/opts"

	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/list"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/vmware/vic/lib/apiservers/service/restapi/handlers/errors"
	"github.com/vmware/vic/lib/config"
	"github.com/vmware/vic/lib/constants"
	"github.com/vmware/vic/lib/install/data"
	"github.com/vmware/vic/lib/install/management"
	"github.com/vmware/vic/lib/install/validate"
	"github.com/vmware/vic/lib/install/vchlog"
	"github.com/vmware/vic/pkg/ip"
	"github.com/vmware/vic/pkg/trace"
	"github.com/vmware/vic/pkg/vsphere/datastore"
	"github.com/vmware/vic/pkg/vsphere/session"
	"github.com/vmware/vic/pkg/vsphere/vm"
)

type Executor interface {
	CreateVCH(conf *config.VirtualContainerHostConfigSpec, settings *data.InstallerData, receiver vchlog.Receiver) error
	DeleteVCH(conf *config.VirtualContainerHostConfigSpec, containers *management.DeleteContainers, volumeStores *management.DeleteVolumeStores) error
}

type executor interface {
	Executor

	NewVCHFromID(id string) (*vm.VirtualMachine, error)
	SearchVCHs(computePath string) ([]*vm.VirtualMachine, error)
	GetNoSecretVCHConfig(vm *vm.VirtualMachine) (*config.VirtualContainerHostConfigSpec, error)
	GetTLSFriendlyHostIP(clientIP net.IP, cert *x509.Certificate, certificateAuthorities []byte) string
}

type Finder interface {
	Element(context.Context, types.ManagedObjectReference) (*list.Element, error)
}

type finder interface {
	Finder
	validate.Finder

	Datastore(ctx context.Context, path string) (*object.Datastore, error)
}

type Validator interface {
	Validate(ctx context.Context, input *data.Data, allowEmptyDC bool) (*config.VirtualContainerHostConfigSpec, error)
	GetIssues() []error

	// TODO (#6032): This probably doesn't belong here.
	AddDeprecatedFields(ctx context.Context, conf *config.VirtualContainerHostConfigSpec, input *data.Data) *data.InstallerData
}

type validator interface {
	Validator

	SetDataFromVM(ctx context.Context, vm *vm.VirtualMachine, d *data.Data) error
}

type HandlerClient struct {
	executor  executor
	finder    finder
	session   *session.Session
	validator validator
}

func NewHandlerClient(op trace.Operation, action management.Action, finder *find.Finder, session *session.Session, validator *validate.Validator) (*HandlerClient, error) {
	executor := management.NewDispatcher(op, session, action, false)

	c := HandlerClient{
		executor:  executor,
		finder:    finder,
		session:   session,
		validator: validator}

	return &c, nil
}

func (c *HandlerClient) Executor() Executor {
	return c.executor
}

func (c *HandlerClient) Finder() Finder {
	return c.finder
}

func (c *HandlerClient) Validator() Validator {
	return c.validator
}

func (c *HandlerClient) GetVCH(op trace.Operation, d *data.Data) (*vm.VirtualMachine, error) {
	vch, err := c.executor.NewVCHFromID(d.ID)
	if err != nil {
		return nil, errors.NewError(http.StatusNotFound, "unable to find VCH %s: %s", d.ID, err)
	}

	err = c.validator.SetDataFromVM(op, vch, d)
	if err != nil {
		return nil, errors.NewError(http.StatusInternalServerError, "failed to load VCH data: %s", err)
	}

	return vch, err
}

func (c *HandlerClient) GetVCHs(op trace.Operation) ([]*vm.VirtualMachine, error) {
	vchs, err := c.executor.SearchVCHs(c.session.ClusterPath)
	if err != nil {
		return nil, errors.NewError(http.StatusInternalServerError, "failed to search VCHs in %s: %s", c.session.PoolPath, err)
	}

	return vchs, err
}

func (c *HandlerClient) GetConfigForVCH(op trace.Operation, vch *vm.VirtualMachine) (*config.VirtualContainerHostConfigSpec, error) {
	vchConfig, err := c.executor.GetNoSecretVCHConfig(vch)
	if err != nil {
		return nil, errors.NewError(http.StatusInternalServerError, "unable to retrieve VCH information: %s", err)
	}

	return vchConfig, nil
}

func (c *HandlerClient) GetVCHConfig(op trace.Operation, d *data.Data) (*config.VirtualContainerHostConfigSpec, error) {
	vch, err := c.GetVCH(op, d)
	if err != nil {
		return nil, err
	}

	return c.GetConfigForVCH(op, vch)
}

// GetDatastoreHelper validates the VCH and returns the datastore helper for the VCH. It errors when validation fails or when datastore is not ready
func (c *HandlerClient) GetDatastoreHelper(op trace.Operation, d *data.Data) (*datastore.Helper, error) {
	vch, err := c.GetVCH(op, d)
	if err != nil {
		return nil, err
	}

	// Relative path of datastore folder
	vmPath, err := vch.VMPathNameAsURL(op)
	if err != nil {
		return nil, errors.NewError(http.StatusNotFound, "unable to retrieve VCH datastore information: %s", err)
	}

	// Get VCH datastore object
	ds, err := c.finder.Datastore(op, vmPath.Host)
	if err != nil {
		return nil, errors.NewError(http.StatusNotFound, "datastore folder not found for VCH %s: %s", d.ID, err)
	}

	// Create a new datastore helper for file finding
	helper, err := datastore.NewHelper(op, c.session, ds, vmPath.Path)
	if err != nil {
		return nil, errors.NewError(http.StatusInternalServerError, "unable to get datastore helper: %s", err)
	}

	return helper, nil
}

func (c *HandlerClient) GetAddresses(vchConfig *config.VirtualContainerHostConfigSpec) (string, string, error) {
	clientNet := vchConfig.ExecutorConfig.Networks["client"]
	if clientNet != nil {
		clientIP := clientNet.Assigned.IP

		if ip.IsUnspecifiedIP(clientIP) {
			return "", "", fmt.Errorf("no client IP address assigned")
		}

		hostIP := clientIP.String()
		// try looking up preferred name irrespective of CAs, if available
		// once we found a preferred address, we use that instead of the assigned client IP address
		if cert, err := vchConfig.HostCertificate.X509Certificate(); err == nil {
			hostIP = c.executor.GetTLSFriendlyHostIP(clientIP, cert, vchConfig.CertificateAuthorities)
		}

		var dockerPort int
		if !vchConfig.HostCertificate.IsNil() {
			dockerPort = opts.DefaultTLSHTTPPort
		} else {
			dockerPort = opts.DefaultHTTPPort
		}

		dockerHost := fmt.Sprintf("%s:%d", hostIP, dockerPort)
		adminPortal := fmt.Sprintf("https://%s:%d", hostIP, constants.VchAdminPortalPort)

		return dockerHost, adminPortal, nil
	}

	return "", "", fmt.Errorf("no client IP address assigned")
}
