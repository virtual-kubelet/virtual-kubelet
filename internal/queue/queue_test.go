package queue

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/virtual-kubelet/virtual-kubelet/log"
	logruslogger "github.com/virtual-kubelet/virtual-kubelet/log/logrus"
	"golang.org/x/time/rate"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/clock"
)

func durationPtr(d time.Duration) *time.Duration {
	return &d
}

func TestQueueMaxRetries(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	ctx = log.WithLogger(ctx, logruslogger.FromLogrus(logrus.NewEntry(logger)))
	n := 0
	knownErr := errors.New("Testing error")
	handler := func(ctx context.Context, key string) error {
		n++
		return knownErr
	}
	wq := New(workqueue.NewMaxOfRateLimiter(
		// The default upper bound is 1000 seconds. Let's not use that.
		workqueue.NewItemExponentialFailureRateLimiter(5*time.Millisecond, 10*time.Millisecond),
		&workqueue.BucketRateLimiter{Limiter: rate.NewLimiter(rate.Limit(10), 100)},
	), t.Name(), handler, nil)
	wq.Enqueue(context.TODO(), "test")

	for n < MaxRetries {
		assert.Assert(t, wq.handleQueueItem(ctx))
	}

	assert.Assert(t, is.Equal(n, MaxRetries))
	assert.Assert(t, is.Equal(0, wq.Len()))
}

func TestQueueCustomRetries(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	ctx = log.WithLogger(ctx, logruslogger.FromLogrus(logrus.NewEntry(logger)))
	n := 0
	errorSeen := 0
	retryTestError := errors.New("Error should be retried every 10 milliseconds")
	handler := func(ctx context.Context, key string) error {
		if key == "retrytest" {
			n++
			return retryTestError
		}
		return errors.New("Unknown error")
	}

	shouldRetryFunc := func(ctx context.Context, key string, timesTried int, originallyAdded time.Time, err error) (*time.Duration, error) {
		var sleepTime *time.Duration
		if errors.Is(err, retryTestError) {
			errorSeen++
			sleepTime = durationPtr(10 * time.Millisecond)
		}
		_, retErr := DefaultRetryFunc(ctx, key, timesTried, originallyAdded, err)
		return sleepTime, retErr
	}

	wq := New(&workqueue.BucketRateLimiter{Limiter: rate.NewLimiter(rate.Limit(1000), 1000)}, t.Name(), handler, shouldRetryFunc)

	timeTaken := func(key string) time.Duration {
		start := time.Now()
		wq.Enqueue(context.TODO(), key)
		for i := 0; i < MaxRetries; i++ {
			assert.Assert(t, wq.handleQueueItem(ctx))
		}
		return time.Since(start)
	}

	unknownTime := timeTaken("unknown")
	assert.Assert(t, n == 0)
	assert.Assert(t, unknownTime < 10*time.Millisecond)

	retrytestTime := timeTaken("retrytest")
	assert.Assert(t, is.Equal(n, MaxRetries))
	assert.Assert(t, is.Equal(errorSeen, MaxRetries))

	assert.Assert(t, is.Equal(0, wq.Len()))
	assert.Assert(t, retrytestTime > 10*time.Millisecond*time.Duration(n-1))
	assert.Assert(t, retrytestTime < 2*10*time.Millisecond*time.Duration(n-1))
}

func TestForget(t *testing.T) {
	t.Parallel()
	handler := func(ctx context.Context, key string) error {
		panic("Should never be called")
	}
	wq := New(workqueue.DefaultItemBasedRateLimiter(), t.Name(), handler, nil)

	wq.Forget(context.TODO(), "val")
	assert.Assert(t, is.Equal(0, wq.Len()))

	v := "test"
	wq.EnqueueWithoutRateLimit(context.TODO(), v)
	assert.Assert(t, is.Equal(1, wq.Len()))
}

func TestQueueEmpty(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	q := New(workqueue.DefaultItemBasedRateLimiter(), t.Name(), func(ctx context.Context, key string) error {
		return nil
	}, nil)

	item, err := q.getNextItem(ctx)
	assert.Error(t, err, context.DeadlineExceeded.Error())
	assert.Assert(t, is.Nil(item))
}

func TestQueueItemNoSleep(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 1000*time.Millisecond)
	defer cancel()

	q := New(workqueue.DefaultItemBasedRateLimiter(), t.Name(), func(ctx context.Context, key string) error {
		return nil
	}, nil)

	q.lock.Lock()
	q.insert(ctx, "foo", false, durationPtr(-1*time.Hour))
	q.insert(ctx, "bar", false, durationPtr(-1*time.Hour))
	q.lock.Unlock()

	item, err := q.getNextItem(ctx)
	assert.NilError(t, err)
	assert.Assert(t, is.Equal(item.key, "foo"))

	item, err = q.getNextItem(ctx)
	assert.NilError(t, err)
	assert.Assert(t, is.Equal(item.key, "bar"))
}

func TestQueueItemSleep(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 1000*time.Millisecond)
	defer cancel()

	q := New(workqueue.DefaultItemBasedRateLimiter(), t.Name(), func(ctx context.Context, key string) error {
		return nil
	}, nil)
	q.lock.Lock()
	q.insert(ctx, "foo", false, durationPtr(100*time.Millisecond))
	q.insert(ctx, "bar", false, durationPtr(100*time.Millisecond))
	q.lock.Unlock()

	item, err := q.getNextItem(ctx)
	assert.NilError(t, err)
	assert.Assert(t, is.Equal(item.key, "foo"))
}

func TestQueueBackgroundAdd(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 5000*time.Millisecond)
	defer cancel()

	q := New(workqueue.DefaultItemBasedRateLimiter(), t.Name(), func(ctx context.Context, key string) error {
		return nil
	}, nil)
	start := time.Now()
	time.AfterFunc(100*time.Millisecond, func() {
		q.lock.Lock()
		defer q.lock.Unlock()
		q.insert(ctx, "foo", false, nil)
	})

	item, err := q.getNextItem(ctx)
	assert.NilError(t, err)
	assert.Assert(t, is.Equal(item.key, "foo"))
	assert.Assert(t, time.Since(start) > 100*time.Millisecond)
}

func TestQueueBackgroundAdvance(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 5000*time.Millisecond)
	defer cancel()

	q := New(workqueue.DefaultItemBasedRateLimiter(), t.Name(), func(ctx context.Context, key string) error {
		return nil
	}, nil)
	start := time.Now()
	q.lock.Lock()
	q.insert(ctx, "foo", false, durationPtr(10*time.Second))
	q.lock.Unlock()

	time.AfterFunc(200*time.Millisecond, func() {
		q.lock.Lock()
		defer q.lock.Unlock()
		q.insert(ctx, "foo", false, nil)
	})

	item, err := q.getNextItem(ctx)
	assert.NilError(t, err)
	assert.Assert(t, is.Equal(item.key, "foo"))
	assert.Assert(t, time.Since(start) > 200*time.Millisecond)
	assert.Assert(t, time.Since(start) < 5*time.Second)
}

func TestQueueRedirty(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 5000*time.Millisecond)
	defer cancel()

	var times int64
	var q *Queue
	q = New(workqueue.DefaultItemBasedRateLimiter(), t.Name(), func(ctx context.Context, key string) error {
		assert.Assert(t, is.Equal(key, "foo"))
		if atomic.AddInt64(&times, 1) == 1 {
			q.EnqueueWithoutRateLimit(context.TODO(), "foo")
		} else {
			cancel()
		}
		return nil
	}, nil)

	q.EnqueueWithoutRateLimit(context.TODO(), "foo")
	q.Run(ctx, 1)
	for !q.Empty() {
		time.Sleep(100 * time.Millisecond)
	}
	assert.Assert(t, is.Equal(atomic.LoadInt64(&times), int64(2)))
}

func TestHeapConcurrency(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 5000*time.Millisecond)
	defer cancel()

	start := time.Now()
	seen := sync.Map{}
	q := New(workqueue.DefaultItemBasedRateLimiter(), t.Name(), func(ctx context.Context, key string) error {
		seen.Store(key, struct{}{})
		time.Sleep(time.Second)
		return nil
	}, nil)
	for i := 0; i < 20; i++ {
		q.EnqueueWithoutRateLimit(context.TODO(), strconv.Itoa(i))
	}

	assert.Assert(t, q.Len() == 20)

	go q.Run(ctx, 20)
	for q.Len() > 0 {
		time.Sleep(100 * time.Millisecond)
	}

	for i := 0; i < 20; i++ {
		_, ok := seen.Load(strconv.Itoa(i))
		assert.Assert(t, ok, "Did not observe: %d", i)
	}
	assert.Assert(t, time.Since(start) < 5*time.Second)
}

func checkConsistency(t *testing.T, q *Queue) {
	q.lock.Lock()
	defer q.lock.Unlock()

	for next := q.items.Front(); next != nil && next.Next() != nil; next = next.Next() {
		qi := next.Value.(*queueItem)
		qiNext := next.Next().Value.(*queueItem)
		assert.Assert(t, qi.plannedToStartWorkAt.Before(qiNext.plannedToStartWorkAt) || qi.plannedToStartWorkAt.Equal(qiNext.plannedToStartWorkAt))
	}
}

func TestHeapOrder(t *testing.T) {
	q := New(workqueue.DefaultItemBasedRateLimiter(), t.Name(), func(ctx context.Context, key string) error {
		return nil
	}, nil)
	q.clock = nonmovingClock{}

	q.EnqueueWithoutRateLimitWithDelay(context.TODO(), "a", 1000)
	q.EnqueueWithoutRateLimitWithDelay(context.TODO(), "b", 2000)
	q.EnqueueWithoutRateLimitWithDelay(context.TODO(), "c", 3000)
	q.EnqueueWithoutRateLimitWithDelay(context.TODO(), "d", 4000)
	q.EnqueueWithoutRateLimitWithDelay(context.TODO(), "e", 5000)
	checkConsistency(t, q)
	t.Logf("%v", q)
	q.EnqueueWithoutRateLimitWithDelay(context.TODO(), "d", 1000)
	checkConsistency(t, q)
	t.Logf("%v", q)
	q.EnqueueWithoutRateLimitWithDelay(context.TODO(), "c", 1001)
	checkConsistency(t, q)
	t.Logf("%v", q)
	q.EnqueueWithoutRateLimitWithDelay(context.TODO(), "e", 999)
	checkConsistency(t, q)
	t.Logf("%v", q)
}

type rateLimitWrapper struct {
	addedMap     sync.Map
	forgottenMap sync.Map
	rl           workqueue.RateLimiter
}

func (r *rateLimitWrapper) When(item interface{}) time.Duration {
	if _, ok := r.forgottenMap.Load(item); ok {
		r.forgottenMap.Delete(item)
		// Reset the added map
		r.addedMap.Store(item, 1)
	} else {
		actual, loaded := r.addedMap.LoadOrStore(item, 1)
		if loaded {
			r.addedMap.Store(item, actual.(int)+1)
		}
	}

	return r.rl.When(item)
}

func (r *rateLimitWrapper) Forget(item interface{}) {
	r.forgottenMap.Store(item, struct{}{})
	r.rl.Forget(item)
}

func (r *rateLimitWrapper) NumRequeues(item interface{}) int {
	return r.rl.NumRequeues(item)
}

func TestRateLimiter(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5000*time.Millisecond)
	defer cancel()

	syncMap := sync.Map{}
	syncMap.Store("foo", 0)
	syncMap.Store("bar", 0)
	syncMap.Store("baz", 0)
	syncMap.Store("quux", 0)

	start := time.Now()
	ratelimiter := &rateLimitWrapper{
		rl: workqueue.NewItemFastSlowRateLimiter(1*time.Millisecond, 100*time.Millisecond, 1),
	}

	q := New(ratelimiter, t.Name(), func(ctx context.Context, key string) error {
		oldValue, _ := syncMap.Load(key)
		syncMap.Store(key, oldValue.(int)+1)
		if oldValue.(int) < 9 {
			return errors.New("test")
		}
		return nil
	}, nil)

	enqueued := 0
	syncMap.Range(func(key, value interface{}) bool {
		enqueued++
		q.Enqueue(context.TODO(), key.(string))
		return true
	})

	assert.Assert(t, enqueued == 4)
	go q.Run(ctx, 10)

	incomplete := true
	for incomplete {
		time.Sleep(10 * time.Millisecond)
		incomplete = false
		// Wait for all items to finish processing.
		syncMap.Range(func(key, value interface{}) bool {
			if value.(int) < 10 {
				incomplete = true
			}
			return true
		})
	}

	// Make sure there were ~9 "slow" rate limits per item, and 1 fast
	assert.Assert(t, time.Since(start) > 9*100*time.Millisecond)
	// Make sure we didn't go off the deep end.
	assert.Assert(t, time.Since(start) < 2*9*100*time.Millisecond)

	// Make sure each item was seen. And Forgotten.
	syncMap.Range(func(key, value interface{}) bool {
		_, ok := ratelimiter.forgottenMap.Load(key)
		assert.Assert(t, ok, "%s in forgotten map", key)
		val, ok := ratelimiter.addedMap.Load(key)
		assert.Assert(t, ok, "%s in added map", key)
		assert.Assert(t, val == 10)
		return true
	})

	q.lock.Lock()
	defer q.lock.Unlock()
	assert.Assert(t, len(q.itemsInQueue) == 0)
	assert.Assert(t, len(q.itemsBeingProcessed) == 0)
	assert.Assert(t, q.items.Len() == 0)

}

func TestQueueForgetInProgress(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var times int64
	var q *Queue
	q = New(workqueue.DefaultItemBasedRateLimiter(), t.Name(), func(ctx context.Context, key string) error {
		assert.Assert(t, is.Equal(key, "foo"))
		atomic.AddInt64(&times, 1)
		q.Forget(context.TODO(), key)
		return errors.New("test")
	}, nil)

	q.EnqueueWithoutRateLimit(context.TODO(), "foo")
	go q.Run(ctx, 1)
	for !q.Empty() {
		time.Sleep(100 * time.Millisecond)
	}
	assert.Assert(t, is.Equal(atomic.LoadInt64(&times), int64(1)))
}

func TestQueueForgetBeforeStart(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	q := New(workqueue.DefaultItemBasedRateLimiter(), t.Name(), func(ctx context.Context, key string) error {
		panic("shouldn't be called")
	}, nil)

	q.EnqueueWithoutRateLimit(context.TODO(), "foo")
	q.Forget(context.TODO(), "foo")
	go q.Run(ctx, 1)
	for !q.Empty() {
		time.Sleep(100 * time.Millisecond)
	}
}

func TestQueueMoveItem(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	q := New(workqueue.DefaultItemBasedRateLimiter(), t.Name(), func(ctx context.Context, key string) error {
		panic("shouldn't be called")
	}, nil)
	q.clock = nonmovingClock{}

	q.insert(ctx, "foo", false, durationPtr(3000))
	q.insert(ctx, "bar", false, durationPtr(2000))
	q.insert(ctx, "baz", false, durationPtr(1000))
	checkConsistency(t, q)
	t.Log(q)

	q.insert(ctx, "foo", false, durationPtr(2000))
	checkConsistency(t, q)
	t.Log(q)

	q.insert(ctx, "foo", false, durationPtr(1999))
	checkConsistency(t, q)
	t.Log(q)

	q.insert(ctx, "foo", false, durationPtr(999))
	checkConsistency(t, q)
	t.Log(q)
}

type nonmovingClock struct {
}

func (n nonmovingClock) Now() time.Time {
	return time.Time{}
}

func (n nonmovingClock) Since(t time.Time) time.Duration {
	return n.Now().Sub(t)
}

func (n nonmovingClock) After(d time.Duration) <-chan time.Time {
	panic("implement me")
}

func (n nonmovingClock) NewTimer(d time.Duration) clock.Timer {
	panic("implement me")
}

func (n nonmovingClock) Sleep(d time.Duration) {
	panic("implement me")
}

func (n nonmovingClock) Tick(d time.Duration) <-chan time.Time {
	panic("implement me")
}
