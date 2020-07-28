package nodeutil

import (
	"context"
	"testing"

	"gotest.tools/assert"
	v1 "k8s.io/api/core/v1"
)

func TestFilterPodsForNodeName(t *testing.T) {
	ctx := context.Background()
	// Doesn't match pod with wrong name
	assert.Check(t, !FilterPodsForNodeName(t.Name())(ctx, &v1.Pod{Spec: v1.PodSpec{NodeName: "NotOurNode"}}))
	// Match pod with wrong name
	assert.Check(t, FilterPodsForNodeName(t.Name())(ctx, &v1.Pod{Spec: v1.PodSpec{NodeName: t.Name()}}))
}
