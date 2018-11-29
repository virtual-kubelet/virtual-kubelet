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
	"sync"
	"time"

	pkgerrors "github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"

	"github.com/virtual-kubelet/virtual-kubelet/log"
)

const (
	// maxRetries is the number of times we try to process a given key before permanently forgetting it.
	maxRetries = 20
)

// PodController is the controller implementation for Pod resources.
type PodController struct {
	// server is the instance to which this controller belongs.
	server *Server
	// podsInformer is an informer for Pod resources.
	podsInformer v1.PodInformer
	// podsLister is able to list/get Pod resources from a shared informer's store.
	podsLister corev1listers.PodLister

	// workqueue is a rate limited work queue.
	// This is used to queue work to be processed instead of performing it as soon as a change happens.
	// This means we can ensure we only process a fixed amount of resources at a time, and makes it easy to ensure we are never processing the same item simultaneously in two different workers.
	workqueue workqueue.RateLimitingInterface
	// recorder is an event recorder for recording Event resources to the Kubernetes API.
	recorder record.EventRecorder
}

// NewPodController returns a new instance of PodController.
func NewPodController(server *Server) *PodController {
	// Create an event broadcaster.
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(log.L.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: server.k8sClient.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: "pod-controller"})

	// Create an instance of PodController having a work queue that uses the rate limiter created above.
	pc := &PodController{
		server:       server,
		podsInformer: server.podInformer,
		podsLister:   server.podInformer.Lister(),
		workqueue:    workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "pods"),
		recorder:     recorder,
	}

	// Set up event handlers for when Pod resources change.
	pc.podsInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(pod interface{}) {
			if key, err := cache.MetaNamespaceKeyFunc(pod); err != nil {
				runtime.HandleError(err)
			} else {
				pc.workqueue.AddRateLimited(key)
			}
		},
		UpdateFunc: func(_, pod interface{}) {
			if key, err := cache.MetaNamespaceKeyFunc(pod); err != nil {
				runtime.HandleError(err)
			} else {
				pc.workqueue.AddRateLimited(key)
			}
		},
		DeleteFunc: func(pod interface{}) {
			if key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(pod); err != nil {
				runtime.HandleError(err)
			} else {
				pc.workqueue.AddRateLimited(key)
			}
		},
	})

	// Return the instance of PodController back to the caller.
	return pc
}

// Run will set up the event handlers for types we are interested in, as well as syncing informer caches and starting workers.
// It will block until stopCh is closed, at which point it will shutdown the work queue and wait for workers to finish processing their current work items.
func (pc *PodController) Run(ctx context.Context, threadiness int) error {
	defer runtime.HandleCrash()
	defer pc.workqueue.ShutDown()

	// Wait for the caches to be synced before starting workers.
	if ok := cache.WaitForCacheSync(ctx.Done(), pc.podsInformer.Informer().HasSynced); !ok {
		return pkgerrors.New("failed to wait for caches to sync")
	}

	// Perform a reconciliation step that deletes any dangling pods from the provider.
	// This happens only when the virtual-kubelet is starting, and operates on a "best-effort" basis.
	// If by any reason the provider fails to delete a dangling pod, it will stay in the provider and deletion won't be retried.
	pc.deleteDanglingPods(ctx)

	// Launch two workers to process Pod resources.
	log.G(ctx).Info("starting workers")
	for i := 0; i < threadiness; i++ {
		go wait.Until(pc.runWorker, time.Second, ctx.Done())
	}

	log.G(ctx).Info("started workers")
	<-ctx.Done()
	log.G(ctx).Info("shutting down workers")

	return nil
}

// runWorker is a long-running function that will continually call the processNextWorkItem function in order to read and process an item on the work queue.
func (pc *PodController) runWorker() {
	for pc.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the work queue and attempt to process it,by calling the syncHandler.
func (pc *PodController) processNextWorkItem() bool {
	obj, shutdown := pc.workqueue.Get()

	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer pc.workqueue.Done.
	err := func(obj interface{}) error {
		// We call Done here so the work queue knows we have finished processing this item.
		// We also must remember to call Forget if we do not want this work item being re-queued.
		// For example, we do not call Forget if a transient error occurs.
		// Instead, the item is put back on the work queue and attempted again after a back-off period.
		defer pc.workqueue.Done(obj)
		var key string
		var ok bool
		// We expect strings to come off the work queue.
		// These are of the form namespace/name.
		// We do this as the delayed nature of the work queue means the items in the informer cache may actually be more up to date that when the item was initially put onto the workqueue.
		if key, ok = obj.(string); !ok {
			// As the item in the work queue is actually invalid, we call Forget here else we'd go into a loop of attempting to process a work item that is invalid.
			pc.workqueue.Forget(obj)
			runtime.HandleError(pkgerrors.Errorf("expected string in work queue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the Pod resource to be synced.
		if err := pc.syncHandler(key); err != nil {
			if pc.workqueue.NumRequeues(key) < maxRetries {
				// Put the item back on the work queue to handle any transient errors.
				log.L.Errorf("requeuing %q due to failed sync", key)
				pc.workqueue.AddRateLimited(key)
				return nil
			}
			// We've exceeded the maximum retries, so we must forget the key.
			pc.workqueue.Forget(key)
			return pkgerrors.Wrapf(err, "forgetting %q due to maximum retries reached", key)
		}
		// Finally, if no error occurs we Forget this item so it does not get queued again until another change happens.
		pc.workqueue.Forget(obj)
		return nil
	}(obj)

	if err != nil {
		runtime.HandleError(err)
		return true
	}

	return true
}

// syncHandler compares the actual state with the desired, and attempts to converge the two.
func (pc *PodController) syncHandler(key string) error {
	// Convert the namespace/name string into a distinct namespace and name.
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		// Log the error but do not requeue the key as it is invalid.
		runtime.HandleError(pkgerrors.Wrapf(err, "invalid resource key: %q", key))
		return nil
	}

	// Create a context to pass to the provider.
	ctx := context.Background()

	// Get the Pod resource with this namespace/name.
	pod, err := pc.podsLister.Pods(namespace).Get(name)
	if err != nil {
		if !errors.IsNotFound(err) {
			// We've failed to fetch the pod from the lister, but the error is not a 404.
			// Hence, we add the key back to the work queue so we can retry processing it later.
			return pkgerrors.Wrapf(err, "failed to fetch pod with key %q from lister", key)
		}
		// At this point we know the Pod resource doesn't exist, which most probably means it was deleted.
		// Hence, we must delete it from the provider if it still exists there.
		return pc.deletePodInProvider(ctx, namespace, name)
	}
	// At this point we know the Pod resource has either been created or updated (which includes being marked for deletion).
	return pc.syncPodInProvider(ctx, pod)
}

// syncPodInProvider tries and reconciles the state of a pod by comparing its Kubernetes representation and the provider's representation.
func (pc *PodController) syncPodInProvider(ctx context.Context, pod *corev1.Pod) error {
	// Reconstruct the pod's key.
	key := metaKey(pod)

	// Check whether the pod has been marked for deletion.
	// If it does, delete it in the provider.
	if pod.DeletionTimestamp != nil {
		// Delete the pod.
		if err := pc.server.deletePod(ctx, pod); err != nil {
			return pkgerrors.Wrapf(err, "failed to delete pod %q in the provider", key)
		}
		return nil
	}

	// Ignore the pod if it is in the "Failed" state.
	if pod.Status.Phase == corev1.PodFailed {
		log.G(ctx).Warnf("skipping sync of pod %q in %q phase", key, pod.Status.Phase)
	}

	// Check if the pod is already known by the provider.
	// NOTE: Some providers return a non-nil error in their GetPod implementation when the pod is not found while some other don't.
	// Hence, we ignore the error and just act upon the pod if it is non-nil (meaning that the provider still knows about the pod).
	pp, _ := pc.server.provider.GetPod(ctx, pod.Namespace, pod.Name)
	if pp != nil {
		// The pod has already been created in the provider.
		// Hence, we return since pod updates are not yet supported.
		return nil
	}
	// Create the pod in the provider.
	if err := pc.server.createPod(ctx, pod); err != nil {
		return pkgerrors.Wrapf(err, "failed to create pod %q in the provider", key)
	}
	return nil
}

// deletePodInProvider checks whether the pod with the specified namespace and name is still known to the provider, and deletes it in case it is.
// This function is meant to be called only when a given Pod resource has already been deleted from Kubernetes.
func (pc *PodController) deletePodInProvider(ctx context.Context, namespace, name string) error {
	// Reconstruct the pod's key.
	key := metaKeyFromNamespaceName(namespace, name)

	// Grab the pod as known by the provider.
	// Since this function is only called when the Pod resource has already been deleted from Kubernetes, we must get it from the provider so we can call "deletePod".
	// NOTE: Some providers return a non-nil error in their GetPod implementation when the pod is not found while some other don't.
	// Hence, we ignore the error and just act upon the pod if it is non-nil (meaning that the provider still knows about the pod).
	pod, _ := pc.server.provider.GetPod(ctx, namespace, name)
	if pod == nil {
		// The provider is not aware of the pod, so we just exit.
		return nil
	}

	// Delete the pod.
	if err := pc.server.deletePod(ctx, pod); err != nil {
		return pkgerrors.Wrapf(err, "failed to delete pod %q in the provider", key)
	}
	return nil
}

// deleteDanglingPods checks whether the provider knows about any pods which Kubernetes doesn't know about, and deletes them.
func (pc *PodController) deleteDanglingPods(ctx context.Context) error {
	// Grab the list of pods known to the provider.
	pps, err := pc.server.provider.GetPods(ctx)
	if err != nil {
		return pkgerrors.Wrap(err, "failed to fetch the list of pods from the provider")
	}

	// Create a slice to hold the pods we will be deleting from the provider.
	ptd := make([]*corev1.Pod, 0)

	// Iterate over the pods known to the provider, marking for deletion those that don't exist in Kubernetes.
	// Take on this opportunity to populate the list of key that correspond to pods known to the provider.
	for _, pp := range pps {
		if _, err := pc.podsLister.Pods(pp.Namespace).Get(pp.Name); err != nil {
			if errors.IsNotFound(err) {
				// The current pod does not exist in Kubernetes, so we mark it for deletion.
				ptd = append(ptd, pp)
				continue
			}
			// For some reason we couldn't fetch the pod from the lister, so we propagate the error.
			return pkgerrors.Wrap(err, "failed to fetch pod from the lister")
		}
	}

	var wg sync.WaitGroup
	wg.Add(len(ptd))

	// Iterate over the slice of pods to be deleted and delete them in the provider.
	for _, pod := range ptd {
		go func(pod *corev1.Pod) {
			if err := pc.server.deletePod(ctx, pod); err != nil {
				log.G(ctx).Errorf("failed to delete pod %q in provider", metaKey(pod))
			} else {
				log.G(ctx).Infof("deleted leaked pod %q in provider", metaKey(pod))
			}
			wg.Done()
		}(pod)
	}

	// Wait for all pods to be deleted.
	wg.Wait()
	return nil
}

// metaKey returns the "namespace/name" key for the specified pod.
// If the key cannot be computed, "(unknown)" is returned.
func metaKey(pod *corev1.Pod) string {
	k, err := cache.MetaNamespaceKeyFunc(pod)
	if err != nil {
		return "(unknown)"
	}
	return k
}

// metaKeyFromNamespaceName returns the "namespace/name" key for the pod identified by the specified namespace and name.
func metaKeyFromNamespaceName(namespace, name string) string {
	return fmt.Sprintf("%s/%s", namespace, name)
}
