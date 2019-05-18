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
	"strconv"
	"sync"
	"time"

	"github.com/cpuguy83/strongerrors/status/ocstatus"
	pkgerrors "github.com/pkg/errors"
	"github.com/virtual-kubelet/virtual-kubelet/trace"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	v1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"

	"github.com/virtual-kubelet/virtual-kubelet/log"
)

// PodLifecycleHandler defines the interface used by the PodController to react
// to new and changed pods scheduled to the node that is being managed.
type PodLifecycleHandler interface {
	// CreatePod takes a Kubernetes Pod and deploys it within the provider.
	CreatePod(ctx context.Context, pod *corev1.Pod) error

	// UpdatePod takes a Kubernetes Pod and updates it within the provider.
	UpdatePod(ctx context.Context, pod *corev1.Pod) error

	// DeletePod takes a Kubernetes Pod and deletes it from the provider.
	DeletePod(ctx context.Context, pod *corev1.Pod) error

	// GetPod retrieves a pod by name from the provider (can be cached).
	GetPod(ctx context.Context, namespace, name string) (*corev1.Pod, error)

	// GetPodStatus retrieves the status of a pod by name from the provider.
	GetPodStatus(ctx context.Context, namespace, name string) (*corev1.PodStatus, error)

	// GetPods retrieves a list of all pods running on the provider (can be cached).
	GetPods(context.Context) ([]*corev1.Pod, error)
}

// PodNotifier notifies callers of pod changes.
// Providers should implement this interface to enable callers to be notified
// of pod status updates asyncronously.
type PodNotifier interface {
	// NotifyPods instructs the notifier to call the passed in function when
	// the pod status changes.
	//
	// NotifyPods should not block callers.
	NotifyPods(context.Context, func(*corev1.Pod))
}

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
		workqueue:    workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "pods"),
		recorder:     recorder,
		inSyncCh:     make(chan struct{}),
	}

	// Set up event handlers for when Pod resources change.
	pc.podsInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(pod interface{}) {
			if key, err := cache.MetaNamespaceKeyFunc(pod); err != nil {
				log.L.Error(err)
			} else {
				pc.workqueue.AddRateLimited(key)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
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
			if key, err := cache.MetaNamespaceKeyFunc(newPod); err != nil {
				log.L.Error(err)
			} else {
				pc.workqueue.AddRateLimited(key)
			}
		},
		DeleteFunc: func(pod interface{}) {
			if key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(pod); err != nil {
				log.L.Error(err)
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
	defer pc.workqueue.ShutDown()

	// Wait for the caches to be synced before starting workers.
	if ok := cache.WaitForCacheSync(ctx.Done(), pc.podsInformer.Informer().HasSynced); !ok {
		return pkgerrors.New("failed to wait for caches to sync")
	}
	log.G(ctx).Info("Pod cache in-sync")

	close(pc.inSyncCh)

	// Perform a reconciliation step that deletes any dangling pods from the provider.
	// This happens only when the virtual-kubelet is starting, and operates on a "best-effort" basis.
	// If by any reason the provider fails to delete a dangling pod, it will stay in the provider and deletion won't be retried.
	pc.deleteDanglingPods(ctx, threadiness)

	// Launch "threadiness" workers to process Pod resources.
	log.G(ctx).Info("starting workers")
	for id := 0; id < threadiness; id++ {
		go wait.Until(func() {
			// Use the worker's "index" as its ID so we can use it for tracing.
			pc.runWorker(ctx, strconv.Itoa(id))
		}, time.Second, ctx.Done())
	}

	log.G(ctx).Info("started workers")
	<-ctx.Done()
	log.G(ctx).Info("shutting down workers")

	return nil
}

// runWorker is a long-running function that will continually call the processNextWorkItem function in order to read and process an item on the work queue.
func (pc *PodController) runWorker(ctx context.Context, workerId string) {
	for pc.processNextWorkItem(ctx, workerId) {
	}
}

// processNextWorkItem will read a single work item off the work queue and attempt to process it,by calling the syncHandler.
func (pc *PodController) processNextWorkItem(ctx context.Context, workerId string) bool {

	// We create a span only after popping from the queue so that we can get an adequate picture of how long it took to process the item.
	ctx, span := trace.StartSpan(ctx, "processNextWorkItem")
	defer span.End()

	// Add the ID of the current worker as an attribute to the current span.
	ctx = span.WithField(ctx, "workerId", workerId)
	return handleQueueItem(ctx, pc.workqueue, pc.syncHandler)
}

// syncHandler compares the actual state with the desired, and attempts to converge the two.
func (pc *PodController) syncHandler(ctx context.Context, key string) error {
	ctx, span := trace.StartSpan(ctx, "syncHandler")
	defer span.End()

	// Add the current key as an attribute to the current span.
	ctx = span.WithField(ctx, "key", key)

	// Convert the namespace/name string into a distinct namespace and name.
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		// Log the error as a warning, but do not requeue the key as it is invalid.
		log.G(ctx).Warn(pkgerrors.Wrapf(err, "invalid resource key: %q", key))
		return nil
	}

	// Get the Pod resource with this namespace/name.
	pod, err := pc.podsLister.Pods(namespace).Get(name)
	if err != nil {
		if !errors.IsNotFound(err) {
			// We've failed to fetch the pod from the lister, but the error is not a 404.
			// Hence, we add the key back to the work queue so we can retry processing it later.
			err := pkgerrors.Wrapf(err, "failed to fetch pod with key %q from lister", key)
			span.SetStatus(ocstatus.FromError(err))
			return err
		}
		// At this point we know the Pod resource doesn't exist, which most probably means it was deleted.
		// Hence, we must delete it from the provider if it still exists there.
		if err := pc.server.deletePod(ctx, namespace, name); err != nil {
			err := pkgerrors.Wrapf(err, "failed to delete pod %q in the provider", loggablePodNameFromCoordinates(namespace, name))
			span.SetStatus(ocstatus.FromError(err))
			return err
		}
		return nil
	}
	// At this point we know the Pod resource has either been created or updated (which includes being marked for deletion).
	return pc.syncPodInProvider(ctx, pod)
}

// syncPodInProvider tries and reconciles the state of a pod by comparing its Kubernetes representation and the provider's representation.
func (pc *PodController) syncPodInProvider(ctx context.Context, pod *corev1.Pod) error {
	ctx, span := trace.StartSpan(ctx, "syncPodInProvider")
	defer span.End()

	// Add the pod's attributes to the current span.
	ctx = addPodAttributes(ctx, span, pod)

	// Check whether the pod has been marked for deletion.
	// If it does, guarantee it is deleted in the provider and Kubernetes.
	if pod.DeletionTimestamp != nil {
		if err := pc.server.deletePod(ctx, pod.Namespace, pod.Name); err != nil {
			err := pkgerrors.Wrapf(err, "failed to delete pod %q in the provider", loggablePodName(pod))
			span.SetStatus(ocstatus.FromError(err))
			return err
		}
		return nil
	}

	// Ignore the pod if it is in the "Failed" or "Succeeded" state.
	if pod.Status.Phase == corev1.PodFailed || pod.Status.Phase == corev1.PodSucceeded {
		log.G(ctx).Warnf("skipping sync of pod %q in %q phase", loggablePodName(pod), pod.Status.Phase)
		return nil
	}

	// Create or update the pod in the provider.
	if err := pc.server.createOrUpdatePod(ctx, pod, pc.recorder); err != nil {
		err := pkgerrors.Wrapf(err, "failed to sync pod %q in the provider", loggablePodName(pod))
		span.SetStatus(ocstatus.FromError(err))
		return err
	}
	return nil
}

// deleteDanglingPods checks whether the provider knows about any pods which Kubernetes doesn't know about, and deletes them.
func (pc *PodController) deleteDanglingPods(ctx context.Context, threadiness int) {
	ctx, span := trace.StartSpan(ctx, "deleteDanglingPods")
	defer span.End()

	// Grab the list of pods known to the provider.
	pps, err := pc.server.provider.GetPods(ctx)
	if err != nil {
		err := pkgerrors.Wrap(err, "failed to fetch the list of pods from the provider")
		span.SetStatus(ocstatus.FromError(err))
		log.G(ctx).Error(err)
		return
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
			err := pkgerrors.Wrap(err, "failed to fetch pod from the lister")
			span.SetStatus(ocstatus.FromError(err))
			log.G(ctx).Error(err)
			return
		}
	}

	// We delete each pod in its own goroutine, allowing a maximum of "threadiness" concurrent deletions.
	semaphore := make(chan struct{}, threadiness)
	var wg sync.WaitGroup
	wg.Add(len(ptd))

	// Iterate over the slice of pods to be deleted and delete them in the provider.
	for _, pod := range ptd {
		go func(ctx context.Context, pod *corev1.Pod) {
			defer wg.Done()

			ctx, span := trace.StartSpan(ctx, "deleteDanglingPod")
			defer span.End()

			semaphore <- struct{}{}
			defer func() {
				<-semaphore
			}()

			// Add the pod's attributes to the current span.
			ctx = addPodAttributes(ctx, span, pod)
			// Actually delete the pod.
			if err := pc.server.deletePod(ctx, pod.Namespace, pod.Name); err != nil {
				span.SetStatus(ocstatus.FromError(err))
				log.G(ctx).Errorf("failed to delete pod %q in provider", loggablePodName(pod))
			} else {
				log.G(ctx).Infof("deleted leaked pod %q in provider", loggablePodName(pod))
			}
		}(ctx, pod)
	}

	// Wait for all pods to be deleted.
	wg.Wait()
	return
}

// loggablePodName returns the "namespace/name" key for the specified pod.
// If the key cannot be computed, "(unknown)" is returned.
// This method is meant to be used for logging purposes only.
func loggablePodName(pod *corev1.Pod) string {
	k, err := cache.MetaNamespaceKeyFunc(pod)
	if err != nil {
		return "(unknown)"
	}
	return k
}

// loggablePodNameFromCoordinates returns the "namespace/name" key for the pod identified by the specified namespace and name (coordinates).
func loggablePodNameFromCoordinates(namespace, name string) string {
	return fmt.Sprintf("%s/%s", namespace, name)
}
