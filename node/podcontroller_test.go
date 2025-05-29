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

// TestPodEventFilter ensure that pod filters are run for each event
func TestPodEventFilter(t *testing.T) {
	tc := newTestController()

	wait := make(chan struct{}, 3)
	tc.podEventFilterFunc = func(_ context.Context, pod *corev1.Pod) bool {
		wait <- struct{}{}
		return true
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error)
	go func() {
		errCh <- tc.Run(ctx, 1)
	}()

	ctxT, cancelT := context.WithTimeout(ctx, 30*time.Second)
	defer cancelT()

	select {
	case <-ctxT.Done():
		t.Fatal(ctxT.Err())
	case <-tc.Done():
		t.Fatal(tc.Err())
	case <-tc.Ready():
	case err := <-errCh:
		t.Fatal(err.Error())
	}

	pod := &corev1.Pod{}
	pod.Namespace = "default"
	pod.Name = "nginx"
	pod.Spec = newPodSpec()

	podC := tc.client.CoreV1().Pods(testNamespace)

	_, err := podC.Create(ctx, pod, metav1.CreateOptions{})
	assert.NilError(t, err)

	pod.Annotations = map[string]string{"updated": "true"}
	_, err = podC.Update(ctx, pod, metav1.UpdateOptions{})
	assert.NilError(t, err)

	err = podC.Delete(ctx, pod.Name, metav1.DeleteOptions{})
	assert.NilError(t, err)

	ctxT, cancelT = context.WithTimeout(ctx, 30*time.Second)
	defer cancelT()
	for i := 0; i < 3; i++ {
		// check that the event filter fires
		select {
		case <-ctxT.Done():
			t.Fatal(ctxT.Err())
		case <-wait:
		case err := <-errCh:
			t.Fatal(err.Error())
		}
	}
}
