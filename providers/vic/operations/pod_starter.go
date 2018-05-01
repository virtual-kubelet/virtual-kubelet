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

package operations

import (
	"context"
	"fmt"

	"github.com/virtual-kubelet/virtual-kubelet/providers/vic/proxy"

	"github.com/vmware/vic/lib/apiservers/portlayer/client"
	"github.com/vmware/vic/pkg/trace"
)

type PodStarter interface {
	Start(op trace.Operation, id, name string) error
}

type VicPodStarter struct {
	client         *client.PortLayer
	isolationProxy proxy.IsolationProxy
	imageStore     proxy.ImageStore
}

type VicPodStarterError string

func (e VicPodStarterError) Error() string { return string(e) }

const (
	PodStarterPortlayerClientError = VicPodStarterError("PodStarter called with an invalid portlayer client")
	PodStarterIsolationProxyError  = VicPodStarterError("PodStarter called with an invalid isolation proxy")
	PodStarterInvalidPodIDError    = VicPodStarterError("PodStarter called with invalid Pod ID")
	PodStarterInvalidPodNameError  = VicPodStarterError("PodStarter called with invalid Pod name")
)

func NewPodStarter(client *client.PortLayer, isolationProxy proxy.IsolationProxy) (*VicPodStarter, error) {
	defer trace.End(trace.Begin("", context.Background()))

	if client == nil {
		return nil, PodStarterPortlayerClientError
	}
	if isolationProxy == nil {
		return nil, PodStarterIsolationProxyError
	}

	return &VicPodStarter{
		client:         client,
		isolationProxy: isolationProxy,
	}, nil
}

// Start starts up the pod vm
//
// arguments:
//		op		operation trace logger
//		id		pod id
//		name	pod name
// returns:
//		error
func (v *VicPodStarter) Start(op trace.Operation, id, name string) error {
	defer trace.End(trace.Begin(fmt.Sprintf("id(%s), name(%s)", id, name), op))

	if id == "" {
		op.Errorf(PodStarterInvalidPodIDError.Error())
		return PodStarterInvalidPodIDError
	}
	if name == "" {
		op.Errorf(PodStarterInvalidPodNameError.Error())
		return PodStarterInvalidPodNameError
	}

	h, err := v.isolationProxy.Handle(op, id, name)
	if err != nil {
		return err
	}

	// Bind the container to the scope
	h, ep, err := v.isolationProxy.BindScope(op, h, name)
	if err != nil {
		return err
	}

	op.Debugf("*** Scope bind returned endpoints %#v", ep)

	defer func() {
		if err != nil {
			op.Debugf("Unbinding %s due to error - %s", id, err.Error())
			v.isolationProxy.UnbindScope(op, h, name)
		}
	}()

	h, err = v.isolationProxy.SetState(op, h, name, "RUNNING")
	if err != nil {
		return err
	}

	// map ports

	err = v.isolationProxy.CommitHandle(op, h, id, -1)

	return nil
}
