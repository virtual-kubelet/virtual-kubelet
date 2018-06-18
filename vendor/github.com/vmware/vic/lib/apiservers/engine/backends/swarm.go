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

package backends

import (
	"golang.org/x/net/context"

	basictypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/backend"
	types "github.com/docker/docker/api/types/swarm"

	"github.com/vmware/vic/lib/apiservers/engine/errors"
)

type SwarmBackend struct {
}

func NewSwarmBackend() *SwarmBackend {
	return &SwarmBackend{}
}

func (s *SwarmBackend) Init(req types.InitRequest) (string, error) {
	return "", errors.SwarmNotSupportedError()
}

func (s *SwarmBackend) Join(req types.JoinRequest) error {
	return errors.SwarmNotSupportedError()
}

func (s *SwarmBackend) Leave(force bool) error {
	return errors.SwarmNotSupportedError()
}

func (s *SwarmBackend) Inspect() (types.Swarm, error) {
	return types.Swarm{}, errors.SwarmNotSupportedError()
}

func (s *SwarmBackend) Update(uint64, types.Spec, types.UpdateFlags) error {
	return errors.SwarmNotSupportedError()
}

func (s *SwarmBackend) GetUnlockKey() (string, error) {
	return "", errors.SwarmNotSupportedError()
}

func (s *SwarmBackend) UnlockSwarm(req types.UnlockRequest) error {
	return errors.SwarmNotSupportedError()
}

func (s *SwarmBackend) GetServices(basictypes.ServiceListOptions) ([]types.Service, error) {
	return nil, errors.SwarmNotSupportedError()
}

func (s *SwarmBackend) GetService(string) (types.Service, error) {
	return types.Service{}, errors.SwarmNotSupportedError()
}

func (s *SwarmBackend) CreateService(types.ServiceSpec, string) (*basictypes.ServiceCreateResponse, error) {
	return nil, errors.SwarmNotSupportedError()
}

func (s *SwarmBackend) UpdateService(string, uint64, types.ServiceSpec, string, string) (*basictypes.ServiceUpdateResponse, error) {
	return nil, errors.SwarmNotSupportedError()
}

func (s *SwarmBackend) RemoveService(string) error {
	return errors.SwarmNotSupportedError()
}

func (s *SwarmBackend) ServiceLogs(context.Context, string, *backend.ContainerLogsConfig, chan struct{}) error {
	return errors.SwarmNotSupportedError()
}

func (s *SwarmBackend) GetNodes(basictypes.NodeListOptions) ([]types.Node, error) {
	return nil, errors.SwarmNotSupportedError()
}

func (s *SwarmBackend) GetNode(string) (types.Node, error) {
	return types.Node{}, errors.SwarmNotSupportedError()
}

func (s *SwarmBackend) UpdateNode(string, uint64, types.NodeSpec) error {
	return errors.SwarmNotSupportedError()
}

func (s *SwarmBackend) RemoveNode(string, bool) error {
	return errors.SwarmNotSupportedError()
}

func (s *SwarmBackend) GetTasks(basictypes.TaskListOptions) ([]types.Task, error) {
	return nil, errors.SwarmNotSupportedError()
}

func (s *SwarmBackend) GetTask(string) (types.Task, error) {
	return types.Task{}, errors.SwarmNotSupportedError()
}

func (s *SwarmBackend) GetSecrets(opts basictypes.SecretListOptions) ([]types.Secret, error) {
	return nil, errors.SwarmNotSupportedError()
}

func (s *SwarmBackend) CreateSecret(sp types.SecretSpec) (string, error) {
	return "", errors.SwarmNotSupportedError()
}

func (s *SwarmBackend) RemoveSecret(id string) error {
	return errors.SwarmNotSupportedError()
}

func (s *SwarmBackend) GetSecret(id string) (types.Secret, error) {
	return types.Secret{}, errors.SwarmNotSupportedError()
}

func (s *SwarmBackend) UpdateSecret(id string, version uint64, spec types.SecretSpec) error {
	return errors.SwarmNotSupportedError()
}
