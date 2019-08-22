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

package node

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"sync"
	"time"

	pkgerrors "github.com/pkg/errors"
	"github.com/virtual-kubelet/virtual-kubelet/errdefs"
	"github.com/virtual-kubelet/virtual-kubelet/internal/manager"
	"github.com/virtual-kubelet/virtual-kubelet/log"
	"github.com/virtual-kubelet/virtual-kubelet/trace"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	corev1informers "k8s.io/client-go/informers/core/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
)

var (
	errPodNotFoundInCache = pkgerrors.New("Pod not found in cache, nor stored in pod for deletion. Pod must have been deleted while processing, retrying")
)

// PodLifecycleHandler defines the interface used by the PodController to react
// to new and changed pods scheduled to the node that is being managed.
//
// Errors produced by these methods should implement an interface from
// github.com/virtual-kubelet/virtual-kubelet/errdefs package in order for the
// core logic to be able to understand the type of failure.
type PodLifecycleHandler interface {
	// CreatePod takes a Kubernetes Pod and deploys it within the provider.
	CreatePod(ctx context.Context, pod *corev1.Pod) error

	// UpdatePod takes a Kubernetes Pod and updates it within the provider.
	UpdatePod(ctx context.Context, pod *corev1.Pod) error

	// DeletePod takes a Kubernetes Pod and deletes it from the provider.
	DeletePod(ctx context.Context, pod *corev1.Pod) error

	// GetPod retrieves a pod by name from the provider (can be cached).
	// The Pod returned is expected to be immutable, and may be accessed
	// concurrently outside of the calling goroutine. Therefore it is recommended
	// to return a version after DeepCopy.
	GetPod(ctx context.Context, namespace, name string) (*corev1.Pod, error)

	// GetPodStatus retrieves the status of a pod by name from the provider.
	// The PodStatus returned is expected to be immutable, and may be accessed
	// concurrently outside of the calling goroutine. Therefore it is recommended
	// to return a version after DeepCopy.
	GetPodStatus(ctx context.Context, namespace, name string) (*corev1.PodStatus, error)

	// GetPods retrieves a list of all pods running on the provider (can be cached).
	// The Pods returned are expected to be immutable, and may be accessed
	// concurrently outside of the calling goroutine. Therefore it is recommended
	// to return a version after DeepCopy.
	GetPods(context.Context) ([]*corev1.Pod, error)
}

// PodNotifier notifies callers of pod changes.
// Providers should implement this interface to enable callers to be notified
// of pod status updates asynchronously.
type PodNotifier interface {
	// NotifyPods instructs the notifier to call the passed in function when
	// the pod status changes. It should be called when a pod's status changes.
	//
	// The provided pointer to a Pod is guaranteed to be used in a read-only
	// fashion. The provided pod's PodStatus should be up to date when
	// this function is called.
	//
	// NotifyPods will not block callers.
	NotifyPods(context.Context, func(*corev1.Pod))
}

// PodController is the controller implementation for Pod resources.
type PodController struct {
	provider PodLifecycleHandler

	// podsInformer is an informer for Pod resources.
	podsInformer corev1informers.PodInformer
	// podsLister is able to list/get Pod resources from a shared informer's store.
	podsLister corev1listers.PodLister

	// recorder is an event recorder for recording Event resources to the Kubernetes API.
	recorder record.EventRecorder

	// ready is a channel which will be closed once the pod controller is fully up and running.
	// this channel will never be closed if there is an error on startup.
	ready chan struct{}

	client corev1client.PodsGetter

	resourceManager *manager.ResourceManager

	k8sQ workqueue.RateLimitingInterface

	// From the time of creation, to termination the knownPods map will contain the pods key
	// (derived from K8's cache library) -> a *knownPod struct. We prevent it from leaking by
	// only allowing it to stay around during the course of the pod's lifetime in the informer.
	knownPods sync.Map

	informerPodUpdatesLock sync.Mutex
	informerPodUpdates     map[string]*informerPodUpdate
}

type knownPod struct {
	// You cannot read (or modify) the fields in this struct without taking the lock. The individual fields
	// should be immutable to avoid having to hold the lock the entire time you're working with them
	sync.Mutex
	lastPodStatusReceivedFromProvider *corev1.Pod
}

type informerPodUpdate struct {
	oldPod *corev1.Pod
	newPod *corev1.Pod
}

// PodControllerConfig is used to configure a new PodController.
type PodControllerConfig struct {
	// PodClient is used to perform actions on the k8s API, such as updating pod status
	// This field is required
	PodClient corev1client.PodsGetter

	// PodInformer is used as a local cache for pods
	// This should be configured to only look at pods scheduled to the node which the controller will be managing
	PodInformer corev1informers.PodInformer

	EventRecorder record.EventRecorder

	Provider PodLifecycleHandler

	// Informers used for filling details for things like downward API in pod spec.
	//
	// We are using informers here instead of listeners because we'll need the
	// informer for certain features (like notifications for updated ConfigMaps)
	ConfigMapInformer corev1informers.ConfigMapInformer
	SecretInformer    corev1informers.SecretInformer
	ServiceInformer   corev1informers.ServiceInformer
}

func NewPodController(cfg PodControllerConfig) (*PodController, error) {
	if cfg.PodClient == nil {
		return nil, errdefs.InvalidInput("missing core client")
	}
	if cfg.EventRecorder == nil {
		return nil, errdefs.InvalidInput("missing event recorder")
	}
	if cfg.PodInformer == nil {
		return nil, errdefs.InvalidInput("missing pod informer")
	}
	if cfg.ConfigMapInformer == nil {
		return nil, errdefs.InvalidInput("missing config map informer")
	}
	if cfg.SecretInformer == nil {
		return nil, errdefs.InvalidInput("missing secret informer")
	}
	if cfg.ServiceInformer == nil {
		return nil, errdefs.InvalidInput("missing service informer")
	}
	if cfg.Provider == nil {
		return nil, errdefs.InvalidInput("missing provider")
	}

	rm, err := manager.NewResourceManager(cfg.PodInformer.Lister(), cfg.SecretInformer.Lister(), cfg.ConfigMapInformer.Lister(), cfg.ServiceInformer.Lister())
	if err != nil {
		return nil, pkgerrors.Wrap(err, "could not create resource manager")
	}

	pc := &PodController{
		provider:           cfg.Provider,
		podsInformer:       cfg.PodInformer,
		podsLister:         cfg.PodInformer.Lister(),
		recorder:           cfg.EventRecorder,
		ready:              make(chan struct{}),
		client:             cfg.PodClient,
		resourceManager:    rm,
		k8sQ:               workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "syncPodsFromKubernetes"),
		informerPodUpdates: make(map[string]*informerPodUpdate),
	}
	// Set up event handlers for when Pod resources change.
	pc.podsInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(pod interface{}) {
			if key, err := cache.MetaNamespaceKeyFunc(pod); err != nil {
				log.L.Error(err)
			} else {
				pc.knownPods.Store(key, &knownPod{})
				pc.informerPodUpdatesLock.Lock()
				defer pc.informerPodUpdatesLock.Unlock()
				pc.informerPodUpdates[key] = &informerPodUpdate{
					newPod: pod.(*corev1.Pod),
				}
				pc.k8sQ.AddRateLimited(key)
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
				pc.informerPodUpdatesLock.Lock()
				defer pc.informerPodUpdatesLock.Unlock()
				pc.informerPodUpdates[key] = &informerPodUpdate{
					newPod: newPod,
					oldPod: oldPod,
				}
				pc.k8sQ.AddRateLimited(key)
			}
		},
		DeleteFunc: func(pod interface{}) {
			if key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(pod); err != nil {
				log.L.Error(err)
			} else {
				pc.informerPodUpdatesLock.Lock()
				defer pc.informerPodUpdatesLock.Unlock()
				pc.informerPodUpdates[key] = &informerPodUpdate{
					oldPod: pod.(*corev1.Pod),
				}
				pc.knownPods.Delete(key)
				pc.k8sQ.AddRateLimited(key)
			}
		},
	})

	return pc, nil
}

// Run will set up the event handlers for types we are interested in, as well as syncing informer caches and starting workers.
// It will block until the context is cancelled, at which point it will shutdown the work queue and wait for workers to finish processing their current work items.
func (pc *PodController) Run(ctx context.Context, podSyncWorkers int) error {
	defer pc.k8sQ.ShutDown()

	podStatusQueue := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "syncPodStatusFromProvider")
	pc.runSyncFromProvider(ctx, podStatusQueue)
	pc.runProviderSyncWorkers(ctx, podStatusQueue, podSyncWorkers)
	defer podStatusQueue.ShutDown()

	// Wait for the caches to be synced *before* starting workers.
	if ok := cache.WaitForCacheSync(ctx.Done(), pc.podsInformer.Informer().HasSynced); !ok {
		return pkgerrors.New("failed to wait for caches to sync")
	}
	log.G(ctx).Info("Pod cache in-sync")

	// Perform a reconciliation step that deletes any dangling pods from the provider.
	// This happens only when the virtual-kubelet is starting, and operates on a "best-effort" basis.
	// If by any reason the provider fails to delete a dangling pod, it will stay in the provider and deletion won't be retried.
	pc.deleteDanglingPods(ctx, podSyncWorkers)

	log.G(ctx).Info("starting workers")
	for id := 0; id < podSyncWorkers; id++ {
		workerID := strconv.Itoa(id)
		go wait.Until(func() {
			// Use the worker's "index" as its ID so we can use it for tracing.
			pc.runWorker(ctx, workerID, pc.k8sQ)
		}, time.Second, ctx.Done())
	}

	close(pc.ready)

	log.G(ctx).Info("started workers")
	<-ctx.Done()
	log.G(ctx).Info("shutting down workers")

	return nil
}

// Ready returns a channel which gets closed once the PodController is ready to handle scheduled pods.
// This channel will never close if there is an error on startup.
// The status of this channel after shutdown is indeterminate.
func (pc *PodController) Ready() <-chan struct{} {
	return pc.ready
}

// runWorker is a long-running function that will continually call the processNextWorkItem function in order to read and process an item on the work queue.
func (pc *PodController) runWorker(ctx context.Context, workerId string, q workqueue.RateLimitingInterface) {
	for pc.processNextWorkItem(ctx, workerId, q) {
	}
}

// processNextWorkItem will read a single work item off the work queue and attempt to process it,by calling the syncHandler.
func (pc *PodController) processNextWorkItem(ctx context.Context, workerId string, q workqueue.RateLimitingInterface) bool {

	// We create a span only after popping from the queue so that we can get an adequate picture of how long it took to process the item.
	ctx, span := trace.StartSpan(ctx, "processNextWorkItem")
	defer span.End()

	// Add the ID of the current worker as an attribute to the current span.
	ctx = span.WithField(ctx, "workerId", workerId)
	return handleQueueItem(ctx, q, pc.syncHandler)
}

// syncHandler compares the actual state with the desired, and attempts to converge the two.
func (pc *PodController) syncHandler(ctx context.Context, key string, willRetry bool) error {
	ctx, span := trace.StartSpan(ctx, "syncHandler")
	defer span.End()

	// Add the current key as an attribute to the current span.
	ctx = span.WithField(ctx, "key", key)

	pc.informerPodUpdatesLock.Lock()
	podUpdate, ok := pc.informerPodUpdates[key]
	delete(pc.informerPodUpdates, key)
	pc.informerPodUpdatesLock.Unlock()

	if !ok {
		log.G(ctx).Warn("Pod update not found?")
		return nil
	}

	err := pc.syncInformerPodUpdateToProvider(ctx, key, podUpdate)
	if err != nil {
		span.SetStatus(err)
		if willRetry {
			pc.informerPodUpdatesLock.Lock()
			defer pc.informerPodUpdatesLock.Unlock()
			_, ok = pc.informerPodUpdates[key]
			// If it's ok, then that means there was a new pod status update added by the informer reactor.
			if !ok {
				pc.informerPodUpdates[key] = podUpdate
			}
		}
	}
	return err
}

func shouldIgnorePod(ctx context.Context, pod *corev1.Pod) bool {
	// Ignore the pod if it is in the "Failed" or "Succeeded" state.
	if pod.Status.Phase == corev1.PodFailed || pod.Status.Phase == corev1.PodSucceeded {
		log.G(ctx).Warnf("skipping sync of pod %q in %q phase", loggablePodName(pod), pod.Status.Phase)
		return true
	}
	return false
}

func (pc *PodController) syncInformerPodUpdateToProvider(ctx context.Context, key string, update *informerPodUpdate) error {
	ctx, span := trace.StartSpan(ctx, "syncInformerPodUpdateToProvider")
	defer span.End()

	if update.newPod == nil {
		ctx = addPodAttributes(ctx, span, update.oldPod)
		// Pod is destroyed.

		// if the old pod was deletable, we already tried to deleted it
		// the one exception here is if we hit the retry limit last time we tried to delete it, and bailed out.
		// in that case, the pod can leak.
		//
		// We avoid this by interrogating the provider, to see if it still has the pod. If it returns a notfound error,
		// or a nil pod and no error, we consider it to be okay.
		if shouldIgnorePod(ctx, update.oldPod) {
			return nil
		}

		if deletable(update.oldPod) {
			if podStatus, err := pc.provider.GetPodStatus(ctx, update.oldPod.Namespace, update.oldPod.Name); errdefs.IsNotFound(err) {
				return nil
			} else if err != nil {
				err = pkgerrors.Wrapf(err, "Failed to retrieve details of %q in the provider", loggablePodNameFromCoordinates(update.oldPod.Namespace, update.oldPod.Name))
				span.SetStatus(err)
				return err
			} else if podStatus == nil {
				return nil
			}
		}
		log.G(ctx).Debug("Deleting pod in provider, after deletes from kubernetes")
		err := pc.provider.DeletePod(ctx, update.oldPod)
		if err != nil && !errdefs.IsNotFound(err) {
			err = pkgerrors.Wrapf(err, "failed to delete pod %q in the provider", loggablePodNameFromCoordinates(update.oldPod.Namespace, update.oldPod.Name))
			span.SetStatus(err)
		}
		return err
	}

	if update.oldPod == nil {
		// Pod is newly created
		// Add the pod's attributes to the current span.
		ctx = addPodAttributes(ctx, span, update.newPod)
		if shouldIgnorePod(ctx, update.newPod) {
			return nil
		}
		if update.newPod.DeletionGracePeriodSeconds != nil {
			return nil
		}
		err := pc.createOrUpdatePod(ctx, update.newPod)
		span.SetStatus(err)
		return err
	} else {
		// Add the pod's attributes to the current span.
		ctx = addPodAttributes(ctx, span, update.oldPod)
	}

	// pod experienced update
	if deletable(update.newPod) {
		if deletable(update.oldPod) {
			if *update.oldPod.DeletionGracePeriodSeconds == *update.newPod.DeletionGracePeriodSeconds {
				return nil
			}
		}
		err := pc.provider.DeletePod(ctx, update.newPod)
		log.G(ctx).WithError(err).Debug()
		if errdefs.IsNotFound(err) {
			log.G(ctx).WithError(err).Debug("Force deleting pod from kubernetes")
			err = pc.forceDeletePodResource(ctx, update.newPod.Namespace, update.newPod.Name)
			span.SetStatus(err)
			return err
		} else if err != nil {
			err = pkgerrors.Wrapf(err, "failed to delete pod %q in the provider", loggablePodNameFromCoordinates(update.oldPod.Namespace, update.oldPod.Name))
			span.SetStatus(err)
			return err
		}
	}

	// At this point we know the Pod resource has either been created or updated (which includes being marked for deletion).
	// Create or update the pod in the provider.
	if err := pc.createOrUpdatePod(ctx, update.newPod); err != nil {
		err := pkgerrors.Wrapf(err, "failed to sync pod %q in the provider", loggablePodName(update.newPod))
		span.SetStatus(err)
		return err
	}
	return nil
}

// deleteDanglingPods checks whether the provider knows about any pods which Kubernetes doesn't know about, and deletes them.
func (pc *PodController) deleteDanglingPods(ctx context.Context, threadiness int) {
	ctx, span := trace.StartSpan(ctx, "deleteDanglingPods")
	defer span.End()

	// Grab the list of pods known to the provider.
	pps, err := pc.provider.GetPods(ctx)
	if err != nil {
		err := pkgerrors.Wrap(err, "failed to fetch the list of pods from the provider")
		span.SetStatus(err)
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
			span.SetStatus(err)
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
			if err := pc.deletePod(ctx, pod); err != nil {
				span.SetStatus(err)
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
