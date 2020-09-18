package lock

import (
	"container/list"
	"context"
	"sync"

	"golang.org/x/sync/semaphore"
)

type waiter struct {
	ch chan struct{}
}

type Ticket interface {
	// Release releases the ticket. The ticket must not be released multiple times.
	Release()
	// Wait waits for someone to wakeup the context. If the condition is woken up, no error is returned, and the ticket
	// remains valid. Otherwise, ctx.Err() is returned by Wait, and the ticket is rendered invalid.
	Wait(context.Context) error
}

type Cond interface {
	// Acquire acquires the lock, blocking until resources are available or ctx is done.
	// On success, returns nil error. On failure nil and ctx.Err() are returned, leaving
	// the lock unchanged.
	Acquire(ctx context.Context) (Ticket, error)

	// Broadcast wakes all goroutines waiting to acquire the lock
	Broadcast()

	// Signal wakes one goroutine waiting to acquire the lock
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

func (t *ticket) Wait(ctx context.Context) error {
	waiter := &waiter{
		ch: make(chan struct{}),
	}
	t.c.waiterLocks.Lock()
	element := t.c.waiters.PushBack(waiter)
	t.c.waiterLocks.Unlock()
	t.Release()

	select {
	case <-ctx.Done():
		t.c.waiterLocks.Lock()
		// remove is idempotent, so no need to check if it's still part of the list
		t.c.waiters.Remove(element)
		t.c.waiterLocks.Unlock()
		return ctx.Err()
	case <-waiter.ch:
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
