package node

import (
	"context"
	"testing"
	"time"

	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func TestCompareResourceVersion(t *testing.T) {
	p1 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			ResourceVersion: "1",
		},
	}
	p2 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			ResourceVersion: "2",
		},
	}
	assert.Assert(t, podsEffectivelyEqual(p1, p2))
}

func TestCompareStatus(t *testing.T) {
	p1 := &corev1.Pod{
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
		},
	}
	p2 := &corev1.Pod{
		Status: corev1.PodStatus{
			Phase: corev1.PodFailed,
		},
	}
	assert.Assert(t, podsEffectivelyEqual(p1, p2))
}

func TestCompareLabels(t *testing.T) {
	p1 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"foo": "bar1",
			},
		},
	}
	p2 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"foo": "bar2",
			},
		},
	}
	assert.Assert(t, !podsEffectivelyEqual(p1, p2))
}
