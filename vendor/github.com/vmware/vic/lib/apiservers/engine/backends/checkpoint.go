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
	"github.com/docker/docker/api/types"

	"github.com/vmware/vic/lib/apiservers/engine/errors"
)

type CheckpointBackend struct {
}

func NewCheckpointBackend() *CheckpointBackend {
	return &CheckpointBackend{}
}

func (c *CheckpointBackend) CheckpointCreate(container string, config types.CheckpointCreateOptions) error {
	return errors.APINotSupportedMsg(ProductName(), "checkpointing")
}

func (c *CheckpointBackend) CheckpointDelete(container string, config types.CheckpointDeleteOptions) error {
	return errors.APINotSupportedMsg(ProductName(), "checkpointing")
}

func (c *CheckpointBackend) CheckpointList(container string, config types.CheckpointListOptions) ([]types.Checkpoint, error) {
	return nil, errors.APINotSupportedMsg(ProductName(), "checkpointing")
}
