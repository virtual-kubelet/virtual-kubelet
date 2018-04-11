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
	"time"

	"github.com/virtual-kubelet/virtual-kubelet/providers/vic/proxy"

	"github.com/vmware/vic/lib/apiservers/engine/errors"
	"github.com/vmware/vic/lib/apiservers/portlayer/client"
	"github.com/vmware/vic/pkg/retry"
	"github.com/vmware/vic/pkg/trace"
)

type PodStopper interface {
	Start(op trace.Operation, id, name string) error
}

type VicPodStopper struct {
	client         *client.PortLayer
	isolationProxy proxy.IsolationProxy
}

func NewPodStopper(client *client.PortLayer, isolationProxy proxy.IsolationProxy) *VicPodStopper {
	return &VicPodStopper{
		client:         client,
		isolationProxy: isolationProxy,
	}
}

func (v *VicPodStopper) Stop(op trace.Operation, id, name string) error {
	operation := func() error {
		return v.stop(op, id, name)
	}

	config := retry.NewBackoffConfig()
	config.MaxElapsedTime = 10 * time.Minute
	if err := retry.DoWithConfig(operation, errors.IsConflictError, config); err != nil {
		return err
	}
	return nil
}

func (v *VicPodStopper) stop(op trace.Operation, id, name string) error {
	defer trace.End(trace.Begin(name, op))

	h, err := v.isolationProxy.Handle(op, id, name)
	if err != nil {
		return err
	}

	// Unbind the container to the scope
	h, ep, err := v.isolationProxy.UnbindScope(op, h, name)
	if err != nil {
		return err
	}

	op.Infof("Scope Unbind returned endpoints %# +v", ep)

	h, err = v.isolationProxy.SetState(op, h, name, "STOPPED")
	if err != nil {
		return err
	}

	err = v.isolationProxy.CommitHandle(op, h, id, -1)
	op.Infof("Commit handler returned %v", err)
	return err
}
