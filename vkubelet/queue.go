package vkubelet

import (
	"context"

	"github.com/cpuguy83/strongerrors/status/ocstatus"
	pkgerrors "github.com/pkg/errors"
	"github.com/virtual-kubelet/virtual-kubelet/log"
	"github.com/virtual-kubelet/virtual-kubelet/trace"
	"k8s.io/client-go/util/workqueue"
)

const (
	// maxRetries is the number of times we try to process a given key before permanently forgetting it.
	maxRetries = 20
)

type queueHandler func(ctx context.Context, key string) error

func handleQueueItem(ctx context.Context, q workqueue.RateLimitingInterface, handler queueHandler) bool {
	ctx, span := trace.StartSpan(ctx, "handleQueueItem")
	defer span.End()

	obj, shutdown := q.Get()
	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		// We call Done here so the work queue knows we have finished processing this item.
		// We also must remember to call Forget if we do not want this work item being re-queued.
		// For example, we do not call Forget if a transient error occurs.
		// Instead, the item is put back on the work queue and attempted again after a back-off period.
		defer q.Done(obj)
		var key string
		var ok bool
		// We expect strings to come off the work queue.
		// These are of the form namespace/name.
		// We do this as the delayed nature of the work queue means the items in the informer cache may actually be more up to date that when the item was initially put onto the workqueue.
		if key, ok = obj.(string); !ok {
			// As the item in the work queue is actually invalid, we call Forget here else we'd go into a loop of attempting to process a work item that is invalid.
			q.Forget(obj)
			log.G(ctx).Warnf("expected string in work queue item but got %#v", obj)
			return nil
		}
		// Add the current key as an attribute to the current span.
		ctx = span.WithField(ctx, "key", key)
		// Run the syncHandler, passing it the namespace/name string of the Pod resource to be synced.
		if err := handler(ctx, key); err != nil {
			if q.NumRequeues(key) < maxRetries {
				// Put the item back on the work queue to handle any transient errors.
				log.G(ctx).Warnf("requeuing %q due to failed sync: %v", key, err)
				q.AddRateLimited(key)
				return nil
			}
			// We've exceeded the maximum retries, so we must forget the key.
			q.Forget(key)
			return pkgerrors.Wrapf(err, "forgetting %q due to maximum retries reached", key)
		}
		// Finally, if no error occurs we Forget this item so it does not get queued again until another change happens.
		q.Forget(obj)
		return nil
	}(obj)

	if err != nil {
		// We've actually hit an error, so we set the span's status based on the error.
		span.SetStatus(ocstatus.FromError(err))
		log.G(ctx).Error(err)
		return true
	}

	return true
}
