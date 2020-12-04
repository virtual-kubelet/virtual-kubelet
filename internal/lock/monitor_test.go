package lock

import (
	"sync"
	"testing"
	"time"

	"golang.org/x/sync/errgroup"
	"k8s.io/apimachinery/pkg/util/sets"

	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
)

func TestMonitorUninitialized(t *testing.T) {
	t.Parallel()
	mv := NewMonitorVariable()
	subscription := mv.Subscribe()
	select {
	case <-subscription.NewValueReady():
		t.Fatalf("Received value update message: %v", subscription.Value())
	case <-time.After(time.Second):
	}
}

func TestGetUninitialized(t *testing.T) {
	mv := NewMonitorVariable()
	subscription := mv.Subscribe()
	val := subscription.Value()
	assert.Assert(t, is.Equal(val.Version, int64(0)))
}

func TestMonitorSetInitialVersionAfterListen(t *testing.T) {
	mv := NewMonitorVariable()
	subscription := mv.Subscribe()
	go mv.Set("test")
	<-subscription.NewValueReady()
	assert.Assert(t, is.Equal(subscription.Value().Value, "test"))
}

func TestMonitorSetInitialVersionBeforeListen(t *testing.T) {
	mv := NewMonitorVariable()
	subscription := mv.Subscribe()
	mv.Set("test")
	<-subscription.NewValueReady()
	assert.Assert(t, is.Equal(subscription.Value().Value, "test"))
}

func TestMonitorMultipleVersionsBlock(t *testing.T) {
	t.Parallel()
	mv := NewMonitorVariable()
	subscription := mv.Subscribe()
	mv.Set("test")
	<-subscription.NewValueReady()
	/* This should mark the "current" version as seen */
	val := subscription.Value()
	assert.Assert(t, is.Equal(val.Version, int64(1)))
	select {
	case <-subscription.NewValueReady():
		t.Fatalf("Received value update message: %v", subscription.Value())
	case <-time.After(time.Second):
	}
}
func TestMonitorMultipleVersions(t *testing.T) {
	t.Parallel()
	lock := sync.Mutex{}
	lock.Lock()
	mv := NewMonitorVariable()
	triggers := []int{}
	ch := make(chan struct{}, 10)
	go func() {
		defer lock.Unlock()
		subscription := mv.Subscribe()
		for {
			// Lint is wrong, we need to call the function each time to get a fresh channel.
			<-subscription.NewValueReady()
			val := subscription.Value()
			triggers = append(triggers, val.Value.(int))
			ch <- struct{}{}
			if val.Value == 9 {
				return
			}
		}
	}()

	for i := 0; i < 10; i++ {
		mv.Set(i)
		// Wait for the trigger to occur
		<-ch
	}

	// Wait for the goroutine to finish
	lock.Lock()
	t.Logf("Saw %v triggers", triggers)
	assert.Assert(t, is.Len(triggers, 10))
	// Make sure we saw all 10 unique values
	assert.Assert(t, is.Equal(sets.NewInt(triggers...).Len(), 10))
}
func TestMonitorMultipleSubscribers(t *testing.T) {
	group := &errgroup.Group{}
	mv := NewMonitorVariable()
	for i := 0; i < 10; i++ {
		sub := mv.Subscribe()
		group.Go(func() error {
			<-sub.NewValueReady()
			return nil
		})
	}
	mv.Set(1)
	_ = group.Wait()
}
