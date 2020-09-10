package node

import (
	"context"
	"sync"
)

type waitableInt struct {
	cond *sync.Cond
	val  int
}

func newWaitableInt() *waitableInt {
	return &waitableInt{
		cond: sync.NewCond(&sync.Mutex{}),
	}
}

func (w *waitableInt) read() int {
	defer w.cond.L.Unlock()
	w.cond.L.Lock()
	return w.val
}

func (w *waitableInt) until(ctx context.Context, f func(int) bool) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		<-ctx.Done()
		w.cond.Broadcast()
	}()

	w.cond.L.Lock()
	defer w.cond.L.Unlock()

	for !f(w.val) {
		if err := ctx.Err(); err != nil {
			return err
		}
		w.cond.Wait()
	}
	return nil
}

func (w *waitableInt) increment() {
	w.cond.L.Lock()
	defer w.cond.L.Unlock()
	w.val++
	w.cond.Broadcast()
}
