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

package backends

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"

	"net/http"

	log "github.com/Sirupsen/logrus"
	derr "github.com/docker/docker/api/errors"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	apinet "github.com/docker/docker/api/types/network"
	"github.com/docker/libnetwork"
	"github.com/docker/libnetwork/networkdb"

	"github.com/vmware/vic/lib/apiservers/engine/backends/cache"
	"github.com/vmware/vic/lib/apiservers/engine/backends/convert"
	vicendpoint "github.com/vmware/vic/lib/apiservers/engine/backends/endpoint"
	"github.com/vmware/vic/lib/apiservers/engine/errors"
	"github.com/vmware/vic/lib/apiservers/portlayer/client/containers"
	"github.com/vmware/vic/lib/apiservers/portlayer/client/scopes"
	"github.com/vmware/vic/lib/apiservers/portlayer/models"
	"github.com/vmware/vic/pkg/retry"
	"github.com/vmware/vic/pkg/trace"
)

type NetworkBackend struct {
}

func NewNetworkBackend() *NetworkBackend {
	return &NetworkBackend{}
}

func (n *NetworkBackend) NetworkControllerEnabled() bool {
	return false
}

func (n *NetworkBackend) FindNetwork(idName string) (libnetwork.Network, error) {
	op := trace.NewOperation(context.Background(), "FindNetwork: %s", idName)
	defer trace.End(trace.Audit(idName, op))
	opID := op.ID()

	ok, err := PortLayerClient().Scopes.List(scopes.NewListParamsWithContext(op).WithOpID(&opID).WithIDName(idName))
	if err != nil {
		switch err := err.(type) {
		case *scopes.ListNotFound:
			return nil, derr.NewRequestNotFoundError(fmt.Errorf("network %s not found", idName))

		case *scopes.ListDefault:
			return nil, derr.NewErrorWithStatusCode(fmt.Errorf(err.Payload.Message), http.StatusInternalServerError)

		default:
			return nil, derr.NewErrorWithStatusCode(err, http.StatusInternalServerError)
		}
	}

	return &vicnetwork{cfg: ok.Payload[0]}, nil
}

func (n *NetworkBackend) GetNetworkByName(idName string) (libnetwork.Network, error) {
	op := trace.NewOperation(context.Background(), "GetNetworkByName: %s", idName)
	defer trace.End(trace.Audit(idName, op))
	opID := op.ID()

	ok, err := PortLayerClient().Scopes.List(scopes.NewListParamsWithContext(op).WithOpID(&opID).WithIDName(idName))
	if err != nil {
		switch err := err.(type) {
		case *scopes.ListNotFound:
			return nil, nil

		case *scopes.ListDefault:
			return nil, derr.NewErrorWithStatusCode(fmt.Errorf(err.Payload.Message), http.StatusInternalServerError)

		default:
			return nil, derr.NewErrorWithStatusCode(err, http.StatusInternalServerError)
		}
	}

	return &vicnetwork{cfg: ok.Payload[0]}, nil
}

func (n *NetworkBackend) GetNetworksByID(partialID string) []libnetwork.Network {
	op := trace.NewOperation(context.Background(), "GetNetworksByID: %s", partialID)
	defer trace.End(trace.Audit(partialID, op))
	opID := op.ID()

	ok, err := PortLayerClient().Scopes.List(scopes.NewListParamsWithContext(op).WithOpID(&opID).WithIDName(partialID))
	if err != nil {
		return nil
	}

	nets := make([]libnetwork.Network, len(ok.Payload))
	for i, cfg := range ok.Payload {
		nets[i] = &vicnetwork{cfg: cfg}
	}

	return nets
}

func (n *NetworkBackend) GetNetworks() []libnetwork.Network {
	op := trace.NewOperation(context.Background(), "GetNetworks")
	defer trace.End(trace.Audit("", op))
	opID := op.ID()

	ok, err := PortLayerClient().Scopes.ListAll(scopes.NewListAllParamsWithContext(op).WithOpID(&opID))
	if err != nil {
		return nil
	}

	nets := make([]libnetwork.Network, len(ok.Payload))
	for i, cfg := range ok.Payload {
		nets[i] = &vicnetwork{cfg: cfg}
		i++
	}

	return nets
}

func (n *NetworkBackend) CreateNetwork(nc types.NetworkCreateRequest) (*types.NetworkCreateResponse, error) {
	op := trace.NewOperation(context.Background(), "CreateNetwork: %s", nc.Name)
	defer trace.End(trace.Audit(nc.Name, op))
	opID := op.ID()

	if nc.IPAM != nil && len(nc.IPAM.Config) > 1 {
		return nil, fmt.Errorf("at most one ipam config supported")
	}

	var gateway, subnet string
	var pools []string
	if nc.IPAM != nil && len(nc.IPAM.Config) > 0 {
		if nc.IPAM.Config[0].Gateway != "" {
			gateway = nc.IPAM.Config[0].Gateway
		}

		if nc.IPAM.Config[0].Subnet != "" {
			subnet = nc.IPAM.Config[0].Subnet
		}

		if nc.IPAM.Config[0].IPRange != "" {
			pools = append(pools, nc.IPAM.Config[0].IPRange)
		}
	}

	if nc.Driver == "" {
		nc.Driver = "bridge"
	}

	cfg := &models.ScopeConfig{
		Gateway:     gateway,
		Name:        nc.Name,
		ScopeType:   nc.Driver,
		Subnet:      subnet,
		IPAM:        pools,
		Annotations: make(map[string]string),
		Internal:    nc.Internal,
	}

	// Marshal and encode the labels for transport and storage in the portlayer
	if labelsBytes, err := json.Marshal(nc.Labels); err == nil {
		encodedLabels := base64.StdEncoding.EncodeToString(labelsBytes)
		cfg.Annotations[convert.AnnotationKeyLabels] = encodedLabels
	} else {
		op.Errorf("error marshaling labels: %s", err)
		return nil, derr.NewErrorWithStatusCode(fmt.Errorf("unable to marshal labels: %s", err), http.StatusInternalServerError)
	}

	created, err := PortLayerClient().Scopes.CreateScope(scopes.NewCreateScopeParamsWithContext(op).WithOpID(&opID).WithConfig(cfg))
	if err != nil {
		switch err := err.(type) {
		case *scopes.CreateScopeConflict:
			return nil, derr.NewErrorWithStatusCode(fmt.Errorf("vicnetwork %s already exists", nc.Name), http.StatusConflict)

		case *scopes.CreateScopeDefault:
			return nil, derr.NewErrorWithStatusCode(fmt.Errorf(err.Payload.Message), http.StatusInternalServerError)

		default:
			return nil, derr.NewErrorWithStatusCode(err, http.StatusInternalServerError)
		}
	}

	ncResponse := &types.NetworkCreateResponse{
		ID:      created.Payload.ID,
		Warning: "",
	}

	return ncResponse, nil
}

// isCommitConflictError returns true if err is a conflict error from the portlayer's
// handle commit operation, and false otherwise.
func isCommitConflictError(err error) bool {
	_, isConflictErr := err.(*containers.CommitConflict)
	return isConflictErr
}

// connectContainerToNetwork performs portlayer operations to connect a container to a container vicnetwork.
func connectContainerToNetwork(op trace.Operation, containerName, networkName string, endpointConfig *apinet.EndpointSettings) error {
	opID := op.ID()
	client := PortLayerClient()
	getRes, err := client.Containers.Get(containers.NewGetParamsWithContext(op).WithOpID(&opID).WithID(containerName))
	if err != nil {
		switch err := err.(type) {
		case *containers.GetNotFound:
			return derr.NewRequestNotFoundError(fmt.Errorf(err.Payload.Message))

		case *containers.GetDefault:
			return derr.NewErrorWithStatusCode(fmt.Errorf(err.Payload.Message), http.StatusInternalServerError)

		default:
			return derr.NewErrorWithStatusCode(err, http.StatusInternalServerError)
		}
	}

	h := getRes.Payload
	nc := &models.NetworkConfig{NetworkName: networkName}
	if endpointConfig != nil {
		if endpointConfig.IPAMConfig != nil && endpointConfig.IPAMConfig.IPv4Address != "" {
			nc.Address = endpointConfig.IPAMConfig.IPv4Address

		}

		// Pass Links and Aliases to PL.
		nc.Aliases = vicendpoint.Alias(endpointConfig)
	}

	addConRes, err := client.Scopes.AddContainer(scopes.NewAddContainerParamsWithContext(op).
		WithOpID(&opID).
		WithScope(nc.NetworkName).
		WithConfig(&models.ScopesAddContainerConfig{
			Handle:        h,
			NetworkConfig: nc,
		}))
	if err != nil {
		switch err := err.(type) {
		case *scopes.AddContainerNotFound:
			return derr.NewRequestNotFoundError(fmt.Errorf(err.Payload.Message))

		case *scopes.AddContainerInternalServerError:
			return derr.NewErrorWithStatusCode(fmt.Errorf(err.Payload.Message), http.StatusInternalServerError)

		default:
			return derr.NewErrorWithStatusCode(err, http.StatusInternalServerError)
		}
	}

	h = addConRes.Payload

	// Get the power state of the container.
	getStateRes, err := client.Containers.GetState(containers.NewGetStateParamsWithContext(op).WithOpID(&opID).WithHandle(h))
	if err != nil {
		switch err := err.(type) {
		case *containers.GetStateNotFound:
			return derr.NewRequestNotFoundError(fmt.Errorf(err.Payload.Message))

		case *containers.GetStateDefault:
			return derr.NewErrorWithStatusCode(fmt.Errorf(err.Payload.Message), http.StatusInternalServerError)

		default:
			return derr.NewErrorWithStatusCode(err, http.StatusInternalServerError)
		}
	}

	h = getStateRes.Payload.Handle
	// Only bind if the container is running.
	if getStateRes.Payload.State == "RUNNING" {
		bindRes, err := client.Scopes.BindContainer(scopes.NewBindContainerParamsWithContext(op).WithOpID(&opID).WithHandle(h))
		if err != nil {
			switch err := err.(type) {
			case *scopes.BindContainerNotFound:
				return derr.NewRequestNotFoundError(fmt.Errorf(err.Payload.Message))

			case *scopes.BindContainerInternalServerError:
				return derr.NewErrorWithStatusCode(fmt.Errorf(err.Payload.Message), http.StatusInternalServerError)

			default:
				return derr.NewErrorWithStatusCode(err, http.StatusInternalServerError)
			}
		}

		defer func() {
			if err == nil {
				return
			}
			if _, err2 := client.Scopes.UnbindContainer(scopes.NewUnbindContainerParamsWithContext(op).
				WithOpID(&opID).
				WithHandle(h)); err2 != nil {
				op.Warnf("failed bind container rollback: %s", err2)
			}
		}()

		h = bindRes.Payload.Handle
	}

	// Commit the handle.
	_, err = client.Containers.Commit(containers.NewCommitParamsWithContext(op).WithOpID(&opID).WithHandle(h))
	return err
}

// ConnectContainerToNetwork connects a container to a container vicnetwork. It wraps the portlayer operations
// in a retry for when there's a conflict error received, such as one during a similar concurrent operation.
func (n *NetworkBackend) ConnectContainerToNetwork(containerName, networkName string, endpointConfig *apinet.EndpointSettings) error {
	op := trace.NewOperation(context.Background(), "ConnectContainerToNetwork: %s to %s", containerName, networkName)
	defer trace.End(trace.Audit(containerName, op))

	vc := cache.ContainerCache().GetContainer(containerName)
	if vc != nil {
		containerName = vc.ContainerID
	}

	operation := func() error {
		return connectContainerToNetwork(op, containerName, networkName, endpointConfig)
	}

	config := retry.NewBackoffConfig()
	config.MaxElapsedTime = maxElapsedTime
	err := retry.DoWithConfig(operation, isCommitConflictError, config)
	if err != nil {
		switch err := err.(type) {
		case *containers.CommitNotFound:
			return derr.NewRequestNotFoundError(fmt.Errorf(err.Payload.Message))

		case *containers.CommitDefault:
			return derr.NewErrorWithStatusCode(fmt.Errorf(err.Payload.Message), http.StatusInternalServerError)

		default:
			return derr.NewErrorWithStatusCode(err, http.StatusInternalServerError)
		}
	}

	return nil
}

func (n *NetworkBackend) DisconnectContainerFromNetwork(containerName, networkName string, force bool) error {
	op := trace.NewOperation(context.Background(), "DisconnectContainerFromNetwork: %s to %s", containerName, networkName)
	defer trace.End(trace.Audit(containerName, op))

	vc := cache.ContainerCache().GetContainer(containerName)
	if vc != nil {
		containerName = vc.ContainerID
	}
	return errors.APINotSupportedMsg(ProductName(), "DisconnectContainerFromNetwork")
}

func (n *NetworkBackend) DeleteNetwork(name string) error {
	op := trace.NewOperation(context.Background(), "DeleteNetwork: %s", name)
	defer trace.End(trace.Audit(name, op))
	opID := op.ID()

	client := PortLayerClient()

	if _, err := client.Scopes.DeleteScope(scopes.NewDeleteScopeParamsWithContext(op).
		WithOpID(&opID).
		WithIDName(name)); err != nil {
		switch err := err.(type) {
		case *scopes.DeleteScopeNotFound:
			return derr.NewRequestNotFoundError(fmt.Errorf("network %s not found", name))

		case *scopes.DeleteScopeInternalServerError:
			return derr.NewErrorWithStatusCode(fmt.Errorf(err.Payload.Message), http.StatusInternalServerError)

		default:
			return derr.NewErrorWithStatusCode(err, http.StatusInternalServerError)
		}
	}

	return nil
}

func (n *NetworkBackend) NetworksPrune(pruneFilters filters.Args) (*types.NetworksPruneReport, error) {
	return nil, errors.APINotSupportedMsg(ProductName(), "NetworksPrune")
}

// vicnetwork implements the libnetwork.Network and libnetwork.NetworkInfo interfaces
type vicnetwork struct {
	sync.Mutex

	cfg *models.ScopeConfig
}

// A user chosen name for this vicnetwork.
func (n *vicnetwork) Name() string {
	return n.cfg.Name
}

// A system generated id for this vicnetwork.
func (n *vicnetwork) ID() string {
	return n.cfg.ID
}

// The type of vicnetwork, which corresponds to its managing driver.
func (n *vicnetwork) Type() string {
	return n.cfg.ScopeType
}

// Create a new endpoint to this vicnetwork symbolically identified by the
// specified unique name. The options parameter carry driver specific options.
func (n *vicnetwork) CreateEndpoint(name string, options ...libnetwork.EndpointOption) (libnetwork.Endpoint, error) {
	return nil, fmt.Errorf("not implemented")
}

// Delete the vicnetwork.
func (n *vicnetwork) Delete() error {
	return fmt.Errorf("not implemented")
}

// Endpoints returns the list of Endpoint(s) in this vicnetwork.
func (n *vicnetwork) Endpoints() []libnetwork.Endpoint {
	eps := make([]libnetwork.Endpoint, len(n.cfg.Endpoints))
	for i, e := range n.cfg.Endpoints {
		eps[i] = &endpoint{ep: e, sc: n.cfg}
	}

	return eps
}

// WalkEndpoints uses the provided function to walk the Endpoints
func (n *vicnetwork) WalkEndpoints(walker libnetwork.EndpointWalker) {
	for _, e := range n.cfg.Endpoints {
		if walker(&endpoint{ep: e, sc: n.cfg}) {
			return
		}
	}
}

// EndpointByName returns the Endpoint which has the passed name. If not found, the error ErrNoSuchEndpoint is returned.
func (n *vicnetwork) EndpointByName(name string) (libnetwork.Endpoint, error) {
	for _, e := range n.cfg.Endpoints {
		if e.Name == name {
			return &endpoint{ep: e, sc: n.cfg}, nil
		}
	}

	return nil, fmt.Errorf("not found")
}

// EndpointByID returns the Endpoint which has the passed id. If not found, the error ErrNoSuchEndpoint is returned.
func (n *vicnetwork) EndpointByID(id string) (libnetwork.Endpoint, error) {
	for _, e := range n.cfg.Endpoints {
		if e.ID == id {
			return &endpoint{ep: e, sc: n.cfg}, nil
		}
	}

	return nil, fmt.Errorf("not found")
}

// Return certain operational data belonging to this vicnetwork
func (n *vicnetwork) Info() libnetwork.NetworkInfo {
	return n
}

func (n *vicnetwork) IpamConfig() (string, map[string]string, []*libnetwork.IpamConf, []*libnetwork.IpamConf) {
	n.Lock()
	defer n.Unlock()

	confs := make([]*libnetwork.IpamConf, len(n.cfg.IPAM))
	for j, i := range n.cfg.IPAM {
		conf := &libnetwork.IpamConf{
			PreferredPool: n.cfg.Subnet,
			Gateway:       "",
		}

		if i != n.cfg.Subnet {
			conf.SubPool = i
		}

		if n.cfg.Gateway != "" {
			conf.Gateway = n.cfg.Gateway
		}

		confs[j] = conf
	}

	return "", make(map[string]string), confs, nil
}

func (n *vicnetwork) IpamInfo() ([]*libnetwork.IpamInfo, []*libnetwork.IpamInfo) {
	n.Lock()
	defer n.Unlock()

	var infos []*libnetwork.IpamInfo
	for _, i := range n.cfg.IPAM {
		_, pool, err := net.ParseCIDR(i)
		if err != nil {
			continue
		}

		info := &libnetwork.IpamInfo{
			Meta: make(map[string]string),
		}

		info.Pool = pool
		if n.cfg.Gateway != "" {
			info.Gateway = &net.IPNet{
				IP:   net.ParseIP(n.cfg.Gateway),
				Mask: net.CIDRMask(32, 32),
			}
		}

		info.AuxAddresses = make(map[string]*net.IPNet)
		infos = append(infos, info)
	}

	return infos, nil
}

func (n *vicnetwork) DriverOptions() map[string]string {
	return make(map[string]string)
}

func (n *vicnetwork) Scope() string {
	return ""
}

func (n *vicnetwork) IPv6Enabled() bool {
	return false
}

func (n *vicnetwork) Internal() bool {
	n.Lock()
	defer n.Unlock()

	return n.cfg.Internal
}

// Labels decodes and unmarshals the stored blob of vicnetwork labels.
func (n *vicnetwork) Labels() map[string]string {
	n.Lock()
	defer n.Unlock()

	labels := make(map[string]string)
	if n.cfg.Annotations == nil {
		return labels
	}

	// Look for the Docker-specific annotation (label) blob and process it for the output
	if encodedLabels, ok := n.cfg.Annotations[convert.AnnotationKeyLabels]; ok {

		if labelsBytes, decodeErr := base64.StdEncoding.DecodeString(encodedLabels); decodeErr == nil {
			if unmarshalErr := json.Unmarshal(labelsBytes, &labels); unmarshalErr != nil {
				log.Errorf("error unmarshaling labels: %s", unmarshalErr)
			}
		} else {
			log.Errorf("error decoding label blob: %s", decodeErr)
		}
	}

	return labels
}

func (n *vicnetwork) Attachable() bool {
	return false //?
}

func (n *vicnetwork) Dynamic() bool {
	return false //?
}

func (n *vicnetwork) Created() time.Time {
	return time.Now()
}

// Peers returns a slice of PeerInfo structures which has the information about the peer
// nodes participating in the same overlay vicnetwork. This is currently the per-vicnetwork
// gossip cluster. For non-dynamic overlay networks and bridge networks it returns an
// empty slice
func (n *vicnetwork) Peers() []networkdb.PeerInfo {
	return nil
}
