package node

import (
	"errors"
	"testing"

	"gotest.tools/assert"
)

func TestNotReadyError(t *testing.T) {
	n := newNodeNotReadyError(nil)
	assert.Assert(t, errors.Is(n, &nodeNotReadyError{}))
}
