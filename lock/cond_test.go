package lock_test

import (
	"context"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/virtual-kubelet/virtual-kubelet/lock"
	"golang.org/x/sync/errgroup"
	"gotest.tools/assert"
	"gotest.tools/assert/cmp"
)

func TestCondWakeupEmpty(t *testing.T) {
	c := lock.NewCond()
	c.Broadcast()
	c.Signal()
}

func TestBasicLock(t *testing.T) {
	c := lock.NewCond()
	ctx := context.Background()
	ticket, err := c.Acquire(ctx)
	assert.NilError(t, err)
	ticket.Release()
	ticket, err = c.Acquire(ctx)
	assert.NilError(t, err)
	ticket.Release()
}

func TestPanics(t *testing.T) {
	c := lock.NewCond()
	ctx := context.Background()
	assert.Check(t, cmp.Panics(func() {
		ticket, err := c.Acquire(ctx)
		assert.NilError(t, err)
		ticket.Release()
		ticket.Release()
	}))
}

func TestContextCancel(t *testing.T) {
	c := lock.NewCond()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	group, ctx := errgroup.WithContext(ctx)
	wg := &sync.WaitGroup{}
	wg.Add(100)
	for i := 0; i < 100; i++ {
		group.Go(func() error {
			ticket, err := c.Acquire(ctx)
			if err != nil {
				return err
			}
			wg.Done()
			if ticket.Wait(ctx) != context.Canceled {
				return err
			}
			return nil
		})
	}
	// Wait for everyone to move to the waiting state.
	wg.Wait()
	cancel()
	assert.NilError(t, group.Wait())
}

func TestTicketReuse(t *testing.T) {
	c := lock.NewCond()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ticket, err := c.Acquire(ctx)
	assert.NilError(t, err)
	ch := make(chan error)
	go func() {
		ch <- ticket.Wait(ctx)
	}()
	ticket2, err := c.Acquire(ctx)
	assert.NilError(t, err)
	c.Signal()
	ticket2.Release()
	assert.NilError(t, <-ch)

	go func() {
		ch <- ticket.Wait(ctx)
	}()
	ticket2, err = c.Acquire(ctx)
	assert.NilError(t, err)
	c.Signal()
	ticket2.Release()
	assert.NilError(t, <-ch)

	// Make sure once the ticket goes "bad", it can't be reused
	go func() {
		ch <- ticket.Wait(ctx)
	}()
	cancel()
	assert.Error(t, <-ch, context.Canceled.Error())

	assert.Check(t, cmp.Panics(func() {
		_ = ticket.Wait(context.Background())
	}))

	// Make sure the lock was released by the last failed try to acquire
	_, err = c.Acquire(context.Background())
	assert.NilError(t, err)
}

// Much of the following is borrowed from:
// https://golang.org/src/sync/cond_test.go
func TestCondSignal(t *testing.T) {
	ctx := context.Background()
	c := lock.NewCond()
	n := 2
	running := make(chan bool, n)
	awake := make(chan bool, n)
	for i := 0; i < n; i++ {
		go func() {
			ticket, err := c.Acquire(ctx)
			assert.NilError(t, err)
			running <- true
			assert.NilError(t, ticket.Wait(ctx))
			awake <- true
			ticket.Release()
		}()
	}
	for i := 0; i < n; i++ {
		<-running // Wait for everyone to run.
	}
	for n > 0 {
		select {
		case <-awake:
			t.Fatal("goroutine not asleep")
		default:
		}
		ticket, err := c.Acquire(ctx)
		assert.NilError(t, err)
		c.Signal()
		ticket.Release()
		<-awake // Will deadlock if no goroutine wakes up
		select {
		case <-awake:
			t.Fatal("too many goroutines awake")
		default:
		}
		n--
	}
	c.Signal()
}

func TestCondSignalGenerations(t *testing.T) {
	ctx := context.Background()
	c := lock.NewCond()
	n := 100
	running := make(chan bool, n)
	awake := make(chan int, n)
	for i := 0; i < n; i++ {
		go func(i int) {
			ticket, err := c.Acquire(ctx)
			assert.NilError(t, err)
			running <- true
			assert.NilError(t, ticket.Wait(ctx))
			awake <- i
			ticket.Release()
		}(i)
		if i > 0 {
			a := <-awake
			if a != i-1 {
				t.Fatalf("wrong goroutine woke up: want %d, got %d", i-1, a)
			}
		}
		<-running
		ticket, err := c.Acquire(ctx)
		assert.NilError(t, err)
		c.Signal()
		ticket.Release()
	}
}

func TestCondBroadcast(t *testing.T) {
	ctx := context.Background()
	c := lock.NewCond()
	n := 200
	running := make(chan int, n)
	awake := make(chan int, n)
	exit := false
	for i := 0; i < n; i++ {
		go func(g int) {
			ticket, err := c.Acquire(ctx)
			assert.NilError(t, err)
			for !exit {
				running <- g
				assert.NilError(t, ticket.Wait(ctx))
				awake <- g
			}
			ticket.Release()
		}(i)
	}
	for i := 0; i < n; i++ {
		for i := 0; i < n; i++ {
			<-running // Will deadlock unless n are running.
		}
		if i == n-1 {
			ticket, err := c.Acquire(ctx)
			assert.NilError(t, err)
			exit = true
			ticket.Release()
		}
		select {
		case <-awake:
			t.Fatal("goroutine not asleep")
		default:
		}
		ticket, err := c.Acquire(ctx)
		assert.NilError(t, err)
		c.Broadcast()
		ticket.Release()
		seen := make([]bool, n)
		for i := 0; i < n; i++ {
			g := <-awake
			if seen[g] {
				t.Fatal("goroutine woke up twice")
			}
			seen[g] = true
		}
	}
	select {
	case <-running:
		t.Fatal("goroutine did not exit")
	default:
	}
	c.Broadcast()
}

func TestRace(t *testing.T) {
	x := 0
	ctx := context.Background()
	c := lock.NewCond()
	done := make(chan bool)
	go func() {
		ticket, err := c.Acquire(ctx)
		assert.NilError(t, err)
		x = 1
		assert.NilError(t, ticket.Wait(ctx))
		if x != 2 {
			t.Error("want 2")
		}
		x = 3
		c.Signal()
		ticket.Release()
		done <- true
	}()
	go func() {
		ticket, err := c.Acquire(ctx)
		assert.NilError(t, err)
		for {
			if x == 1 {
				x = 2
				c.Signal()
				break
			}
			ticket.Release()
			runtime.Gosched()
			ticket, err = c.Acquire(ctx)
			assert.NilError(t, err)
		}
		ticket.Release()
		done <- true
	}()
	go func() {
		ticket, err := c.Acquire(ctx)
		assert.NilError(t, err)
		for {
			if x == 2 {
				assert.NilError(t, ticket.Wait(ctx))
				if x != 3 {
					t.Error("want 3")
				}
				break
			}
			if x == 3 {
				break
			}
			ticket.Release()
			runtime.Gosched()
			ticket, err = c.Acquire(ctx)
			assert.NilError(t, err)
		}
		ticket.Release()
		done <- true
	}()
	<-done
	<-done
	<-done
}

func TestCondSignalStealing(t *testing.T) {
	for iters := 0; iters < 1000; iters++ {
		ctx := context.Background()
		c := lock.NewCond()

		// Start a waiter.
		ch := make(chan struct{})
		go func() {
			ticket, err := c.Acquire(ctx)
			assert.NilError(t, err)
			ch <- struct{}{}
			assert.NilError(t, ticket.Wait(ctx))
			ticket.Release()

			ch <- struct{}{}
		}()

		<-ch
		ticket, err := c.Acquire(ctx)
		assert.NilError(t, err)
		ticket.Release()

		// We know that the waiter is in the cond.Wait() call because we
		// synchronized with it, then acquired/released the mutex it was
		// holding when we synchronized.
		//
		// Start two goroutines that will race: one will broadcast on
		// the cond var, the other will wait on it.
		//
		// The new waiter may or may not get notified, but the first one
		// has to be notified.
		done := false
		go func() {
			c.Broadcast()
		}()

		go func() {
			ticket, err := c.Acquire(ctx)
			assert.NilError(t, err)
			for !done {
				assert.NilError(t, ticket.Wait(ctx))
			}
			ticket.Release()
		}()

		// Check that the first waiter does get signaled.
		select {
		case <-ch:
		case <-time.After(2 * time.Second):
			t.Fatalf("First waiter didn't get broadcast.")
		}

		// Release the second waiter in case it didn't get the
		// broadcast.
		ticket, err = c.Acquire(ctx)
		assert.NilError(t, err)
		done = true
		ticket.Release()
		c.Broadcast()
	}
}
