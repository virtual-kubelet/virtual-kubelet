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
	"fmt"

	"github.com/virtual-kubelet/virtual-kubelet/providers/vic/cache"
	vicpod "github.com/virtual-kubelet/virtual-kubelet/providers/vic/pod"
	"github.com/virtual-kubelet/virtual-kubelet/providers/vic/proxy"

	"github.com/vmware/vic/lib/apiservers/engine/errors"
	"github.com/vmware/vic/lib/apiservers/portlayer/client"
	"github.com/vmware/vic/pkg/retry"
	"github.com/vmware/vic/pkg/trace"

	"k8s.io/api/core/v1"
)

type PodDeleter interface {
	DeletePod(op trace.Operation, pod *v1.Pod, start bool) error
}

type VicPodDeleter struct {
	client         *client.PortLayer
	imageStore     proxy.ImageStore
	isolationProxy proxy.IsolationProxy
	podCache       cache.PodCache
	personaAddr    string
	portlayerAddr  string
}

type DeleteResponse struct {
	Id       string `json:"Id"`
	Warnings string `json:"Warnings"`
}

func NewPodDeleter(client *client.PortLayer, isolationProxy proxy.IsolationProxy, podCache cache.PodCache, personaAddr string, portlayerAddr string) *VicPodDeleter {
	return &VicPodDeleter{
		client:         client,
		podCache:       podCache,
		personaAddr:    personaAddr,
		portlayerAddr:  portlayerAddr,
		isolationProxy: isolationProxy,
	}
}

func (v *VicPodDeleter) DeletePod(op trace.Operation, pod *v1.Pod) error {
	defer trace.End(trace.Begin(pod.Name, op))

	// Get pod from cache
	vp, err := v.podCache.Get(op, "", pod.Name)

	if err != nil {
		return err
	}

	// Stop pod if not already stopped

	// Transform kube container config to docker create config
	err = v.deletePod(op, vp, true)
	if err != nil {
		op.Errorf("PodDeleter failed to delete pod: %s", err.Error())
		return err
	}

	op.Infof("PodDeleter deleting from cache, name: %s, ID: %s", pod.Name, vp.ID)
	v.podCache.Delete(op, pod.Name)

	return nil
}

//  deletes a pod using the VIC portlayer.
//
//	returns id of pod as a string and error
func (v *VicPodDeleter) deletePod(op trace.Operation, vp *vicpod.VicPod, force bool) error {
	defer trace.End(trace.Begin("", op))

	id := vp.ID
	name := vp.Pod.Name
	running := false

	stopper := NewPodStopper(v.client, v.isolationProxy)
	// Use the force and stop the container first
	if force {
		if err := stopper.Stop(op, id, name); err != nil {
			return err
		}
	} else {
		state, err := v.isolationProxy.State(op, id, name)
		if err != nil {
			return err
		}

		switch state {
		case "Error":
			// force stop if container state is error to make sure container is deletable later
			stopper.Stop(op, id, name)
		case "Starting":
			// if we are starting let the user know they must use the force
			return fmt.Errorf("The container is starting.  To remove use -f")
		case "Running":
			running = true
		}

		handle, err := v.isolationProxy.Handle(op, id, name)
		if err != nil {
			return err
		}

		// Unbind the container to the scope
		_, ep, err := v.isolationProxy.UnbindScope(op, handle, name)
		if err != nil {
			return err
		}
		op.Infof("Scope Unbind returned endpoints %# +v", ep)
	}

	// Retry remove operation if container is not in running state.  If in running state, we only try
	// once to prevent retries from degrading performance.
	if !running {
		operation := func() error {
			return v.isolationProxy.Remove(op, id, true)
		}
		op.Infof("Delete Pod, ID: %s, running: %v", vp.ID, running)
		return retry.Do(operation, errors.IsConflictError)
	}

	err := v.isolationProxy.Remove(op, id, true)
	op.Infof("Delete Pod, ID: %s, running: %v err: %v", vp.ID, running, err)
	return err
}
