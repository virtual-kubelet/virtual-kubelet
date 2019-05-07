// +build e2e

package e2e

import (
	"fmt"
	"testing"
	"time"

	"github.com/virtual-kubelet/virtual-kubelet/cmd/virtual-kubelet/commands/root"

	"github.com/virtual-kubelet/virtual-kubelet/vkubelet"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
	"k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/kubernetes/pkg/kubelet/apis/stats/v1alpha1"
)

const (
	// deleteGracePeriodForProvider is the maximum amount of time we allow for the provider to react to deletion of a pod
	// before proceeding to assert that the pod has been deleted.
	deleteGracePeriodForProvider = 1 * time.Second
)

// TestGetStatsSummary creates a pod having two containers and queries the /stats/summary endpoint of the virtual-kubelet.
// It expects this endpoint to return stats for the current node, as well as for the aforementioned pod and each of its two containers.
func TestGetStatsSummary(t *testing.T) {
	// Create a pod with prefix "nginx-0-" having three containers.
	pod, err := f.CreatePod(f.CreateDummyPodObjectWithPrefix("nginx-0-", "foo", "bar", "baz"))
	if err != nil {
		t.Fatal(err)
	}
	// Delete the "nginx-0-X" pod after the test finishes.
	defer func() {
		if err := f.DeletePod(pod.Namespace, pod.Name); err != nil && !apierrors.IsNotFound(err) {
			t.Error(err)
		}
	}()

	// Wait for the "nginx-0-X" pod to be reported as running and ready.
	if _, err := f.WaitUntilPodReady(pod.Namespace, pod.Name); err != nil {
		t.Fatal(err)
	}

	// Grab the stats from the provider.
	stats, err := f.GetStatsSummary()
	if err != nil {
		t.Fatal(err)
	}

	// Make sure that we've got stats for the current node.
	if stats.Node.NodeName != f.NodeName {
		t.Fatalf("expected stats for node %s, got stats for node %s", f.NodeName, stats.Node.NodeName)
	}

	// Make sure the "nginx-0-X" pod exists in the slice of PodStats.
	idx, err := findPodInPodStats(stats, pod)
	if err != nil {
		t.Fatal(err)
	}

	// Make sure that we've got stats for all the containers in the "nginx-0-X" pod.
	desiredContainerStatsCount := len(pod.Spec.Containers)
	currentContainerStatsCount := len(stats.Pods[idx].Containers)
	if currentContainerStatsCount != desiredContainerStatsCount {
		t.Fatalf("expected stats for %d containers, got stats for %d containers", desiredContainerStatsCount, currentContainerStatsCount)
	}
}

// TestPodLifecycle creates two pods and verifies that the provider has been asked to create them.
// Then, it deletes one of the pods and verifies that the provider has been asked to delete it.
// These verifications are made using the /stats/summary endpoint of the virtual-kubelet, by checking for the presence or absence of the pods.
// Hence, the provider being tested must implement the PodMetricsProvider interface.
func TestPodLifecycleGracefulDelete(t *testing.T) {
	node, err := f.GetNode()
	assert.NilError(t, err)
	if val, ok := node.Annotations[root.SoftDeleteKey]; ok && val == "true" {
		t.Run("SoftDeletesEnabled", testPodLifecycleGracefulDeleteSoftDeletesEnabled)
	} else {
		t.Run("SoftDeletesDisabled", testPodLifecycleGracefulDeleteSoftDeletesDisabled)

	}
}
func testPodLifecycleGracefulDeleteSoftDeletesEnabled(t *testing.T) {
	pod := testPodLifecycleGracefulDelete(t)

	// Wait for the pod to be deleted in a separate goroutine.
	// This ensures that we don't possibly miss the MODIFIED/DELETED events due to establishing the watch too late in the process.
	podCh := make(chan error)
	var podLast *v1.Pod
	go func() {
		defer close(podCh) // Close the podCh channel, signaling we've observed deletion of the pod.
		deletePhases := []v1.PodPhase{v1.PodSucceeded, v1.PodFailed}
		var err error
		// Wait for the "nginx-1-Y" pod to be reported as having been marked for deletion.
		podLast, err = f.WaitUntilPodInPhase(pod.Namespace, pod.Name, deletePhases...)
		if err != nil {
			// Propagate the error to the outside so we can fail the test.
			podCh <- err
		}
	}()

	// Delete the pod.
	if err := f.DeletePod(pod.Namespace, pod.Name); err != nil {
		t.Fatal(err)
	}
	t.Log("Deleted pod: ", pod.Name)
	// Wait for the delete event to be ACKed.
	if err := <-podCh; err != nil {
		t.Fatal(err)
	}
	// Give the provider some time to react to the MODIFIED/DELETED events before proceeding.
	time.Sleep(deleteGracePeriodForProvider)

	// Grab the stats from the provider.
	stats, err := f.GetStatsSummary()
	if err != nil {
		t.Fatal(err)
	}

	// Make sure the pod DOES NOT exist in the slice of PodStats anymore.
	if _, err := findPodInPodStats(stats, pod); err == nil {
		t.Fatalf("expected to NOT find pod \"%s/%s\" in the slice of pod stats", pod.Namespace, pod.Name)
	}

	assert.Assert(t, podLast != nil)
	assert.Equal(t, podLast.Status.Phase, v1.PodSucceeded)
	assert.Assert(t, podLast.ObjectMeta.GetDeletionGracePeriodSeconds() != nil)
	assert.Assert(t, *podLast.ObjectMeta.GetDeletionGracePeriodSeconds() > int64(0))

	t.Logf("Pod ended as phase: %+v", podLast.Status.Phase)
	t.Logf("Pod 0 ended as metadata: %+v", *podLast.ObjectMeta.GetDeletionGracePeriodSeconds())
	assert.Equal(t, "true", podLast.ObjectMeta.Annotations["virtual-kubelet.io/softDelete"])
}
func testPodLifecycleGracefulDeleteSoftDeletesDisabled(t *testing.T) {
	pod := testPodLifecycleGracefulDelete(t)
	podCh := make(chan error)
	var podLast *v1.Pod
	go func() {
		// Close the podCh channel, signaling we've observed deletion of the pod.
		defer close(podCh)

		var err error
		podLast, err = f.WaitUntilPodDeleted(pod.Namespace, pod.Name)
		if err != nil {
			// Propagate the error to the outside so we can fail the test.
			podCh <- err
		}
	}()

	// Gracefully delete the "nginx-0" pod.
	if err := f.DeletePod(pod.Namespace, pod.Name); err != nil {
		t.Fatal(err)
	}
	t.Log("Force deleted pod: ", pod.Name)

	// Wait for the delete event to be ACKed.
	if err := <-podCh; err != nil {
		t.Fatal(err)
	}
	// Give the provider some time to react to the MODIFIED/DELETED events before proceeding.
	time.Sleep(deleteGracePeriodForProvider)
	assert.Assert(t, podLast != nil)
	assert.Assert(t, podLast.ObjectMeta.GetDeletionGracePeriodSeconds() != nil)
	assert.Equal(t, int64(0), *podLast.ObjectMeta.GetDeletionGracePeriodSeconds())
	assert.Assert(t, is.Nil(podLast.ObjectMeta.Annotations["virtual-kubelet.io/softDelete"]))
}

func testPodLifecycleGracefulDelete(t *testing.T) *v1.Pod {
	// Create a pod with prefix "nginx-0-" having a single container.
	podSpec := f.CreateDummyPodObjectWithPrefix("nginx-"+t.Name()+"-", "foo")
	podSpec.Spec.NodeName = nodeName

	pod, err := f.CreatePod(podSpec)
	if err != nil {
		t.Fatal(err)
	}
	// Delete the pod after the test finishes.
	defer func() {
		if err := f.DeletePod(pod.Namespace, pod.Name); err != nil && !apierrors.IsNotFound(err) {
			t.Error(err)
		}
	}()
	t.Log("Created pod: " + pod.Name)

	// Wait for the "nginx-0-X" pod to be reported as running and ready.
	if _, err := f.WaitUntilPodReady(pod.Namespace, pod.Name); err != nil {
		t.Fatal(err)
	}
	t.Logf("Pod %s ready", pod.Name)

	// Grab the stats from the provider.
	stats, err := f.GetStatsSummary()
	if err != nil {
		t.Fatal(err)
	}

	// Make sure the "nginx-0-X" pod exists in the slice of PodStats.
	if _, err := findPodInPodStats(stats, pod); err != nil {
		t.Fatal(err)
	}
	return pod
}

// TestPodLifecycleNonGracefulDelete creates one podsand verifies that the provider has created them
// and put them in the running lifecycle. It then does a force delete on the pod, and verifies the provider
// has deleted it.
func TestPodLifecycleForceDelete(t *testing.T) {

	// Create a pod with prefix having a single container.
	pod, err := f.CreatePod(f.CreateDummyPodObjectWithPrefix("nginx-0-", "foo"))
	if err != nil {
		t.Fatal(err)
	}
	// Delete the pod after the test finishes.
	defer func() {
		if err := f.DeletePod(pod.Namespace, pod.Name); err != nil && !apierrors.IsNotFound(err) {
			t.Error(err)
		}
	}()
	t.Log("Created pod: " + pod.Name)

	// Wait for the "nginx-0-X" pod to be reported as running and ready.
	if _, err := f.WaitUntilPodReady(pod.Namespace, pod.Name); err != nil {
		t.Fatal(err)
	}
	t.Logf("Pod %s ready", pod.Name)

	// Grab the stats from the provider.
	stats, err := f.GetStatsSummary()
	if err != nil {
		t.Fatal(err)
	}

	// Make sure the pod exists in the slice of PodStats.
	if _, err := findPodInPodStats(stats, pod); err != nil {
		t.Fatal(err)
	}

	// Wait for the pod to be deleted in a separate goroutine.
	// This ensures that we don't possibly miss the MODIFIED/DELETED events due to establishing the watch too late in the process.
	// It also makes sure that in light of soft deletes, we properly handle non-graceful pod deletion
	podCh := make(chan error)
	var podLast *v1.Pod
	go func() {
		// Close the podCh channel, signaling we've observed deletion of the pod.
		defer close(podCh)

		var err error
		// Wait for the pod to be reported as having been deleted.
		podLast, err = f.WaitUntilPodDeleted(pod.Namespace, pod.Name)
		if err != nil {
			// Propagate the error to the outside so we can fail the test.
			podCh <- err
		}
	}()

	// Forcibly delete the pod.
	if err := f.DeletePodImmediately(pod.Namespace, pod.Name); err != nil {
		t.Fatal(err)
	}
	t.Log("Force deleted pod: ", pod.Name)

	// Wait for the delete event to be ACKed.
	if err := <-podCh; err != nil {
		t.Fatal(err)
	}
	// Give the provider some time to react to the MODIFIED/DELETED events before proceeding.
	time.Sleep(deleteGracePeriodForProvider)

	// Grab the stats from the provider.
	stats, err = f.GetStatsSummary()
	if err != nil {
		t.Fatal(err)
	}

	// Make sure the "nginx-0-X" pod DOES NOT exist in the slice of PodStats anymore.
	if _, err := findPodInPodStats(stats, pod); err == nil {
		t.Fatalf("expected to NOT find pod \"%s/%s\" in the slice of pod stats", pod.Namespace, pod.Name)
	}

	t.Logf("Pod 0 ended as phase: %+v", podLast.Status.Phase)
	t.Logf("Pod 0 ended as phase: %+v", *podLast.GetObjectMeta().GetDeletionGracePeriodSeconds())

}

// TestCreatePodWithOptionalInexistentSecrets tries to create a pod referencing optional, inexistent secrets.
// It then verifies that the pod is created successfully.
func TestCreatePodWithOptionalInexistentSecrets(t *testing.T) {
	// Create a pod with a single container referencing optional, inexistent secrets.
	pod, err := f.CreatePod(f.CreatePodObjectWithOptionalSecretKey())
	if err != nil {
		t.Fatal(err)
	}

	// Delete the pod after the test finishes.
	defer func() {
		if err := f.DeletePod(pod.Namespace, pod.Name); err != nil && !apierrors.IsNotFound(err) {
			t.Error(err)
		}
	}()

	// Wait for the pod to be reported as running and ready.
	if _, err := f.WaitUntilPodReady(pod.Namespace, pod.Name); err != nil {
		t.Fatal(err)
	}

	// Wait for an event concerning the missing secret to be reported on the pod.
	if err := f.WaitUntilPodEventWithReason(pod, vkubelet.ReasonOptionalSecretNotFound); err != nil {
		t.Fatal(err)
	}

	// Check that the pod is known to the provider.
	stats, err := f.GetStatsSummary()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := findPodInPodStats(stats, pod); err != nil {
		t.Fatal(err)
	}
}

// TestCreatePodWithMandatoryInexistentSecrets tries to create a pod referencing inexistent secrets.
// It then verifies that the pod is not created.
func TestCreatePodWithMandatoryInexistentSecrets(t *testing.T) {
	// Create a pod with a single container referencing inexistent secrets.
	pod, err := f.CreatePod(f.CreatePodObjectWithMandatorySecretKey())
	if err != nil {
		t.Fatal(err)
	}

	// Delete the pod after the test finishes.
	defer func() {
		if err := f.DeletePodImmediately(pod.Namespace, pod.Name); err != nil && !apierrors.IsNotFound(err) {
			t.Error(err)
		}
	}()

	// Wait for an event concerning the missing secret to be reported on the pod.
	if err := f.WaitUntilPodEventWithReason(pod, vkubelet.ReasonMandatorySecretNotFound); err != nil {
		t.Fatal(err)
	}

	// Check that the pod is NOT known to the provider.
	stats, err := f.GetStatsSummary()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := findPodInPodStats(stats, pod); err == nil {
		t.Fatalf("Expecting to NOT find pod \"%s/%s\" having mandatory, inexistent secrets.", pod.Namespace, pod.Name)
	}
}

// TestCreatePodWithOptionalInexistentConfigMap tries to create a pod referencing optional, inexistent config map.
// It then verifies that the pod is created successfully.
func TestCreatePodWithOptionalInexistentConfigMap(t *testing.T) {
	// Create a pod with a single container referencing optional, inexistent config map.
	pod, err := f.CreatePod(f.CreatePodObjectWithOptionalConfigMapKey())
	if err != nil {
		t.Fatal(err)
	}

	// Delete the pod after the test finishes.
	defer func() {
		if err := f.DeletePod(pod.Namespace, pod.Name); err != nil && !apierrors.IsNotFound(err) {
			t.Error(err)
		}
	}()

	// Wait for the pod to be reported as running and ready.
	if _, err := f.WaitUntilPodReady(pod.Namespace, pod.Name); err != nil {
		t.Fatal(err)
	}

	// Wait for an event concerning the missing config map to be reported on the pod.
	if err := f.WaitUntilPodEventWithReason(pod, vkubelet.ReasonOptionalConfigMapNotFound); err != nil {
		t.Fatal(err)
	}

	// Check that the pod is known to the provider.
	stats, err := f.GetStatsSummary()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := findPodInPodStats(stats, pod); err != nil {
		t.Fatal(err)
	}
}

// TestCreatePodWithMandatoryInexistentConfigMap tries to create a pod referencing inexistent secrets.
// It then verifies that the pod is not created.
func TestCreatePodWithMandatoryInexistentConfigMap(t *testing.T) {
	// Create a pod with a single container referencing inexistent config map.
	pod, err := f.CreatePod(f.CreatePodObjectWithMandatoryConfigMapKey())
	if err != nil {
		t.Fatal(err)
	}

	// Delete the pod after the test finishes.
	defer func() {
		if err := f.DeletePodImmediately(pod.Namespace, pod.Name); err != nil && !apierrors.IsNotFound(err) {
			t.Error(err)
		}
	}()

	// Wait for an event concerning the missing config map to be reported on the pod.
	if err := f.WaitUntilPodEventWithReason(pod, vkubelet.ReasonMandatoryConfigMapNotFound); err != nil {
		t.Fatal(err)
	}

	// Check that the pod is NOT known to the provider.
	stats, err := f.GetStatsSummary()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := findPodInPodStats(stats, pod); err == nil {
		t.Fatalf("Expecting to NOT find pod \"%s/%s\" having mandatory, inexistent config map.", pod.Namespace, pod.Name)
	}
}

// findPodInPodStats returns the index of the specified pod in the .pods field of the specified Summary object.
// It returns an error if the specified pod is not found.
func findPodInPodStats(summary *v1alpha1.Summary, pod *v1.Pod) (int, error) {
	for i, p := range summary.Pods {
		if p.PodRef.Namespace == pod.Namespace && p.PodRef.Name == pod.Name && string(p.PodRef.UID) == string(pod.UID) {
			return i, nil
		}
	}
	return -1, fmt.Errorf("failed to find pod \"%s/%s\" in the slice of pod stats", pod.Namespace, pod.Name)
}
