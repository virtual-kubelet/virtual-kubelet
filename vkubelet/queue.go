package vkubelet

import (
	"context"
	"hash/fnv"

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

type workItem func(ctx context.Context) error

type wrappedWorkItem struct {
	key  string
	item workItem
}

func addItem(q []workqueue.RateLimitingInterface, key string, item workItem) {
	hash := fnv.New32a()
	_, err := hash.Write([]byte(key))
	// This should be impossible
	if err != nil {
		panic(err)
	}

	idx := int(hash.Sum32()) % len(q)
	addItemToSingleQueue(q[idx], key, item)
}

func addItemToSingleQueue(q workqueue.RateLimitingInterface, key string, item workItem) {
	q.AddRateLimited(&wrappedWorkItem{key: key, item: item})
}

func handleQueueItem(ctx context.Context, q workqueue.RateLimitingInterface) bool {
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
		var item *wrappedWorkItem
		var ok bool
		// We expect strings to come off the work queue.
		// These are of the form namespace/name.
		// We do this as the delayed nature of the work queue means the items in the informer cache may actually be more up to date that when the item was initially put onto the workqueue.
		if item, ok = obj.(*wrappedWorkItem); !ok {
			// As the item in the work queue is actually invalid, we call Forget here else we'd go into a loop of attempting to process a work item that is invalid.
			q.Forget(obj)
			log.G(ctx).Warnf("expected wrappedWorkItem in work queue item but got %#v", obj)
			return nil
		}

		// Run the syncHandler, passing it the namespace/name string of the Pod resource to be synced.
		if err := item.item(ctx); err != nil {
			if q.NumRequeues(item) < maxRetries {
				// Put the item back on the work queue to handle any transient errors.
				log.G(ctx).WithField("key", item.key).WithError(err).Warn("requeuing due to failed sync")
				q.AddRateLimited(item)
				return nil
			}
			// We've exceeded the maximum retries, so we must forget the key.
			q.Forget(item)
			return pkgerrors.Wrapf(err, "forgetting %q due to maximum retries reached", item.key)
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
