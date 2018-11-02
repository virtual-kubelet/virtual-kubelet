package vkubelet

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
)

func NewCancellableMutex() *CancellableMutex {
	return &CancellableMutex{
		ch: make(chan struct{}, 1),
	}
}

// CancellableMutex is a mutex in which the Lock() action can be canncelled by
// cancelling a context.
// This must be initialized by calling `NewCancellableMutex()`
//
// You should expect this to be quite a bit slower than a normal sync.Mutex from
// the stdlib.
type CancellableMutex struct {
	ch chan struct{}
}

// Lock locks the mutex.
// If the passed in context is cancelled before the mutex can be acquired the
// lock request will be cancelled and an error will be returned.
//
// An error is only returned if the context was cancelled.
func (m *CancellableMutex) Lock(ctx context.Context) error {
	if m.ch == nil {
		panic("cancellable mutex is not initialized")
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case m.ch <- struct{}{}:
	}
	return nil
}

// Unlock unlocks the mutex
// This will panic if the mutex is not locked.
func (m *CancellableMutex) Unlock() {
	select {
	case <-m.ch:
	default:
		panic("unlock of unlocked mutex")
	}
}

// ErrNoSuchLock is returned when the requested lock does not exist
var ErrNoSuchLock = errors.New("no such lock")

// NamedLock provides a locking mechanism based on the passed in reference name
type NamedLock struct {
	mu    sync.Mutex
	locks map[string]*lockCtr
}

// lockCtr is used by NamedLock to represent a lock with a given name.
type lockCtr struct {
	mu *CancellableMutex
	// waiters is the number of waiters waiting to acquire the lock
	// this is int32 instead of uint32 so we can add `-1` in `dec()`
	waiters int32
}

// inc increments the number of waiters waiting for the lock
func (l *lockCtr) inc() {
	atomic.AddInt32(&l.waiters, 1)
}

// dec decrements the number of waiters waiting on the lock
func (l *lockCtr) dec() {
	atomic.AddInt32(&l.waiters, -1)
}

// count gets the current number of waiters
func (l *lockCtr) count() int32 {
	return atomic.LoadInt32(&l.waiters)
}

// Lock locks the mutex
func (l *lockCtr) Lock(ctx context.Context) error {
	return l.mu.Lock(ctx)
}

// Unlock unlocks the mutex
func (l *lockCtr) Unlock() {
	l.mu.Unlock()
}

// New creates a new NamedLock
func NewNamedLock() *NamedLock {
	return &NamedLock{
		locks: make(map[string]*lockCtr),
	}
}

// Lock locks a mutex with the given name. If it doesn't exist, one is created
func (l *NamedLock) Lock(ctx context.Context, name string) error {
	l.mu.Lock()
	if l.locks == nil {
		l.locks = make(map[string]*lockCtr)
	}

	nameLock, exists := l.locks[name]
	if !exists {
		nameLock = &lockCtr{mu: NewCancellableMutex()}
		l.locks[name] = nameLock
	}

	// increment the nameLock waiters while inside the main mutex
	// this makes sure that the lock isn't deleted if `Lock` and `Unlock` are called concurrently
	nameLock.inc()
	l.mu.Unlock()

	// Lock the nameLock outside the main mutex so we don't block other operations
	// once locked then we can decrement the number of waiters for this lock
	err := nameLock.Lock(ctx)
	nameLock.dec()
	return err
}

// Unlock unlocks the mutex with the given name
// If the given lock is not being waited on by any other callers, it is deleted
func (l *NamedLock) Unlock(name string) {
	l.mu.Lock()
	nameLock, exists := l.locks[name]
	if !exists {
		l.mu.Unlock()
		panic(ErrNoSuchLock)
	}

	if nameLock.count() == 0 {
		delete(l.locks, name)
	}
	nameLock.Unlock()

	l.mu.Unlock()
}
