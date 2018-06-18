// Copyright 2017 VMware, Inc. All Rights Reserved.
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

package executor

import (
	"io"
	"time"

	"golang.org/x/net/context"

	"github.com/docker/distribution"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	swarmtypes "github.com/docker/docker/api/types/swarm"
	clustertypes "github.com/docker/docker/daemon/cluster/provider"
	"github.com/docker/docker/plugin"
	"github.com/docker/docker/reference"
	"github.com/docker/libnetwork"
	"github.com/docker/libnetwork/cluster"
	networktypes "github.com/docker/libnetwork/types"
	"github.com/docker/swarmkit/agent/exec"

	"github.com/vmware/vic/lib/apiservers/engine/errors"
)

type SwarmBackend struct {
}

func (b SwarmBackend) CreateManagedNetwork(clustertypes.NetworkCreateRequest) error {
	return nil
}

func (b SwarmBackend) DeleteManagedNetwork(name string) error {
	return nil
}

func (b SwarmBackend) FindNetwork(idName string) (libnetwork.Network, error) {
	return nil, nil
}

func (b SwarmBackend) SetupIngress(req clustertypes.NetworkCreateRequest, nodeIP string) error {
	return nil
}

func (b SwarmBackend) PullImage(ctx context.Context, image, tag string, metaHeaders map[string][]string, authConfig *types.AuthConfig, outStream io.Writer) error {
	return nil
}

func (b SwarmBackend) CreateManagedContainer(config types.ContainerCreateConfig) (container.ContainerCreateCreatedBody, error) {
	return container.ContainerCreateCreatedBody{}, nil
}

func (b SwarmBackend) ContainerStart(name string, hostConfig *container.HostConfig, checkpoint string, checkpointDir string) error {
	return nil
}

func (b SwarmBackend) ContainerStop(name string, seconds *int) error {
	return nil
}

// ContainerLogs hooks up a container's stdout and stderr streams
// configured with the given struct.
func (b SwarmBackend) ContainerLogs(ctx context.Context, containerName string, config *backend.ContainerLogsConfig, started chan struct{}) error {
	return nil
}

func (b SwarmBackend) ConnectContainerToNetwork(containerName, networkName string, endpointConfig *network.EndpointSettings) error {
	return nil
}

func (b SwarmBackend) ActivateContainerServiceBinding(containerName string) error {
	return nil
}

func (b SwarmBackend) DeactivateContainerServiceBinding(containerName string) error {
	return nil
}

func (b SwarmBackend) UpdateContainerServiceConfig(containerName string, serviceConfig *clustertypes.ServiceConfig) error {
	return nil
}

func (b SwarmBackend) ContainerInspectCurrent(name string, size bool) (*types.ContainerJSON, error) {
	return nil, nil
}

func (b SwarmBackend) ContainerWaitWithContext(ctx context.Context, name string) error {
	return nil
}

func (b SwarmBackend) ContainerRm(name string, config *types.ContainerRmConfig) error {
	return nil
}

func (b SwarmBackend) ContainerKill(name string, sig uint64) error {
	return nil
}

func (b SwarmBackend) SetContainerSecretStore(name string, store exec.SecretGetter) error {
	return nil
}

func (b SwarmBackend) SetContainerSecretReferences(name string, refs []*swarmtypes.SecretReference) error {
	return nil
}

func (b SwarmBackend) SystemInfo() (*types.Info, error) {
	return nil, nil
}

func (b SwarmBackend) VolumeCreate(name, driverName string, opts, labels map[string]string) (*types.Volume, error) {
	return nil, nil
}

func (b SwarmBackend) Containers(config *types.ContainerListOptions) ([]*types.Container, error) {
	return nil, nil
}

func (b SwarmBackend) SetNetworkBootstrapKeys([]*networktypes.EncryptionKey) error {
	return nil
}

func (b SwarmBackend) SetClusterProvider(provider cluster.Provider) {
}

func (b SwarmBackend) IsSwarmCompatible() error {
	return errors.SwarmNotSupportedError()
}

func (b SwarmBackend) SubscribeToEvents(since, until time.Time, filter filters.Args) ([]events.Message, chan interface{}) {
	return nil, nil
}

func (b SwarmBackend) UnsubscribeFromEvents(listener chan interface{}) {
}

func (b SwarmBackend) UpdateAttachment(string, string, string, *network.NetworkingConfig) error {
	return nil
}

func (b SwarmBackend) WaitForDetachment(context.Context, string, string, string, string) error {
	return nil
}

func (b SwarmBackend) GetRepository(context.Context, reference.NamedTagged, *types.AuthConfig) (distribution.Repository, bool, error) {
	return nil, false, nil
}

func (b SwarmBackend) LookupImage(name string) (*types.ImageInspect, error) {
	return nil, nil
}

func (b SwarmBackend) PluginManager() *plugin.Manager {
	return nil
}
