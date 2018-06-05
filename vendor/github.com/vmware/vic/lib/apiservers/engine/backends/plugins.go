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
	"io"
	"net/http"

	enginetypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/reference"
	"golang.org/x/net/context"

	"github.com/vmware/vic/lib/apiservers/engine/errors"
)

type PluginBackend struct {
}

func NewPluginBackend() *PluginBackend {
	return &PluginBackend{}
}

func (p *PluginBackend) Disable(name string, config *enginetypes.PluginDisableConfig) error {
	return errors.APINotSupportedMsg(ProductName(), "plugins")
}

func (p *PluginBackend) Enable(name string, config *enginetypes.PluginEnableConfig) error {
	return errors.APINotSupportedMsg(ProductName(), "plugins")
}

func (p *PluginBackend) List() ([]enginetypes.Plugin, error) {
	return nil, errors.APINotSupportedMsg(ProductName(), "plugins")
}

func (p *PluginBackend) Inspect(name string) (*enginetypes.Plugin, error) {
	return nil, errors.PluginNotFoundError(name)
}

func (p *PluginBackend) Remove(name string, config *enginetypes.PluginRmConfig) error {
	return errors.APINotSupportedMsg(ProductName(), "plugins")
}

func (p *PluginBackend) Set(name string, args []string) error {
	return errors.APINotSupportedMsg(ProductName(), "plugins")
}

func (p *PluginBackend) Privileges(ctx context.Context, ref reference.Named, metaHeaders http.Header, authConfig *enginetypes.AuthConfig) (enginetypes.PluginPrivileges, error) {
	return nil, errors.APINotSupportedMsg(ProductName(), "plugins")
}

func (p *PluginBackend) Pull(ctx context.Context, ref reference.Named, name string, metaHeaders http.Header, authConfig *enginetypes.AuthConfig, privileges enginetypes.PluginPrivileges, outStream io.Writer) error {
	return errors.APINotSupportedMsg(ProductName(), "plugins")
}

func (p *PluginBackend) Push(ctx context.Context, name string, metaHeaders http.Header, authConfig *enginetypes.AuthConfig, outStream io.Writer) error {
	return errors.APINotSupportedMsg(ProductName(), "plugins")
}

func (p *PluginBackend) CreateFromContext(ctx context.Context, tarCtx io.ReadCloser, options *enginetypes.PluginCreateOptions) error {
	return errors.APINotSupportedMsg(ProductName(), "plugins")
}
