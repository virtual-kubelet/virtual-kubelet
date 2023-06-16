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
	"sync"
	"time"

	"github.com/google/go-cmp/cmp"
	pkgerrors "github.com/pkg/errors"
	"github.com/virtual-kubelet/virtual-kubelet/errdefs"
	"github.com/virtual-kubelet/virtual-kubelet/internal/manager"
	"github.com/virtual-kubelet/virtual-kubelet/internal/queue"
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

	// DeletePod takes a Kubernetes Pod and deletes it from the provider. Once a pod is deleted, the provider is
	// expected to call the NotifyPods callback with a terminal pod status where all the containers are in a terminal
	// state, as well as the pod. DeletePod may be called multiple times for the same pod.
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

// PodNotifier is used as an extension to PodLifecycleHandler to support async updates of pod statuses.
type PodNotifier interface {
	// NotifyPods instructs the notifier to call the passed in function when
	// the pod status changes. It should be called when a pod's status changes.
	//
	// The provided pointer to a Pod is guaranteed to be used in a read-only
	// fashion. The provided pod's PodStatus should be up to date when
	// this function is called.
	//
	// NotifyPods must not block the caller since it is only used to register the callback.
	// The callback passed into `NotifyPods` may block when called.
	NotifyPods(context.Context, func(*corev1.Pod))
}

// PodEventFilterFunc is used to filter pod events received from Kubernetes.
//
// Filters that return true means the event handler will be run
// Filters that return false means the filter will *not* be run.
type PodEventFilterFunc func(context.Context, *corev1.Pod) bool

// PodController is the controller implementation for Pod resources.
type PodController struct {
	provider PodLifecycleHandler

	// podsInformer is an informer for Pod resources.
	podsInformer corev1informers.PodInformer
	// podsLister is able to list/get Pod resources from a shared informer's store.
	podsLister corev1listers.PodLister

	// recorder is an event recorder for recording Event resources to the Kubernetes API.
	recorder record.EventRecorder

	client corev1client.PodsGetter

	resourceManager *manager.ResourceManager

	syncPodsFromKubernetes *queue.Queue

	// deletePodsFromKubernetes is a queue on which pods are reconciled, and we check if pods are in API server after
	// the grace period
	deletePodsFromKubernetes *queue.Queue

	syncPodStatusFromProvider *queue.Queue

	// From the time of creation, to termination the knownPods map will contain the pods key
	// (derived from Kubernetes' cache library) -> a *knownPod struct.
	knownPods sync.Map

	podEventFilterFunc PodEventFilterFunc

	// ready is a channel which will be closed once the pod controller is fully up and running.
	// this channel will never be closed if there is an error on startup.
	ready chan struct{}
	// done is closed when Run returns
	// Once done is closed `err` may be set to a non-nil value
	done chan struct{}

	mu sync.Mutex
	// err is set if there is an error while while running the pod controller.
	// Typically this would be errors that occur during startup.
	// Once err is set, `Run` should return.
	//
	// This is used since `pc.Run()` is typically called in a goroutine and managing
	// this can be non-trivial for callers.
	err error
}

type knownPod struct {
	// You cannot read (or modify) the fields in this struct without taking the lock. The individual fields
	// should be immutable to avoid having to hold the lock the entire time you're working with them
	sync.Mutex
	lastPodStatusReceivedFromProvider *corev1.Pod
	lastPodUsed                       *corev1.Pod
	lastPodStatusUpdateSkipped        bool
}

// PodControllerConfig is used to configure a new PodController.
type PodControllerConfig struct {
	// PodClient is used to perform actions on the k8s API, such as updating pod status
	// This field is required
	PodClient corev1client.PodsGetter

	// PodInformer is used as a local cache for pods
	// This should be configured to only look at pods scheduled to the node which the controller will be managing
	// If the informer does not filter based on node, then you must provide a `PodEventFilterFunc` parameter so event handlers
	//   can filter pods not assigned to this node.
	PodInformer corev1informers.PodInformer

	EventRecorder record.EventRecorder

	Provider PodLifecycleHandler

	// Informers used for filling details for things like downward API in pod spec.
	//
	// We are using informers here instead of listers because we'll need the
	// informer for certain features (like notifications for updated ConfigMaps)
	ConfigMapInformer corev1informers.ConfigMapInformer
	SecretInformer    corev1informers.SecretInformer
	ServiceInformer   corev1informers.ServiceInformer

	// SyncPodsFromKubernetesRateLimiter defines the rate limit for the SyncPodsFromKubernetes queue
	SyncPodsFromKubernetesRateLimiter workqueue.RateLimiter
	// SyncPodsFromKubernetesShouldRetryFunc allows for a custom retry policy for the SyncPodsFromKubernetes queue
	SyncPodsFromKubernetesShouldRetryFunc ShouldRetryFunc

	// DeletePodsFromKubernetesRateLimiter defines the rate limit for the DeletePodsFromKubernetesRateLimiter queue
	DeletePodsFromKubernetesRateLimiter workqueue.RateLimiter
	// DeletePodsFromKubernetesShouldRetryFunc allows for a custom retry policy for the SyncPodsFromKubernetes queue
	DeletePodsFromKubernetesShouldRetryFunc ShouldRetryFunc

	// SyncPodStatusFromProviderRateLimiter defines the rate limit for the SyncPodStatusFromProviderRateLimiter queue
	SyncPodStatusFromProviderRateLimiter workqueue.RateLimiter
	// SyncPodStatusFromProviderShouldRetryFunc allows for a custom retry policy for the SyncPodStatusFromProvider queue
	SyncPodStatusFromProviderShouldRetryFunc ShouldRetryFunc

	// Add custom filtering for pod informer event handlers
	// Use this for cases where the pod informer handles more than pods assigned to this node
	//
	// For example, if the pod informer is not filtering based on pod.Spec.NodeName, you should
	// set that filter here so the pod controller does not handle events for pods assigned to other nodes.
	PodEventFilterFunc PodEventFilterFunc
}

// NewPodController creates a new pod controller with the provided config.
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
	if cfg.SyncPodsFromKubernetesRateLimiter == nil {
		cfg.SyncPodsFromKubernetesRateLimiter = workqueue.DefaultControllerRateLimiter()
	}
	if cfg.DeletePodsFromKubernetesRateLimiter == nil {
		cfg.DeletePodsFromKubernetesRateLimiter = workqueue.DefaultControllerRateLimiter()
	}
	if cfg.SyncPodStatusFromProviderRateLimiter == nil {
		cfg.SyncPodStatusFromProviderRateLimiter = workqueue.DefaultControllerRateLimiter()
	}
	rm, err := manager.NewResourceManager(cfg.PodInformer.Lister(), cfg.SecretInformer.Lister(), cfg.ConfigMapInformer.Lister(), cfg.ServiceInformer.Lister())
	if err != nil {
		return nil, pkgerrors.Wrap(err, "could not create resource manager")
	}

	pc := &PodController{
		client:             cfg.PodClient,
		podsInformer:       cfg.PodInformer,
		podsLister:         cfg.PodInformer.Lister(),
		provider:           cfg.Provider,
		resourceManager:    rm,
		ready:              make(chan struct{}),
		done:               make(chan struct{}),
		recorder:           cfg.EventRecorder,
		podEventFilterFunc: cfg.PodEventFilterFunc,
	}

	pc.syncPodsFromKubernetes = queue.New(cfg.SyncPodsFromKubernetesRateLimiter, "syncPodsFromKubernetes", pc.syncPodFromKubernetesHandler, cfg.SyncPodsFromKubernetesShouldRetryFunc)
	pc.deletePodsFromKubernetes = queue.New(cfg.DeletePodsFromKubernetesRateLimiter, "deletePodsFromKubernetes", pc.deletePodsFromKubernetesHandler, cfg.DeletePodsFromKubernetesShouldRetryFunc)
	pc.syncPodStatusFromProvider = queue.New(cfg.SyncPodStatusFromProviderRateLimiter, "syncPodStatusFromProvider", pc.syncPodStatusFromProviderHandler, cfg.SyncPodStatusFromProviderShouldRetryFunc)

	return pc, nil
}

type asyncProvider interface {
	PodLifecycleHandler
	PodNotifier
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers.  It will block until the
// context is cancelled, at which point it will shutdown the work queue and
// wait for workers to finish processing their current work items prior to
// returning.
//
// Once this returns, you should not re-use the controller.
func (pc *PodController) Run(ctx context.Context, podSyncWorkers int) (retErr error) {
	// Shutdowns are idempotent, so we can call it multiple times. This is in case we have to bail out early for some reason.
	// This is to make extra sure that any workers we started are terminated on exit
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	defer func() {
		pc.mu.Lock()
		pc.err = retErr
		close(pc.done)
		pc.mu.Unlock()
	}()

	var provider asyncProvider
	runProvider := func(context.Context) {}

	if p, ok := pc.provider.(asyncProvider); ok {
		provider = p
	} else {
		wrapped := &syncProviderWrapper{PodLifecycleHandler: pc.provider, l: pc.podsLister}
		runProvider = wrapped.run
		provider = wrapped
		log.G(ctx).Debug("Wrapped non-async provider with async")
	}
	pc.provider = provider

	provider.NotifyPods(ctx, func(pod *corev1.Pod) {
		pc.enqueuePodStatusUpdate(ctx, pod.DeepCopy())
	})
	go runProvider(ctx)

	// Wait for the caches to be synced *before* starting to do work.
	if ok := cache.WaitForCacheSync(ctx.Done(), pc.podsInformer.Informer().HasSynced); !ok {
		return pkgerrors.New("failed to wait for caches to sync")
	}
	log.G(ctx).Info("Pod cache in-sync")

	// Set up event handlers for when Pod resources change. Since the pod cache is in-sync, the informer will generate
	// synthetic add events at this point. It again avoids the race condition of adding handlers while the cache is
	// syncing.

	var eventHandler cache.ResourceEventHandler = cache.ResourceEventHandlerFuncs{
		AddFunc: func(pod interface{}) {
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()
			ctx, span := trace.StartSpan(ctx, "AddFunc")
			defer span.End()

			if key, err := cache.MetaNamespaceKeyFunc(pod); err != nil {
				log.G(ctx).Error(err)
			} else {
				ctx = span.WithField(ctx, "key", key)
				pc.knownPods.Store(key, &knownPod{})
				pc.syncPodsFromKubernetes.Enqueue(ctx, key)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()
			ctx, span := trace.StartSpan(ctx, "UpdateFunc")
			defer span.End()

			// Create a copy of the old and new pod objects so we don't mutate the cache.
			oldPod := oldObj.(*corev1.Pod)
			newPod := newObj.(*corev1.Pod)

			// At this point we know that something in .metadata or .spec has changed, so we must proceed to sync the pod.
			if key, err := cache.MetaNamespaceKeyFunc(newPod); err != nil {
				log.G(ctx).Error(err)
			} else {
				ctx = span.WithField(ctx, "key", key)
				obj, ok := pc.knownPods.Load(key)
				if !ok {
					// Pods are only ever *added* to knownPods in the above AddFunc, and removed
					// in the below *DeleteFunc*
					panic("Pod not found in known pods. This should never happen.")
				}

				kPod := obj.(*knownPod)
				kPod.Lock()
				if kPod.lastPodStatusUpdateSkipped &&
					(!cmp.Equal(newPod.Status, kPod.lastPodStatusReceivedFromProvider.Status) ||
						!cmp.Equal(newPod.Annotations, kPod.lastPodStatusReceivedFromProvider.Annotations) ||
						!cmp.Equal(newPod.Labels, kPod.lastPodStatusReceivedFromProvider.Labels) ||
						!cmp.Equal(newPod.Finalizers, kPod.lastPodStatusReceivedFromProvider.Finalizers)) {
					// The last pod from the provider -> kube api server was skipped, but we see they no longer match.
					// This means that the pod in API server was changed by someone else [this can be okay], but we skipped
					// a status update on our side because we compared the status received from the provider to the status
					// received from the k8s api server based on outdated information.
					pc.syncPodStatusFromProvider.Enqueue(ctx, key)
					// Reset this to avoid re-adding it continuously
					kPod.lastPodStatusUpdateSkipped = false
				}
				kPod.Unlock()

				if podShouldEnqueue(oldPod, newPod) {
					pc.syncPodsFromKubernetes.Enqueue(ctx, key)
				}
			}
		},
		DeleteFunc: func(pod interface{}) {
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()
			ctx, span := trace.StartSpan(ctx, "DeleteFunc")
			defer span.End()

			if key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(pod); err != nil {
				log.G(ctx).Error(err)
			} else {
				k8sPod, ok := pod.(*corev1.Pod)
				if !ok {
					return
				}
				ctx = span.WithField(ctx, "key", key)
				pc.knownPods.Delete(key)
				pc.syncPodsFromKubernetes.Enqueue(ctx, key)
				// If this pod was in the deletion queue, forget about it
				key = fmt.Sprintf("%v/%v", key, k8sPod.UID)
				pc.deletePodsFromKubernetes.Forget(ctx, key)
			}
		},
	}

	if pc.podEventFilterFunc != nil {
		eventHandler = cache.FilteringResourceEventHandler{
			FilterFunc: func(obj interface{}) bool {
				p, ok := obj.(*corev1.Pod)
				if !ok {
					return false
				}
				return pc.podEventFilterFunc(ctx, p)
			},
			Handler: eventHandler,
		}
	}

	_, err := pc.podsInformer.Informer().AddEventHandler(eventHandler)
	if err != nil {
		log.G(ctx).Error(err)
	}

	// Perform a reconciliation step that deletes any dangling pods from the provider.
	// This happens only when the virtual-kubelet is starting, and operates on a "best-effort" basis.
	// If by any reason the provider fails to delete a dangling pod, it will stay in the provider and deletion won't be retried.
	pc.deleteDanglingPods(ctx, podSyncWorkers)

	log.G(ctx).Info("starting workers")
	group := &wait.Group{}
	group.StartWithContext(ctx, func(ctx context.Context) {
		pc.syncPodsFromKubernetes.Run(ctx, podSyncWorkers)
	})
	group.StartWithContext(ctx, func(ctx context.Context) {
		pc.deletePodsFromKubernetes.Run(ctx, podSyncWorkers)
	})
	group.StartWithContext(ctx, func(ctx context.Context) {
		pc.syncPodStatusFromProvider.Run(ctx, podSyncWorkers)
	})
	defer group.Wait()
	log.G(ctx).Info("started workers")
	close(pc.ready)

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

// Done returns a channel receiver which is closed when the pod controller has exited.
// Once the pod controller has exited you can call `pc.Err()` to see if any error occurred.
func (pc *PodController) Done() <-chan struct{} {
	return pc.done
}

// Err returns any error that has occurred and caused the pod controller to exit.
func (pc *PodController) Err() error {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	return pc.err
}

// syncPodFromKubernetesHandler compares the actual state with the desired, and attempts to converge the two.
func (pc *PodController) syncPodFromKubernetesHandler(ctx context.Context, key string) error {
	ctx, span := trace.StartSpan(ctx, "syncPodFromKubernetesHandler")
	defer span.End()

	// Add the current key as an attribute to the current span.
	ctx = span.WithField(ctx, "key", key)
	log.G(ctx).WithField("key", key).Debug("sync handled")

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
			span.SetStatus(err)
			return err
		}

		pod, err = pc.provider.GetPod(ctx, namespace, name)
		if err != nil && !errdefs.IsNotFound(err) {
			err = pkgerrors.Wrapf(err, "failed to fetch pod with key %q from provider", key)
			span.SetStatus(err)
			return err
		}
		if errdefs.IsNotFound(err) || pod == nil {
			return nil
		}

		err = pc.provider.DeletePod(ctx, pod)
		if errdefs.IsNotFound(err) {
			return nil
		}
		if err != nil {
			err = pkgerrors.Wrapf(err, "failed to delete pod %q in the provider", loggablePodNameFromCoordinates(namespace, name))
			span.SetStatus(err)
		}
		return err

	}

	// At this point we know the Pod resource has either been created or updated (which includes being marked for deletion).
	return pc.syncPodInProvider(ctx, pod, key)
}

// syncPodInProvider tries and reconciles the state of a pod by comparing its Kubernetes representation and the provider's representation.
func (pc *PodController) syncPodInProvider(ctx context.Context, pod *corev1.Pod, key string) (retErr error) {
	ctx, span := trace.StartSpan(ctx, "syncPodInProvider")
	defer span.End()

	// Add the pod's attributes to the current span.
	ctx = addPodAttributes(ctx, span, pod)

	// If the pod('s containers) is no longer in a running state then we force-delete the pod from API server
	// more context is here: https://github.com/virtual-kubelet/virtual-kubelet/pull/760
	if pod.DeletionTimestamp != nil && !running(&pod.Status) {
		log.G(ctx).Debug("Force deleting pod from API Server as it is no longer running")
		key = fmt.Sprintf("%v/%v", key, pod.UID)
		pc.deletePodsFromKubernetes.EnqueueWithoutRateLimit(ctx, key)
		return nil
	}
	obj, ok := pc.knownPods.Load(key)
	if !ok {
		// That means the pod was deleted while we were working
		return nil
	}
	kPod := obj.(*knownPod)
	kPod.Lock()
	if kPod.lastPodUsed != nil && podsEffectivelyEqual(kPod.lastPodUsed, pod) {
		kPod.Unlock()
		return nil
	}
	kPod.Unlock()

	defer func() {
		if retErr == nil {
			kPod.Lock()
			defer kPod.Unlock()
			kPod.lastPodUsed = pod
		}
	}()

	// Check whether the pod has been marked for deletion.
	// If it does, guarantee it is deleted in the provider and Kubernetes.
	if pod.DeletionTimestamp != nil {
		log.G(ctx).Debug("Deleting pod in provider")
		if err := pc.deletePod(ctx, pod); errdefs.IsNotFound(err) {
			log.G(ctx).Debug("Pod not found in provider")
		} else if err != nil {
			err := pkgerrors.Wrapf(err, "failed to delete pod %q in the provider", loggablePodName(pod))
			span.SetStatus(err)
			return err
		}

		key = fmt.Sprintf("%v/%v", key, pod.UID)
		pc.deletePodsFromKubernetes.EnqueueWithoutRateLimitWithDelay(ctx, key, time.Second*time.Duration(*pod.DeletionGracePeriodSeconds))
		return nil
	}

	// Ignore the pod if it is in the "Failed" or "Succeeded" state.
	if pod.Status.Phase == corev1.PodFailed || pod.Status.Phase == corev1.PodSucceeded {
		log.G(ctx).Warnf("skipping sync of pod %q in %q phase", loggablePodName(pod), pod.Status.Phase)
		return nil
	}

	// Create or update the pod in the provider.
	if err := pc.createOrUpdatePod(ctx, pod); err != nil {
		err := pkgerrors.Wrapf(err, "failed to sync pod %q in the provider", loggablePodName(pod))
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
			if err := pc.provider.DeletePod(ctx, pod.DeepCopy()); err != nil && !errdefs.IsNotFound(err) {
				span.SetStatus(err)
				log.G(ctx).Errorf("failed to delete pod %q in provider", loggablePodName(pod))
			} else {
				log.G(ctx).Infof("deleted leaked pod %q in provider", loggablePodName(pod))
			}
		}(ctx, pod)
	}

	// Wait for all pods to be deleted.
	wg.Wait()
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

// podsEffectivelyEqual compares two pods, and ignores the pod status, and the resource version
func podsEffectivelyEqual(p1, p2 *corev1.Pod) bool {
	filterForResourceVersion := func(p cmp.Path) bool {
		if p.String() == "ObjectMeta.ResourceVersion" {
			return true
		}
		if p.String() == "Status" {
			return true
		}
		return false
	}

	return cmp.Equal(p1, p2, cmp.FilterPath(filterForResourceVersion, cmp.Ignore()))
}

// borrowed from https://github.com/kubernetes/kubernetes/blob/f64c631cd7aea58d2552ae2038c1225067d30dde/pkg/kubelet/kubelet_pods.go#L944-L953
// running returns true, unless if every status is terminated or waiting, or the status list
// is empty.
func running(podStatus *corev1.PodStatus) bool {
	statuses := podStatus.ContainerStatuses
	for _, status := range statuses {
		if status.State.Terminated == nil && status.State.Waiting == nil {
			return true
		}
	}
	return false
}
