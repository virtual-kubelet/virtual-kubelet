// +build e2e

package e2e

import (
	"context"
	"testing"
	"time"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"

	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	assert.Assert(t, is.Len(podList.Items, 0), "Kubernetes does not allow node deletion with dependent objects (pods) in existence: %v", podList.Items)

	chErr := make(chan error, 1)
	chDone := make(chan struct{})
	defer close(chDone)

	originalNode, err := f.GetNode()
	assert.NilError(t, err)

	var deleted bool
	go func() {
		wait := func(e watchapi.Event) (bool, error) {
			select {
			// The test has exited, go ahead and exit
			case <-ctx.Done():
				return true, nil
			default:
			}

			if e.Type == watchapi.Deleted {
				deleted = true
				return false, nil
			} else if e.Type == watchapi.Error {
				return false, nil
			}

			return originalNode.ObjectMeta.UID != e.Object.(*v1.Node).ObjectMeta.UID, nil
		}
		chErr <- f.WaitUntilNodeCondition(wait)
	}()

	assert.NilError(t, f.DeleteNode())

	timer := time.NewTimer(60 * time.Second)
	defer timer.Stop()

	select {
	case <-timer.C:
		t.Logf("Observed deletion: %v", deleted)
		t.Fatal("timeout waiting for node to be recreated")
	case err := <-chErr:
		t.Logf("Observed deletion: %v", deleted)
		assert.NilError(t, err)
	}
}
