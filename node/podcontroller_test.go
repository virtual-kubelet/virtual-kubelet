package node

import (
	"context"
	"testing"
	"time"

	"gotest.tools/assert"
)

func TestPodControllerExitOnContextCancel(t *testing.T) {
	tc := newTestController()
	ctx := context.Background()
	ctxRun, cancel := context.WithCancel(ctx)

	done := make(chan error)
	go func() {
		done <- tc.Run(ctxRun, 1)
	}()

	ctxT, cancelT := context.WithTimeout(ctx, 30*time.Second)
	select {
	case <-ctx.Done():
		assert.NilError(t, ctxT.Err())
	case <-tc.Ready():
	case <-tc.Done():
	}
	assert.NilError(t, tc.Err())

	cancelT()

	cancel()

	ctxT, cancelT = context.WithTimeout(ctx, 30*time.Second)
	defer cancelT()

	select {
	case <-ctxT.Done():
		assert.NilError(t, ctxT.Err(), "timeout waiting for Run() to exit")
	case err := <-done:
		assert.NilError(t, err)
	}
	assert.NilError(t, tc.Err())
}
