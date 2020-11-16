package leasecontroller

import (
	"errors"
	"testing"

	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
)

func TestShutdownError(t *testing.T) {
	originalError := errors.New("original error")
	sErr := newShutdownError(originalError)
	val := &shutdownError{}
	ok := errors.As(sErr, &val)
	assert.Assert(t, ok)
	assert.Assert(t, is.Equal(errors.Unwrap(sErr), originalError))
}

func TestNotReadyError(t *testing.T) {
	n := newNodeNodeReady(nil)
	assert.Assert(t, errors.Is(n, &nodeNodeReadyError{}))
}
