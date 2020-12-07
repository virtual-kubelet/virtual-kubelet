package node

import (
	"errors"
	"testing"

	"gotest.tools/assert"
)

func TestNotReadyError(t *testing.T) {
	n := newNodeNodeReady(nil)
	assert.Assert(t, errors.Is(n, &nodeNodeReadyError{}))
}
