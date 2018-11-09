package vkubelet

import (
	"context"
	"path"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/staging/src/k8s.io/client-go/util/workqueue"
)

func TestHandleEvent(t *testing.T) {
	ctx := context.Background()
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: t.Name(), Namespace: "myNamespace"}}
	q := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	defer q.ShutDown()

	chEvent := make(chan interface{}, 1)
	go func() {
		obj, _ := q.Get()
		chEvent <- obj
	}()

	handleEvent(ctx, pod, "someEvent", q)

	var e *event
	select {
	case obj := <-chEvent:
		var ok bool
		e, ok = obj.(*event)
		if !ok {
			t.Fatalf("unexpected type %T", obj)
		}
		q.Done(obj)
	case <-time.After(30 * time.Second):
		t.Fatal("timeout waiting for dequeue event")
	}

	expectedKey := path.Join(pod.GetNamespace(), pod.GetName())
	if e.key != expectedKey {
		t.Fatalf("got unexpected cache key, expected %s, got: %s", expectedKey, e.key)
	}

	if q.Len() != 0 {
		t.Fatalf("expected an empty work queue, got: %d", q.Len())
	}
}
