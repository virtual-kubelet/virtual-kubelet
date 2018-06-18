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

package proxy

//****
// system_proxy.go
//
// Contains all code that touches the portlayer for system operations and all
// code that converts swagger based returns to docker personality backend structs.
// The goal is to make the backend code that implements the docker engine-api
// interfaces be as simple as possible and contain no swagger or portlayer code.
//
// Rule for code to be in here:
// 1. touches VIC portlayer
// 2. converts swagger to docker engine-api structs
// 3. errors MUST be docker engine-api compatible errors.  DO NOT return arbitrary errors!
//		- Do NOT return portlayer errors
//		- Do NOT return fmt.Errorf()
//		- Do NOT return errors.New()
//		- DO USE the aliased docker error package 'derr'

import (
	"fmt"
	"net/http"

	"golang.org/x/net/context"

	log "github.com/Sirupsen/logrus"
	derr "github.com/docker/docker/api/errors"

	"github.com/vmware/vic/lib/apiservers/engine/errors"
	"github.com/vmware/vic/lib/apiservers/portlayer/client"
	"github.com/vmware/vic/lib/apiservers/portlayer/client/containers"
	"github.com/vmware/vic/lib/apiservers/portlayer/client/misc"
	"github.com/vmware/vic/lib/apiservers/portlayer/models"
	"github.com/vmware/vic/pkg/trace"
)

type VicSystemProxy interface {
	PingPortlayer(ctx context.Context) bool
	ContainerCount(ctx context.Context) (int, int, int, error)
	VCHInfo(ctx context.Context) (*models.VCHInfo, error)
}

type SystemProxy struct {
	client *client.PortLayer
}

func NewSystemProxy(client *client.PortLayer) VicSystemProxy {
	if client == nil {
		return nil
	}

	return &SystemProxy{client: client}
}

func (s *SystemProxy) PingPortlayer(ctx context.Context) bool {
	defer trace.End(trace.Begin(""))

	if s.client == nil {
		log.Errorf("Portlayer client is invalid")
		return false
	}

	pingParams := misc.NewPingParamsWithContext(ctx)
	_, err := s.client.Misc.Ping(pingParams)
	if err != nil {
		log.Info("Ping to portlayer failed")
		return false
	}
	return true
}

// Use the Portlayer's support for docker ps to get the container count
//   return order: running, paused, stopped counts
func (s *SystemProxy) ContainerCount(ctx context.Context) (int, int, int, error) {
	defer trace.End(trace.Begin(""))

	var running, paused, stopped int

	if s.client == nil {
		return 0, 0, 0, errors.NillPortlayerClientError("SystemProxy")
	}

	all := true
	containList, err := s.client.Containers.GetContainerList(containers.NewGetContainerListParamsWithContext(ctx).WithAll(&all))
	if err != nil {
		return 0, 0, 0, derr.NewErrorWithStatusCode(fmt.Errorf("Failed to get container list: %s", err), http.StatusInternalServerError)
	}

	for _, t := range containList.Payload {
		st := t.ContainerConfig.State
		if st == "Running" {
			running++
		} else if st == "Stopped" || st == "Created" {
			stopped++
		}
	}

	return running, paused, stopped, nil
}

func (s *SystemProxy) VCHInfo(ctx context.Context) (*models.VCHInfo, error) {
	defer trace.End(trace.Begin(""))

	if s.client == nil {
		return nil, errors.NillPortlayerClientError("SystemProxy")
	}

	params := misc.NewGetVCHInfoParamsWithContext(ctx)
	resp, err := s.client.Misc.GetVCHInfo(params)
	if err != nil {
		//There are no custom error for this operation.  If we get back an error, it's
		//unknown.
		return nil, derr.NewErrorWithStatusCode(fmt.Errorf("Unknown error from port layer: %s", err),
			http.StatusInternalServerError)
	}

	return resp.Payload, nil
}
