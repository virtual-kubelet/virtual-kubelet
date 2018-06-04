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

package handlers

import (
	"context"
	"fmt"
	"net"
	"net/http"

	log "github.com/Sirupsen/logrus"
	"github.com/go-openapi/runtime/middleware"

	"github.com/vmware/vic/lib/apiservers/portlayer/models"
	"github.com/vmware/vic/lib/apiservers/portlayer/restapi/operations"
	"github.com/vmware/vic/lib/apiservers/portlayer/restapi/operations/scopes"
	"github.com/vmware/vic/lib/portlayer/exec"
	"github.com/vmware/vic/lib/portlayer/network"
	"github.com/vmware/vic/pkg/ip"
	"github.com/vmware/vic/pkg/trace"
)

// ScopesHandlersImpl is the receiver for all of the storage handler methods
type ScopesHandlersImpl struct {
	netCtx     *network.Context
	handlerCtx *HandlerContext
}

// Configure assigns functions to all the scopes api handlers
func (handler *ScopesHandlersImpl) Configure(api *operations.PortLayerAPI, handlerCtx *HandlerContext) {
	api.ScopesCreateScopeHandler = scopes.CreateScopeHandlerFunc(handler.ScopesCreate)
	api.ScopesDeleteScopeHandler = scopes.DeleteScopeHandlerFunc(handler.ScopesDelete)
	api.ScopesListAllHandler = scopes.ListAllHandlerFunc(handler.ScopesListAll)
	api.ScopesListHandler = scopes.ListHandlerFunc(handler.ScopesList)
	api.ScopesGetContainerEndpointsHandler = scopes.GetContainerEndpointsHandlerFunc(handler.ScopesGetContainerEndpoints)
	api.ScopesAddContainerHandler = scopes.AddContainerHandlerFunc(handler.ScopesAddContainer)
	api.ScopesRemoveContainerHandler = scopes.RemoveContainerHandlerFunc(handler.ScopesRemoveContainer)
	api.ScopesBindContainerHandler = scopes.BindContainerHandlerFunc(handler.ScopesBindContainer)
	api.ScopesUnbindContainerHandler = scopes.UnbindContainerHandlerFunc(handler.ScopesUnbindContainer)

	handler.netCtx = network.DefaultContext
	handler.handlerCtx = handlerCtx
}

func parseScopeConfig(cfg *models.ScopeConfig) (subnet *net.IPNet, gateway net.IP, dns []net.IP, annotations map[string]string, err error) {
	if cfg.Subnet != "" {
		if _, subnet, err = net.ParseCIDR(cfg.Subnet); err != nil {
			return
		}
	}

	gateway = net.IPv4(0, 0, 0, 0)
	if cfg.Gateway != "" {
		if gateway = net.ParseIP(cfg.Gateway); gateway == nil {
			err = fmt.Errorf("invalid gateway")
			return
		}
	}

	dns = make([]net.IP, len(cfg.DNS))
	for i, d := range cfg.DNS {
		dns[i] = net.ParseIP(d)
		if dns[i] == nil {
			err = fmt.Errorf("invalid dns entry")
			return
		}
	}

	// Parse annotations
	if len(cfg.Annotations) > 0 {
		annotations = make(map[string]string)
		for k, v := range cfg.Annotations {
			annotations[k] = v
		}
	}

	return
}

func (handler *ScopesHandlersImpl) listScopes(idName string) ([]*models.ScopeConfig, error) {
	defer trace.End(trace.Begin(idName))
	scs, err := handler.netCtx.Scopes(context.Background(), &idName)
	if err != nil {
		return nil, err
	}

	cfgs := make([]*models.ScopeConfig, len(scs))
	for i, s := range scs {
		cfgs[i] = toScopeConfig(s)
	}

	return cfgs, nil
}

func errorPayload(err error) *models.Error {
	return &models.Error{Message: err.Error()}
}

func (handler *ScopesHandlersImpl) ScopesCreate(params scopes.CreateScopeParams) middleware.Responder {
	defer trace.End(trace.Begin(""))

	cfg := params.Config
	if cfg.ScopeType == "external" {
		return scopes.NewCreateScopeDefault(http.StatusServiceUnavailable).WithPayload(
			&models.Error{Message: "cannot create external networks"})
	}

	subnet, gateway, dns, annotations, err := parseScopeConfig(cfg)
	if err != nil {
		return scopes.NewCreateScopeDefault(http.StatusServiceUnavailable).WithPayload(
			errorPayload(err))
	}

	scopeData := &network.ScopeData{
		ScopeType:   cfg.ScopeType,
		Name:        cfg.Name,
		Subnet:      subnet,
		Gateway:     gateway,
		DNS:         dns,
		Pools:       cfg.IPAM,
		Annotations: annotations,
		Internal:    cfg.Internal,
	}

	s, err := handler.netCtx.NewScope(context.Background(), scopeData)
	if _, ok := err.(network.DuplicateResourceError); ok {
		return scopes.NewCreateScopeConflict()
	}

	if err != nil {
		return scopes.NewCreateScopeDefault(http.StatusServiceUnavailable).WithPayload(
			errorPayload(err))
	}

	return scopes.NewCreateScopeCreated().WithPayload(toScopeConfig(s))
}

func (handler *ScopesHandlersImpl) ScopesDelete(params scopes.DeleteScopeParams) middleware.Responder {
	defer trace.End(trace.Begin(params.IDName))

	if err := handler.netCtx.DeleteScope(context.Background(), params.IDName); err != nil {
		switch err := err.(type) {
		case network.ResourceNotFoundError:
			return scopes.NewDeleteScopeNotFound().WithPayload(errorPayload(err))

		default:
			return scopes.NewDeleteScopeInternalServerError().WithPayload(errorPayload(err))
		}
	}

	return scopes.NewDeleteScopeOK()
}

func (handler *ScopesHandlersImpl) ScopesListAll(params scopes.ListAllParams) middleware.Responder {
	defer trace.End(trace.Begin(""))

	cfgs, err := handler.listScopes("")
	if err != nil {
		return scopes.NewListDefault(http.StatusServiceUnavailable).WithPayload(errorPayload(err))
	}

	return scopes.NewListAllOK().WithPayload(cfgs)
}

func (handler *ScopesHandlersImpl) ScopesList(params scopes.ListParams) middleware.Responder {
	defer trace.End(trace.Begin("ScopesList"))

	cfgs, err := handler.listScopes(params.IDName)
	if _, ok := err.(network.ResourceNotFoundError); ok {
		return scopes.NewListNotFound().WithPayload(errorPayload(err))
	}

	return scopes.NewListOK().WithPayload(cfgs)
}

func (handler *ScopesHandlersImpl) ScopesGetContainerEndpoints(params scopes.GetContainerEndpointsParams) middleware.Responder {
	defer trace.End(trace.Begin(params.HandleOrID))

	cid := params.HandleOrID
	// lookup by handle
	h := exec.GetHandle(cid)
	if h != nil {
		cid = h.ExecConfig.ID
	}

	c := handler.netCtx.Container(cid)
	if c == nil {
		return scopes.NewGetContainerEndpointsNotFound().WithPayload(errorPayload(fmt.Errorf("container not found")))
	}
	eps := c.Endpoints()
	ecs := make([]*models.EndpointConfig, len(eps))
	for i, e := range eps {
		ecs[i] = toEndpointConfig(e)
	}

	return scopes.NewGetContainerEndpointsOK().WithPayload(ecs)
}

func (handler *ScopesHandlersImpl) ScopesAddContainer(params scopes.AddContainerParams) middleware.Responder {
	defer trace.End(trace.Begin(fmt.Sprintf("handle(%s)", params.Config.Handle)))

	h := exec.GetHandle(params.Config.Handle)
	if h == nil {
		return scopes.NewAddContainerNotFound().WithPayload(&models.Error{Message: "container not found"})
	}

	err := func() error {
		addr := params.Config.NetworkConfig.Address
		var ip net.IP
		if addr != "" {
			ip = net.ParseIP(addr)
			if ip == nil {
				return fmt.Errorf("invalid ip address %q", addr)
			}
		}

		if len(params.Config.NetworkConfig.Aliases) > 0 {
			log.Debugf("Links/Aliases: %#v", params.Config.NetworkConfig.Aliases)
		}

		options := &network.AddContainerOptions{
			Scope:   params.Config.NetworkConfig.NetworkName,
			IP:      ip,
			Aliases: params.Config.NetworkConfig.Aliases,
			Ports:   params.Config.NetworkConfig.Ports,
		}
		return handler.netCtx.AddContainer(h, options)
	}()

	if err != nil {
		if _, ok := err.(*network.ResourceNotFoundError); ok {
			return scopes.NewAddContainerNotFound().WithPayload(errorPayload(err))
		}

		return scopes.NewAddContainerInternalServerError().WithPayload(errorPayload(err))
	}

	return scopes.NewAddContainerOK().WithPayload(h.String())
}

func (handler *ScopesHandlersImpl) ScopesRemoveContainer(params scopes.RemoveContainerParams) middleware.Responder {
	defer trace.End(trace.Begin(fmt.Sprintf("handle(%s)", params.Handle)))

	h := exec.GetHandle(params.Handle)
	if h == nil {
		return scopes.NewRemoveContainerNotFound().WithPayload(&models.Error{Message: "container not found"})
	}

	if err := handler.netCtx.RemoveContainer(h, params.Scope); err != nil {
		if _, ok := err.(*network.ResourceNotFoundError); ok {
			return scopes.NewRemoveContainerNotFound().WithPayload(errorPayload(err))
		}

		return scopes.NewRemoveContainerInternalServerError().WithPayload(errorPayload(err))
	}

	return scopes.NewRemoveContainerOK().WithPayload(h.String())
}

func (handler *ScopesHandlersImpl) ScopesBindContainer(params scopes.BindContainerParams) middleware.Responder {
	op := trace.NewOperation(context.Background(), params.Handle)
	defer trace.End(trace.Begin(fmt.Sprintf("handle(%s)", params.Handle), op))

	h := exec.GetHandle(params.Handle)
	if h == nil {
		return scopes.NewBindContainerNotFound().WithPayload(&models.Error{Message: "container not found"})
	}

	var endpoints []*network.Endpoint
	var err error
	if endpoints, err = handler.netCtx.BindContainer(op, h); err != nil {
		switch err := err.(type) {
		case network.ResourceNotFoundError:
			return scopes.NewBindContainerNotFound().WithPayload(errorPayload(err))

		default:
			return scopes.NewBindContainerInternalServerError().WithPayload(errorPayload(err))
		}
	}

	res := &models.BindContainerResponse{
		Handle:    h.String(),
		Endpoints: make([]*models.EndpointConfig, len(endpoints)),
	}
	for i, e := range endpoints {
		res.Endpoints[i] = toEndpointConfig(e)
	}

	return scopes.NewBindContainerOK().WithPayload(res)
}

func (handler *ScopesHandlersImpl) ScopesUnbindContainer(params scopes.UnbindContainerParams) middleware.Responder {
	op := trace.NewOperation(context.Background(), params.Handle)
	defer trace.End(trace.Begin(fmt.Sprintf("handle(%s)", params.Handle), op))

	h := exec.GetHandle(params.Handle)
	if h == nil {
		return scopes.NewUnbindContainerNotFound()
	}

	var endpoints []*network.Endpoint
	var err error
	if endpoints, err = handler.netCtx.UnbindContainer(op, h); err != nil {
		switch err := err.(type) {
		case network.ResourceNotFoundError:
			return scopes.NewUnbindContainerNotFound().WithPayload(errorPayload(err))

		default:
			return scopes.NewUnbindContainerInternalServerError().WithPayload(errorPayload(err))
		}
	}

	res := &models.UnbindContainerResponse{
		Handle:    h.String(),
		Endpoints: make([]*models.EndpointConfig, len(endpoints)),
	}
	for i, e := range endpoints {
		res.Endpoints[i] = toEndpointConfig(e)
	}

	return scopes.NewUnbindContainerOK().WithPayload(res)
}

func toScopeConfig(scope *network.Scope) *models.ScopeConfig {
	subnet := ""
	if !ip.IsUnspecifiedIP(scope.Subnet().IP) {
		subnet = scope.Subnet().String()
	}

	gateway := ""
	if !scope.Gateway().IsUnspecified() {
		gateway = scope.Gateway().String()
	}

	sc := &models.ScopeConfig{
		ID:        scope.ID().String(),
		Name:      scope.Name(),
		ScopeType: scope.Type(),
		Subnet:    subnet,
		Gateway:   gateway,
		Internal:  scope.Internal(),
	}

	var pools []string
	for _, p := range scope.Pools() {
		pools = append(pools, p.String())
	}

	sc.IPAM = pools
	if len(sc.IPAM) == 0 {
		sc.IPAM = []string{subnet}
	}

	eps := scope.Endpoints()
	sc.Endpoints = make([]*models.EndpointConfig, len(eps))
	for i, e := range eps {
		sc.Endpoints[i] = toEndpointConfig(e)
	}

	sc.Annotations = make(map[string]string)
	annotations := scope.Annotations()
	for k, v := range annotations {
		sc.Annotations[k] = v
	}

	return sc
}

func toEndpointConfig(e *network.Endpoint) *models.EndpointConfig {
	addr := ""
	if !ip.IsUnspecifiedIP(e.IP()) {
		addr = e.IP().String()
	}

	ports := e.Ports()
	ecports := make([]string, len(ports))
	for i, p := range e.Ports() {
		ecports[i] = p.FullString()
	}

	return &models.EndpointConfig{
		Address:   addr,
		Container: e.ID().String(),
		ID:        e.ID().String(),
		Name:      e.Name(),
		Scope:     e.Scope().Name(),
		Ports:     ecports,
	}
}
