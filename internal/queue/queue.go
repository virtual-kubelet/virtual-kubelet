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

package queue

import (
	"context"
	"fmt"
	"sync"
	"time"

	pkgerrors "github.com/pkg/errors"
	"github.com/virtual-kubelet/virtual-kubelet/log"
	"github.com/virtual-kubelet/virtual-kubelet/trace"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/workqueue"
)

const (
	// MaxRetries is the number of times we try to process a given key before permanently forgetting it.
	MaxRetries = 20
)

// ItemHandler is a callback that handles a single key on the Queue
type ItemHandler func(ctx context.Context, key string) error

// Queue implements a wrapper around workqueue with native VK instrumentation
type Queue struct {
	lock      sync.Mutex
	running   bool
	name      string
	workqueue workqueue.RateLimitingInterface
	handler   ItemHandler
}

// New creates a queue
//
// It expects to get a item rate limiter, and a friendly name which is used in logs, and
// in the internal kubernetes metrics.
func New(ratelimiter workqueue.RateLimiter, name string, handler ItemHandler) *Queue {
	return &Queue{
		name:      name,
		workqueue: workqueue.NewNamedRateLimitingQueue(ratelimiter, name),
		handler:   handler,
	}
}

// Enqueue enqueues the key in a rate limited fashion
func (q *Queue) Enqueue(key string) {
	q.workqueue.AddRateLimited(key)
}

// EnqueueWithoutRateLimit enqueues the key without a rate limit
func (q *Queue) EnqueueWithoutRateLimit(key string) {
	q.workqueue.Add(key)
}

// Forget forgets the key
func (q *Queue) Forget(key string) {
	q.workqueue.Forget(key)
}

// EnqueueAfter enqueues the item after this period
//
// Since it wrap workqueue semantics, if an item has been enqueued  after, and it is immediately scheduled for work,
// it will process the immediate item, and then upon the latter delayed processing it will be processed again
func (q *Queue) EnqueueAfter(key string, after time.Duration) {
	q.workqueue.AddAfter(key, after)
}

// Empty returns if the queue has no items in it
//
// It should only be used for debugging, as delayed items are not counted, leading to confusion
func (q *Queue) Empty() bool {
	return q.workqueue.Len() == 0
}

// Run starts the workers
//
// It blocks until context is cancelled, and all of the workers exit.
func (q *Queue) Run(ctx context.Context, workers int) {
	if workers <= 0 {
		panic(fmt.Sprintf("Workers must be greater than 0, got: %d", workers))
	}

	q.lock.Lock()
	if q.running {
		panic(fmt.Sprintf("Queue %s is already running", q.name))
	}
	q.running = true
	q.lock.Unlock()
	defer func() {
		q.lock.Lock()
		defer q.lock.Unlock()
		q.running = false
	}()

	// Make sure all workers are stopped before we finish up.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	group := &wait.Group{}
	for i := 0; i < workers; i++ {
		group.StartWithContext(ctx, func(ctx context.Context) {
			q.worker(ctx, i)
		})
	}
	defer group.Wait()
	<-ctx.Done()
	q.workqueue.ShutDown()
}

func (q *Queue) worker(ctx context.Context, i int) {
	ctx = log.WithLogger(ctx, log.G(ctx).WithFields(map[string]interface{}{
		"workerId": i,
		"Queue":    q.name,
	}))
	for q.handleQueueItem(ctx) {
	}
}

// handleQueueItem handles a single item
//
// A return value of "false" indicates that further processing should be stopped.
func (q *Queue) handleQueueItem(ctx context.Context) bool {
	ctx, span := trace.StartSpan(ctx, "handleQueueItem")
	defer span.End()

	obj, shutdown := q.workqueue.Get()
	if shutdown {
		return false
	}

	// We expect strings to come off the work Queue.
	// These are of the form namespace/name.
	// We do this as the delayed nature of the work Queue means the items in the informer cache may actually be more u
	// to date that when the item was initially put onto the workqueue.
	key := obj.(string)
	ctx = span.WithField(ctx, "key", key)
	log.G(ctx).Debug("Got Queue object")

	err := q.handleQueueItemObject(ctx, key)
	if err != nil {
		// We've actually hit an error, so we set the span's status based on the error.
		span.SetStatus(err)
		log.G(ctx).WithError(err).Error("Error processing Queue item")
		return true
	}
	log.G(ctx).Debug("Processed Queue item")

	return true
}

func (q *Queue) handleQueueItemObject(ctx context.Context, key string) error {
	// This is a separate function / span, because the handleQueueItem span is the time spent waiting for the object
	// plus the time spend handling the object. Instead, this function / span is scoped to a single object.
	ctx, span := trace.StartSpan(ctx, "handleQueueItemObject")
	defer span.End()
	ctx = span.WithField(ctx, "key", key)

	// We call Done here so the work Queue knows we have finished processing this item.
	// We also must remember to call Forget if we do not want this work item being re-queued.
	// For example, we do not call Forget if a transient error occurs.
	// Instead, the item is put back on the work Queue and attempted again after a back-off period.
	defer q.workqueue.Done(key)

	// Add the current key as an attribute to the current span.
	ctx = span.WithField(ctx, "key", key)
	// Run the syncHandler, passing it the namespace/name string of the Pod resource to be synced.
	if err := q.handler(ctx, key); err != nil {
		if q.workqueue.NumRequeues(key) < MaxRetries {
			// Put the item back on the work Queue to handle any transient errors.
			log.G(ctx).WithError(err).Warnf("requeuing %q due to failed sync", key)
			q.workqueue.AddRateLimited(key)
			return nil
		}
		// We've exceeded the maximum retries, so we must Forget the key.
		q.workqueue.Forget(key)
		return pkgerrors.Wrapf(err, "forgetting %q due to maximum retries reached", key)
	}
	// Finally, if no error occurs we Forget this item so it does not get queued again until another change happens.
	q.workqueue.Forget(key)

	return nil
}
