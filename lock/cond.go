package lock

import (
	"container/list"
	"context"
	"sync"

	"golang.org/x/sync/semaphore"
)

type Ticket interface {
	// Release releases the ticket. The ticket must not be released multiple times.
	Release()
	// Wait waits for someone to wakeup the context. It is not concurrency safe [in that multiple goroutines cannot
	// wait on the same ticket]. If the condition is woken up, no error is returned, the ticket remains valid.
	// The ticket now holds an exclusive lock on the condition, and it must be released for others to
	// make progress.
	// Otherwise, ctx.Err() is returned by Wait, and the ticket is rendered invalid.
	Wait(context.Context) error

	// Waiter returns a waiter object. The underlying ticket is released upon this call, and cannot be reused.
	Waiter() Waiter
}

type Cond interface {
	// Acquire acquires the lock, blocking until resources are available or ctx is done.
	// On success, returns nil error. On failure nil and ctx.Err() are returned, leaving
	// the lock unchanged.
	Acquire(ctx context.Context) (Ticket, error)

	// Broadcast wakes all goroutines waiting to acquire the lock. You do not need to have a ticket to broadcast.
	Broadcast()

	// Signal wakes one goroutine waiting to acquire the lock. You do not need to have a ticket to signal.
	Signal()
}

type cond struct {
	semaphore *semaphore.Weighted

	waiterLocks sync.Mutex
	waiters     *list.List
}

type ticket struct {
	c        *cond
	unlocked bool
}

func (t *ticket) Release() {
	if t.unlocked {
		panic("Ticket has been consumed, and must not be reused")
	}
	t.unlocked = true
	t.c.semaphore.Release(1)
}

// Waiter is an alternative interface to the lock condition. Once channel closes
type Waiter interface {
	// Abandon() must be called if the intent is to stop waiting prior to channel being closed. If the Waiter was
	// triggered, it will return true, otherwise false. Otherwise the waiter is leaked. Abandon is idempotent, and
	// concurrency safe.
	Abandon() bool
	// Channel is an idempotent call to return the waiter channel. The channel is closed upon a signal. When the channel
	// is closed, the lock is not reacquired.
	Channel() <-chan struct{}
}

type waiter struct {
	ch      chan struct{}
	element *list.Element
	c       *cond
}

func (w *waiter) Abandon() bool {
	w.c.waiterLocks.Lock()
	// remove is idempotent, so no need to check if it's still part of the list
	w.c.waiters.Remove(w.element)
	w.c.waiterLocks.Unlock()
	select {
	case <-w.ch:
		return true
	default:
		return false
	}
}

func (w *waiter) Channel() <-chan struct{} {
	return w.ch
}

func (t *ticket) Waiter() Waiter {
	waiter := &waiter{
		ch: make(chan struct{}),
		c:  t.c,
	}
	t.c.waiterLocks.Lock()
	waiter.element = t.c.waiters.PushBack(waiter)
	t.c.waiterLocks.Unlock()
	t.Release()

	return waiter
}

func (t *ticket) Wait(ctx context.Context) error {
	waiter := t.Waiter()
	select {
	case <-ctx.Done():
		waiter.Abandon()
		return ctx.Err()
	case <-waiter.Channel():
		// If this is closed, it means that the item has been removed from the list, and we should move to the next
		// step of trying to acquire the semaphore
		t2, err := t.c.acquire(ctx)
		if err != nil {
			return err
		}
		*t = *t2
		return nil
	}
}

func (c *cond) Acquire(ctx context.Context) (Ticket, error) {
	return c.acquire(ctx)
}

func (c *cond) acquire(ctx context.Context) (*ticket, error) {
	err := c.semaphore.Acquire(ctx, 1)
	if err != nil {
		return nil, err
	}

	return &ticket{c: c}, err
}

func (c *cond) Broadcast() {
	c.waiterLocks.Lock()
	element := c.waiters.Front()
	for element != nil {
		c.waiters.Remove(element)
		close(element.Value.(*waiter).ch)
		element = c.waiters.Front()
	}
	c.waiterLocks.Unlock()
}

func (c *cond) Signal() {
	c.waiterLocks.Lock()
	element := c.waiters.Front()
	if element != nil {
		c.waiters.Remove(element)
		close(element.Value.(*waiter).ch)
	}
	c.waiterLocks.Unlock()
}

func NewCond() Cond {
	return &cond{
		semaphore: semaphore.NewWeighted(1),
		waiters:   list.New(),
	}
}
