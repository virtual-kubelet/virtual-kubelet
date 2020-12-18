package queue

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/virtual-kubelet/virtual-kubelet/log"
	logruslogger "github.com/virtual-kubelet/virtual-kubelet/log/logrus"
	"go.uber.org/goleak"
	"golang.org/x/time/rate"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/workqueue"
)

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
	), t.Name(), handler)
	wq.Enqueue("test")

	for n < MaxRetries {
		assert.Assert(t, wq.handleQueueItem(ctx))
	}

	assert.Assert(t, is.Equal(n, MaxRetries))
	assert.Assert(t, is.Equal(0, wq.workqueue.Len()))
}

func TestForget(t *testing.T) {
	t.Parallel()
	handler := func(ctx context.Context, key string) error {
		panic("Should never be called")
	}
	wq := New(workqueue.DefaultItemBasedRateLimiter(), t.Name(), handler)

	wq.Forget("val")
	assert.Assert(t, is.Equal(0, wq.workqueue.Len()))

	v := "test"
	wq.EnqueueWithoutRateLimit(v)
	assert.Assert(t, is.Equal(1, wq.workqueue.Len()))

	t.Skip("This is broken")
	// Workqueue docs:
	// Forget indicates that an item is finished being retried.  Doesn't matter whether it's for perm failing
	// or for success, we'll stop the rate limiter from tracking it.  This only clears the `rateLimiter`, you
	// still have to call `Done` on the queue.
	// Even if you do this, it doesn't work: https://play.golang.com/p/8vfL_RCsFGI
	assert.Assert(t, is.Equal(0, wq.workqueue.Len()))

}

func TestQueueTerminate(t *testing.T) {
	t.Parallel()
	defer goleak.VerifyNone(t,
		// Ignore existing goroutines
		goleak.IgnoreCurrent(),
		// Ignore klog background flushers
		goleak.IgnoreTopFunction("k8s.io/klog.(*loggingT).flushDaemon"),
		goleak.IgnoreTopFunction("k8s.io/klog/v2.(*loggingT).flushDaemon"),
		// Workqueue runs a goroutine in the background to handle background functions. AFAICT, they're unkillable
		// and are designed to stop after a certain idle window
		goleak.IgnoreTopFunction("k8s.io/client-go/util/workqueue.(*Type).updateUnfinishedWorkLoop"),
		goleak.IgnoreTopFunction("k8s.io/client-go/util/workqueue.(*delayingType).waitingLoop"),
	)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	testMap := &sync.Map{}
	handler := func(ctx context.Context, key string) error {
		testMap.Store(key, struct{}{})
		return nil
	}

	wq := New(workqueue.DefaultItemBasedRateLimiter(), t.Name(), handler)
	group := &wait.Group{}
	group.StartWithContext(ctx, func(ctx context.Context) {
		wq.Run(ctx, 10)
	})
	for i := 0; i < 1000; i++ {
		wq.EnqueueWithoutRateLimit(strconv.Itoa(i))
	}

	for wq.workqueue.Len() > 0 {
		time.Sleep(100 * time.Millisecond)
	}

	for i := 0; i < 1000; i++ {
		_, ok := testMap.Load(strconv.Itoa(i))
		assert.Assert(t, ok, "Item %d missing", i)
	}

	cancel()
	group.Wait()
}
