// Copyright © 2017 The virtual-kubelet authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package vkubelet

import (
	"context"
	"fmt"
	"reflect"

	pkgerrors "github.com/pkg/errors"
	"github.com/virtual-kubelet/virtual-kubelet/log"
	"github.com/virtual-kubelet/virtual-kubelet/trace"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
)

// PodController is the controller implementation for Pod resources.
type PodController struct {
	// server is the instance to which this controller belongs.
	server *Server

	// podsInformer is an informer for Pod resources.
	podsInformer v1.PodInformer

	// podsLister is able to list/get Pod resources from a shared informer's store.
	podsLister corev1listers.PodLister

	// recorder is an event recorder for recording Event resources to the Kubernetes API.
	recorder record.EventRecorder

	// inSync is a channel which will be closed once the pod controller has become in-sync with apiserver
	// it will never close if startup fails, or if the run context is cancelled prior to initialization completing
	inSyncCh chan struct{}
}

// NewPodController returns a new instance of PodController.
func NewPodController(server *Server) *PodController {
	// Create an event broadcaster.
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(log.L.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: server.k8sClient.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: fmt.Sprintf("%s/pod-controller", server.nodeName)})

	// Create an instance of PodController having a work queue that uses the rate limiter created above.
	pc := &PodController{
		server:       server,
		podsInformer: server.podInformer,
		podsLister:   server.podInformer.Lister(),
		recorder:     recorder,
		inSyncCh:     make(chan struct{}),
	}

	// Return the instance of PodController back to the caller.
	return pc
}

// Run will set up the event handlers for types we are interested in, as well as syncing informer caches and starting workers.
// It will block until stopCh is closed, at which point it will shutdown the work queue and wait for workers to finish processing their current work items.
func (pc *PodController) Run(ctx context.Context) error {
	// Set up event handlers for when Pod resources change.
	pc.podsInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(pod interface{}) {
			pc.addFunc(ctx, pod)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			pc.updateFunc(ctx, oldObj, newObj)
		},
		DeleteFunc: func(pod interface{}) {
			pc.deleteFunc(ctx, pod)
		},
	})

	// Wait for the caches to be synced before starting workers.
	if ok := cache.WaitForCacheSync(ctx.Done(), pc.podsInformer.Informer().HasSynced); !ok {
		return pkgerrors.New("failed to wait for caches to sync")
	}
	log.G(ctx).Info("Pod cache in-sync")

	close(pc.inSyncCh)

	log.G(ctx).Info("Pod Controller synced")
	<-ctx.Done()

	return nil
}

func (pc *PodController) addFunc(ctx context.Context, pod interface{}) {
	ctx, span := trace.StartSpan(ctx, "addFunc")
	defer span.End()

	key, err := cache.MetaNamespaceKeyFunc(pod)
	if err != nil {
		log.G(ctx).WithError(err).Error("Unable to calculate key")
		return
	}

	pc.server.lock.Lock()
	defer pc.server.lock.Unlock()
	if psm, ok := pc.server.podStateMachines[key]; ok {
		psm.updatePodControllerPod(ctx, pod.(*corev1.Pod))
	} else {
		psm := newPodStateMachineFromAPIServer(ctx, pod.(*corev1.Pod), pc, key)
		pc.server.podStateMachines[key] = psm
		go psm.run(ctx)
	}
}

func (pc *PodController) updateFunc(ctx context.Context, oldObj, newObj interface{}) {
	ctx, span := trace.StartSpan(ctx, "updateFunc")
	defer span.End()

	// Create a copy of the old and new pod objects so we don't mutate the cache.
	oldPod := oldObj.(*corev1.Pod).DeepCopy()
	newPod := newObj.(*corev1.Pod).DeepCopy()
	// We want to check if the two objects differ in anything other than their resource versions.
	// Hence, we make them equal so that this change isn't picked up by reflect.DeepEqual.
	newPod.ResourceVersion = oldPod.ResourceVersion
	// Skip adding this pod's key to the work queue if its .metadata (except .metadata.resourceVersion) and .spec fields haven't changed.
	// This guarantees that we don't attempt to sync the pod every time its .status field is updated.
	if reflect.DeepEqual(oldPod.ObjectMeta, newPod.ObjectMeta) && reflect.DeepEqual(oldPod.Spec, newPod.Spec) {
		return
	}
	// At this point we know that something in .metadata or .spec has changed, so we must proceed to sync the pod.
	pc.server.lock.Lock()
	defer pc.server.lock.Unlock()
	key, err := cache.MetaNamespaceKeyFunc(newObj)
	if err != nil {
		log.G(ctx).WithError(err).Error("Unable to calculate key")
		return
	}

	pod := newObj.(*corev1.Pod)

	if psm, ok := pc.server.podStateMachines[key]; ok {
		psm.updatePodControllerPod(ctx, pod)
	} else {
		log.G(ctx).Error("Pod not found in pod state machine list on update")
	}
}

func (pc *PodController) deleteFunc(ctx context.Context, pod interface{}) {
	ctx, span := trace.StartSpan(ctx, "updateFunc")
	defer span.End()

	key, err := cache.MetaNamespaceKeyFunc(pod)
	if err != nil {
		log.G(ctx).WithError(err).Error("Unable to calculate key")
		return
	}

	pc.server.lock.Lock()
	defer pc.server.lock.Unlock()
	if psm, ok := pc.server.podStateMachines[key]; ok {
		psm.updatePodControllerPod(ctx, pod.(*corev1.Pod))
	} else {
		// TODO: Figure out whether we want to do this or not
		log.G(ctx).Error("Pod not found in pod state machine list on delete")
	}
}
