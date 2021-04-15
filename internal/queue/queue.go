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
	"container/list"
	"context"
	"fmt"
	"sync"
	"time"

	pkgerrors "github.com/pkg/errors"
	"github.com/virtual-kubelet/virtual-kubelet/log"
	"github.com/virtual-kubelet/virtual-kubelet/trace"
	"golang.org/x/sync/semaphore"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/clock"
)

const (
	// MaxRetries is the number of times we try to process a given key before permanently forgetting it.
	MaxRetries = 20
)

// ShouldRetryFunc is a mechanism to have a custom retry policy
type ShouldRetryFunc func(ctx context.Context, key string, timesTried int, originallyAdded time.Time, err error) (*time.Duration, error)

// ItemHandler is a callback that handles a single key on the Queue
type ItemHandler func(ctx context.Context, key string) error

// Queue implements a wrapper around workqueue with native VK instrumentation
type Queue struct {
	// clock is used for testing
	clock clock.Clock
	// lock protects running, and the items list / map
	lock    sync.Mutex
	running bool
	name    string
	handler ItemHandler

	ratelimiter workqueue.RateLimiter
	// items are items that are marked dirty waiting for processing.
	items *list.List
	// itemInQueue is a map of (string) key -> item while it is in the items list
	itemsInQueue map[string]*list.Element
	// itemsBeingProcessed is a map of (string) key -> item once it has been moved
	itemsBeingProcessed map[string]*queueItem
	// Wait for next semaphore is an exclusive (1 item) lock that is taken every time items is checked to see if there
	// is an item in queue for work
	waitForNextItemSemaphore *semaphore.Weighted

	// wakeup
	wakeupCh chan struct{}

	retryFunc ShouldRetryFunc
}

type queueItem struct {
	key                    string
	plannedToStartWorkAt   time.Time
	redirtiedAt            time.Time
	redirtiedWithRatelimit bool
	forget                 bool
	requeues               int

	// Debugging information only
	originallyAdded     time.Time
	addedViaRedirty     bool
	delayedViaRateLimit *time.Duration
}

func (item *queueItem) String() string {
	return fmt.Sprintf("<plannedToStartWorkAt:%s key: %s>", item.plannedToStartWorkAt.String(), item.key)
}

// New creates a queue
//
// It expects to get a item rate limiter, and a friendly name which is used in logs, and in the internal kubernetes
// metrics. If retryFunc is nil, the default retry function.
func New(ratelimiter workqueue.RateLimiter, name string, handler ItemHandler, retryFunc ShouldRetryFunc) *Queue {
	if retryFunc == nil {
		retryFunc = DefaultRetryFunc
	}
	return &Queue{
		clock:                    clock.RealClock{},
		name:                     name,
		ratelimiter:              ratelimiter,
		items:                    list.New(),
		itemsBeingProcessed:      make(map[string]*queueItem),
		itemsInQueue:             make(map[string]*list.Element),
		handler:                  handler,
		wakeupCh:                 make(chan struct{}, 1),
		waitForNextItemSemaphore: semaphore.NewWeighted(1),
		retryFunc:                retryFunc,
	}
}

// Enqueue enqueues the key in a rate limited fashion
func (q *Queue) Enqueue(ctx context.Context, key string) {
	q.lock.Lock()
	defer q.lock.Unlock()

	q.insert(ctx, key, true, nil)
}

// EnqueueWithoutRateLimit enqueues the key without a rate limit
func (q *Queue) EnqueueWithoutRateLimit(ctx context.Context, key string) {
	q.lock.Lock()
	defer q.lock.Unlock()

	q.insert(ctx, key, false, nil)
}

// Forget forgets the key
func (q *Queue) Forget(ctx context.Context, key string) {
	q.lock.Lock()
	defer q.lock.Unlock()
	ctx, span := trace.StartSpan(ctx, "Forget")
	defer span.End()

	ctx = span.WithFields(ctx, map[string]interface{}{
		"queue": q.name,
		"key":   key,
	})

	if item, ok := q.itemsInQueue[key]; ok {
		span.WithField(ctx, "status", "itemInQueue")
		delete(q.itemsInQueue, key)
		q.items.Remove(item)
		return
	}

	if qi, ok := q.itemsBeingProcessed[key]; ok {
		span.WithField(ctx, "status", "itemBeingProcessed")
		qi.forget = true
		return
	}
	span.WithField(ctx, "status", "notfound")
}

func durationDeref(duration *time.Duration, def time.Duration) time.Duration {
	if duration == nil {
		return def
	}

	return *duration
}

// insert inserts a new item to be processed at time time. It will not further delay items if when is later than the
// original time the item was scheduled to be processed. If when is earlier, it will "bring it forward"
// If ratelimit is specified, and delay is nil, then the ratelimiter's delay (return from When function) will be used
// If ratelimit is specified, and the delay is non-nil, then the delay value will be used
// If ratelimit is false, then only delay is used to schedule the work. If delay is nil, it will be considered 0.
func (q *Queue) insert(ctx context.Context, key string, ratelimit bool, delay *time.Duration) *queueItem {
	ctx, span := trace.StartSpan(ctx, "insert")
	defer span.End()

	ctx = span.WithFields(ctx, map[string]interface{}{
		"queue":     q.name,
		"key":       key,
		"ratelimit": ratelimit,
	})
	if delay == nil {
		ctx = span.WithField(ctx, "delay", "nil")
	} else {
		ctx = span.WithField(ctx, "delay", delay.String())
	}

	defer func() {
		select {
		case q.wakeupCh <- struct{}{}:
		default:
		}
	}()

	// First see if the item is already being processed
	if item, ok := q.itemsBeingProcessed[key]; ok {
		span.WithField(ctx, "status", "itemsBeingProcessed")
		when := q.clock.Now().Add(durationDeref(delay, 0))
		// Is the item already been redirtied?
		if item.redirtiedAt.IsZero() {
			item.redirtiedAt = when
			item.redirtiedWithRatelimit = ratelimit
		} else if when.Before(item.redirtiedAt) {
			item.redirtiedAt = when
			item.redirtiedWithRatelimit = ratelimit
		}
		item.forget = false
		return item
	}

	// Is the item already in the queue?
	if item, ok := q.itemsInQueue[key]; ok {
		span.WithField(ctx, "status", "itemsInQueue")
		qi := item.Value.(*queueItem)
		when := q.clock.Now().Add(durationDeref(delay, 0))
		q.adjustPosition(qi, item, when)
		return qi
	}

	span.WithField(ctx, "status", "added")
	now := q.clock.Now()
	val := &queueItem{
		key:                  key,
		plannedToStartWorkAt: now,
		originallyAdded:      now,
	}

	if ratelimit {
		actualDelay := q.ratelimiter.When(key)
		// Check if delay is overridden
		if delay != nil {
			actualDelay = *delay
		}
		span.WithField(ctx, "delay", actualDelay.String())
		val.plannedToStartWorkAt = val.plannedToStartWorkAt.Add(actualDelay)
		val.delayedViaRateLimit = &actualDelay
	} else {
		val.plannedToStartWorkAt = val.plannedToStartWorkAt.Add(durationDeref(delay, 0))
	}

	for item := q.items.Back(); item != nil; item = item.Prev() {
		qi := item.Value.(*queueItem)
		if qi.plannedToStartWorkAt.Before(val.plannedToStartWorkAt) {
			q.itemsInQueue[key] = q.items.InsertAfter(val, item)
			return val
		}
	}

	q.itemsInQueue[key] = q.items.PushFront(val)
	return val
}

func (q *Queue) adjustPosition(qi *queueItem, element *list.Element, when time.Time) {
	if when.After(qi.plannedToStartWorkAt) {
		// The item has already been delayed appropriately
		return
	}

	qi.plannedToStartWorkAt = when
	for prev := element.Prev(); prev != nil; prev = prev.Prev() {
		item := prev.Value.(*queueItem)
		// does this item plan to start work *before* the new time? If so add it
		if item.plannedToStartWorkAt.Before(when) {
			q.items.MoveAfter(element, prev)
			return
		}
	}

	q.items.MoveToFront(element)
}

// EnqueueWithoutRateLimitWithDelay enqueues without rate limiting, but work will not start for this given delay period
func (q *Queue) EnqueueWithoutRateLimitWithDelay(ctx context.Context, key string, after time.Duration) {
	q.lock.Lock()
	defer q.lock.Unlock()
	q.insert(ctx, key, false, &after)
}

// Empty returns if the queue has no items in it
//
// It should only be used for debugging.
func (q *Queue) Empty() bool {
	return q.Len() == 0
}

// Len includes items that are in the queue, and are being processed
func (q *Queue) Len() int {
	q.lock.Lock()
	defer q.lock.Unlock()
	if q.items.Len() != len(q.itemsInQueue) {
		panic("Internally inconsistent state")
	}

	return q.items.Len() + len(q.itemsBeingProcessed)
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
		// This is required because i is referencing a mutable variable and that's running in a separate goroutine
		idx := i
		group.StartWithContext(ctx, func(ctx context.Context) {
			q.worker(ctx, idx)
		})
	}
	defer group.Wait()
	<-ctx.Done()
}

func (q *Queue) worker(ctx context.Context, i int) {
	ctx = log.WithLogger(ctx, log.G(ctx).WithFields(map[string]interface{}{
		"workerId": i,
		"queue":    q.name,
	}))
	for q.handleQueueItem(ctx) {
	}
}

func (q *Queue) getNextItem(ctx context.Context) (*queueItem, error) {
	if err := q.waitForNextItemSemaphore.Acquire(ctx, 1); err != nil {
		return nil, err
	}
	defer q.waitForNextItemSemaphore.Release(1)

	for {
		q.lock.Lock()
		element := q.items.Front()
		if element == nil {
			// Wait for the next item
			q.lock.Unlock()
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-q.wakeupCh:
			}
		} else {
			qi := element.Value.(*queueItem)
			timeUntilProcessing := time.Until(qi.plannedToStartWorkAt)

			// Do we need to sleep? If not, let's party.
			if timeUntilProcessing <= 0 {
				q.itemsBeingProcessed[qi.key] = qi
				q.items.Remove(element)
				delete(q.itemsInQueue, qi.key)
				q.lock.Unlock()
				return qi, nil
			}

			q.lock.Unlock()
			if err := func() error {
				timer := q.clock.NewTimer(timeUntilProcessing)
				defer timer.Stop()
				select {
				case <-timer.C():
				case <-ctx.Done():
					return ctx.Err()
				case <-q.wakeupCh:
				}
				return nil
			}(); err != nil {
				return nil, err
			}
		}
	}
}

// handleQueueItem handles a single item
//
// A return value of "false" indicates that further processing should be stopped.
func (q *Queue) handleQueueItem(ctx context.Context) bool {
	ctx, span := trace.StartSpan(ctx, "handleQueueItem")
	defer span.End()

	qi, err := q.getNextItem(ctx)
	if err != nil {
		span.SetStatus(err)
		return false
	}

	// We expect strings to come off the work Queue.
	// These are of the form namespace/name.
	// We do this as the delayed nature of the work Queue means the items in the informer cache may actually be more u
	// to date that when the item was initially put onto the workqueue.
	ctx = span.WithField(ctx, "key", qi.key)
	log.G(ctx).Debug("Got Queue object")

	err = q.handleQueueItemObject(ctx, qi)
	if err != nil {
		// We've actually hit an error, so we set the span's status based on the error.
		span.SetStatus(err)
		log.G(ctx).WithError(err).Error("Error processing Queue item")
		return true
	}
	log.G(ctx).Debug("Processed Queue item")

	return true
}

func (q *Queue) handleQueueItemObject(ctx context.Context, qi *queueItem) error {
	// This is a separate function / span, because the handleQueueItem span is the time spent waiting for the object
	// plus the time spend handling the object. Instead, this function / span is scoped to a single object.
	ctx, span := trace.StartSpan(ctx, "handleQueueItemObject")
	defer span.End()

	ctx = span.WithFields(ctx, map[string]interface{}{
		"requeues":        qi.requeues,
		"originallyAdded": qi.originallyAdded.String(),
		"addedViaRedirty": qi.addedViaRedirty,
		"plannedForWork":  qi.plannedToStartWorkAt.String(),
	})

	if qi.delayedViaRateLimit != nil {
		ctx = span.WithField(ctx, "delayedViaRateLimit", qi.delayedViaRateLimit.String())
	}

	// Add the current key as an attribute to the current span.
	ctx = span.WithField(ctx, "key", qi.key)
	// Run the syncHandler, passing it the namespace/name string of the Pod resource to be synced.
	err := q.handler(ctx, qi.key)

	q.lock.Lock()
	defer q.lock.Unlock()

	delete(q.itemsBeingProcessed, qi.key)
	if qi.forget {
		q.ratelimiter.Forget(qi.key)
		log.G(ctx).WithError(err).Warnf("forgetting %q as told to forget while in progress", qi.key)
		return nil
	}

	if err != nil {
		ctx = span.WithField(ctx, "error", err.Error())
		var delay *time.Duration

		// Stash the original error for logging below
		originalError := err
		delay, err = q.retryFunc(ctx, qi.key, qi.requeues+1, qi.originallyAdded, err)
		if err == nil {
			// Put the item back on the work Queue to handle any transient errors.
			log.G(ctx).WithError(originalError).Warnf("requeuing %q due to failed sync", qi.key)
			newQI := q.insert(ctx, qi.key, true, delay)
			newQI.requeues = qi.requeues + 1
			newQI.originallyAdded = qi.originallyAdded

			return nil
		}
		if !qi.redirtiedAt.IsZero() {
			err = fmt.Errorf("temporarily (requeued) forgetting %q due to: %w", qi.key, err)
		} else {
			err = fmt.Errorf("forgetting %q due to: %w", qi.key, err)
		}
	}

	// We've exceeded the maximum retries or we were successful.
	q.ratelimiter.Forget(qi.key)
	if !qi.redirtiedAt.IsZero() {
		delay := time.Until(qi.redirtiedAt)
		newQI := q.insert(ctx, qi.key, qi.redirtiedWithRatelimit, &delay)
		newQI.addedViaRedirty = true
	}

	span.SetStatus(err)
	return err
}

func (q *Queue) String() string {
	q.lock.Lock()
	defer q.lock.Unlock()

	items := make([]string, 0, q.items.Len())

	for next := q.items.Front(); next != nil; next = next.Next() {
		items = append(items, next.Value.(*queueItem).String())
	}
	return fmt.Sprintf("<items:%s>", items)
}

// DefaultRetryFunc is the default function used for retries by the queue subsystem.
func DefaultRetryFunc(ctx context.Context, key string, timesTried int, originallyAdded time.Time, err error) (*time.Duration, error) {
	if timesTried < MaxRetries {
		return nil, nil
	}

	return nil, pkgerrors.Wrapf(err, "maximum retries (%d) reached", MaxRetries)
}
