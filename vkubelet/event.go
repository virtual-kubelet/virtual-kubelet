package vkubelet

import (
	"context"

	"github.com/cpuguy83/strongerrors"
	"github.com/cpuguy83/strongerrors/status/ocstatus"
	pkgerrors "github.com/pkg/errors"
	"github.com/virtual-kubelet/virtual-kubelet/log"
	"go.opencensus.io/trace"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"
	"k8s.io/kubernetes/staging/src/k8s.io/client-go/util/workqueue"
)

type event struct {
	key string
	ctx context.Context
}

func handleEvent(ctx context.Context, obj interface{}, eventName string, q workqueue.RateLimitingInterface) {
	ctx, span := trace.StartSpan(ctx, eventName)
	defer span.End()
	logger := log.G(ctx).WithField("eventType", eventName)

	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		span.SetStatus(trace.Status{Code: trace.StatusCodeInvalidArgument, Message: err.Error()})
		logger.WithError(err).Errorf("error generating key for object in event handler")
		return
	}

	logger = logger.WithField("storeKey", key)
	ctx = log.WithLogger(ctx, logger)
	span.AddAttributes(trace.StringAttribute("storeKey", key))

	logger.Debug("Adding event to queue")
	q.Add(&event{key: key, ctx: ctx})
	span.Annotate(nil, "added pod to queue")
}

// watchForPodEvent waits for pod changes from kubernetes and updates the details accordingly in the local state.
// This returns after a single pod event.
func (s *Server) watchForPodEvent(ctx context.Context) error {
	q := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	defer q.ShutDown()

	s.podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			handleEvent(ctx, obj, "AddEvent", q)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			handleEvent(ctx, newObj, "UpdateEvent", q)
		},
		DeleteFunc: func(obj interface{}) {
			handleEvent(ctx, obj, "DeleteEvent", q)
		},
	})

	if !cache.WaitForCacheSync(ctx.Done(), s.podInformer.Informer().HasSynced) {
		return pkgerrors.Wrap(ctx.Err(), "error waiting for cache sync")
	}

	for i := 0; i < s.podSyncWorkers; i++ {
		go s.startPodSynchronizer(ctx, i, q)
	}

	log.G(ctx).Info("Start to run pod cache controller.")

	<-ctx.Done()

	return ctx.Err()
}

func (s *Server) startPodSynchronizer(ctx context.Context, id int, q workqueue.RateLimitingInterface) {
	logger := log.G(ctx).WithField("method", "startPodSynchronizer").WithField("podSynchronizer", id)
	logger.Debug("Start pod synchronizer")
	id64 := int64(id)

	for {
		e, quit := q.Get()
		if quit {
			return
		}

		event := e.(*event)

		retries := q.NumRequeues(event)

		logger = log.G(event.ctx).
			WithField("retries", retries).
			WithField("workerID", id).
			WithField("maxRetries", maxSyncRetries)
		ctx := log.WithLogger(event.ctx, logger)

		func() {
			defer q.Done(event)

			ctx, span := trace.StartSpan(ctx, "processEvent")
			defer span.End()
			span.AddAttributes(
				trace.Int64Attribute("workerID", id64),
				trace.StringAttribute("storeKey", event.key),
				trace.Int64Attribute("retries", int64(retries)),
			)

			if err := s.processEvent(ctx, event); err != nil {
				if strongerrors.IsNotFound(err) {
					logger.WithError(err).Debug("could not process event")
				} else {
					span.SetStatus(ocstatus.FromError(err))
					logger.WithError(err).Error("Error processing event")

					if retries > maxSyncRetries {
						q.Forget(event)
						span.Annotate(nil, "not requeueing")
						return
					}

					q.AddRateLimited(event)
					span.Annotate(nil, "requeued event")
				}
			}

			q.Forget(event)
		}()
	}
}

func (s *Server) processEvent(ctx context.Context, e *event) error {
	ns, name, err := cache.SplitMetaNamespaceKey(e.key)
	if err != nil {
		return pkgerrors.Wrap(err, "error splitting cache key")
	}

	pod, err := s.podInformer.Lister().Pods(ns).Get(name)
	if err != nil {
		if !errors.IsNotFound(err) {
			return pkgerrors.Wrapf(err, "error looking up pod %q", e.key)
		}

		if pod, _ := s.provider.GetPod(ctx, ns, name); pod != nil {
			if err := s.provider.DeletePod(ctx, pod); err != nil {
				return pkgerrors.Wrapf(err, "error deleting pod %q from provider", pod.GetName())
			}
		}

		if pod := s.resourceManager.GetPod(ns, name); pod != nil {
			log.G(ctx).Debug("cleaning up pod from resource manager")
			s.resourceManager.DeletePod(pod)
		}
		// This case can happen especially if an event either arrived late or was
		// processed late and the resources are no longer available.
		//
		// It's not a very bad error and any uneeded resources not dealt with here
		// will get cleaned up separately.
		return strongerrors.NotFound(pkgerrors.Wrapf(err, "could not find pod %s/%s", ns, name))
	}

	if !s.resourceManager.UpdatePod(pod) {
		if span := trace.FromContext(ctx); span != nil {
			addPodAttributes(span, pod)
			span.Annotate(nil, "no pod update required")
		}
		return nil
	}

	return s.syncPod(e.ctx, pod)
}
