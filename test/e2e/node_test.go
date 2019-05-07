// +build e2e

package e2e

import (
	"testing"
	"time"

	"gotest.tools/assert"
	watchapi "k8s.io/apimachinery/pkg/watch"
)

// TestNodeCreateAfterDelete makes sure that a node is automatically recreated
// if it is deleted while VK is running.
func TestNodeCreateAfterDelete(t *testing.T) {
	chErr := make(chan error, 1)
	chDone := make(chan struct{})
	defer close(chDone)

	var deleted bool
	go func() {
		wait := func(e watchapi.Event) (bool, error) {
			select {
			case <-chDone:
				return true, nil
			default:
			}

			if deleted {
				return f.WaitUntilNodeAdded(e)
			}
			if e.Type == watchapi.Deleted {
				deleted = true
			}
			return false, nil
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
		assert.NilError(t, err)
	}
}
