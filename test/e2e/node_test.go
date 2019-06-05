// +build e2e

package e2e

import (
	"context"
	"testing"
	"time"

	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	watchapi "k8s.io/apimachinery/pkg/watch"
)

// TestNodeCreateAfterDelete makes sure that a node is automatically recreated
// if it is deleted while VK is running.
func TestNodeCreateAfterDelete(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	podList, err := f.KubeClient.CoreV1().Pods(f.Namespace).List(metav1.ListOptions{
		FieldSelector: fields.OneTermEqualSelector("spec.nodeName", f.NodeName).String(),
	})

	assert.NilError(t, err)
	assert.Assert(t, is.Len(podList.Items, 0), "Kubernetes does not allow node deletion with dependent objects (pods) in existence: %v")

	chErr := make(chan error, 1)

	originalNode, err := f.GetNode()
	assert.NilError(t, err)

	ctx, cancel = context.WithTimeout(ctx, time.Minute)
	defer cancel()

	go func() {
		wait := func(e watchapi.Event) (bool, error) {
			err = ctx.Err()
			// Our timeout has expired
			if err != nil {
				return true, err
			}
			if e.Type == watchapi.Deleted || e.Type == watchapi.Error {
				return false, nil
			}

			return originalNode.ObjectMeta.UID != e.Object.(*v1.Node).ObjectMeta.UID, nil
		}
		chErr <- f.WaitUntilNodeCondition(wait)
	}()

	assert.NilError(t, f.DeleteNode())

	select {
	case result := <-chErr:
		assert.NilError(t, result, "Did not observe new node object created after deletion")
	case <-ctx.Done():
		t.Fatal("Test timed out while waiting for node object to be deleted / recreated")
	}
}
