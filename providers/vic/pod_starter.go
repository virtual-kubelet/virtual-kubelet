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

package vic

import (
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

func NewPodStarter(client *client.PortLayer, isolationProxy proxy.IsolationProxy) *VicPodStarter {
	return &VicPodStarter{
		client:         client,
		isolationProxy: isolationProxy,
	}
}

func (v *VicPodStarter) Start(op trace.Operation, id, name string) error {
	defer trace.End(trace.Begin(name, op))

	h, err := v.isolationProxy.Handle(op, id, name)
	if err != nil {
		return err
	}

	// Bind the container to the scope
	h, ep, err := v.isolationProxy.BindScope(op, h, name)
	if err != nil {
		return err
	}

	op.Infof("*** Scope bind returned endpoints %#v", ep)

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
