// +build e2e

package e2e

import (
	"fmt"
	"testing"
	"time"

	"github.com/virtual-kubelet/virtual-kubelet/vkubelet"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/kubernetes/pkg/kubelet/apis/stats/v1alpha1"
)

const (
	// deleteGracePeriodForProvider is the amount of time we allow for the provider to react to deletion of a pod before proceeding to assert that the pod has been deleted.
	deleteGracePeriodForProvider = 100 * time.Millisecond
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
	if err := f.WaitUntilPodReady(pod.Namespace, pod.Name); err != nil {
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
func TestPodLifecycle(t *testing.T) {
	// Create a pod with prefix "nginx-0-" having a single container.
	pod0, err := f.CreatePod(f.CreateDummyPodObjectWithPrefix("nginx-0-", "foo"))
	if err != nil {
		t.Fatal(err)
	}
	// Delete the "nginx-0-X" pod after the test finishes.
	defer func() {
		if err := f.DeletePod(pod0.Namespace, pod0.Name); err != nil && !apierrors.IsNotFound(err) {
			t.Error(err)
		}
	}()

	// Create a pod with prefix "nginx-1-" having a single container.
	pod1, err := f.CreatePod(f.CreateDummyPodObjectWithPrefix("nginx-1-", "bar"))
	if err != nil {
		t.Fatal(err)
	}
	// Delete the "nginx-1-Y" pod after the test finishes.
	defer func() {
		if err := f.DeletePod(pod1.Namespace, pod1.Name); err != nil && !apierrors.IsNotFound(err) {
			t.Error(err)
		}
	}()

	// Wait for the "nginx-0-X" pod to be reported as running and ready.
	if err := f.WaitUntilPodReady(pod0.Namespace, pod0.Name); err != nil {
		t.Fatal(err)
	}
	// Wait for the "nginx-1-Y" pod to be reported as running and ready.
	if err := f.WaitUntilPodReady(pod1.Namespace, pod1.Name); err != nil {
		t.Fatal(err)
	}

	// Grab the stats from the provider.
	stats, err := f.GetStatsSummary()
	if err != nil {
		t.Fatal(err)
	}

	// Make sure the "nginx-0-X" pod exists in the slice of PodStats.
	if _, err := findPodInPodStats(stats, pod0); err != nil {
		t.Fatal(err)
	}

	// Make sure the "nginx-1-Y" pod exists in the slice of PodStats.
	if _, err := findPodInPodStats(stats, pod1); err != nil {
		t.Fatal(err)
	}

	// Wait for the "nginx-1-Y" pod to be deleted in a separate goroutine.
	// This ensures that we don't possibly miss the MODIFIED/DELETED events due to establishing the watch too late in the process.
	pod1Ch := make(chan error)
	go func() {
		// Wait for the "nginx-1-Y" pod to be reported as having been marked for deletion.
		if err := f.WaitUntilPodDeleted(pod1.Namespace, pod1.Name); err != nil {
			// Propagate the error to the outside so we can fail the test.
			pod1Ch <- err
		} else {
			// Close the pod0Ch channel, signaling we've observed deletion of the pod.
			close(pod1Ch)
		}
	}()

	// Delete the "nginx-1" pod.
	if err := f.DeletePod(pod1.Namespace, pod1.Name); err != nil {
		t.Fatal(err)
	}
	// Wait for the delete event to be ACKed.
	if err := <-pod1Ch; err != nil {
		t.Fatal(err)
	}
	// Give the provider some time to react to the MODIFIED/DELETED events before proceeding.
	time.Sleep(deleteGracePeriodForProvider)

	// Grab the stats from the provider.
	stats, err = f.GetStatsSummary()
	if err != nil {
		t.Fatal(err)
	}

	// Make sure the "nginx-1-Y" pod DOES NOT exist in the slice of PodStats anymore.
	if _, err := findPodInPodStats(stats, pod1); err == nil {
		t.Fatalf("expected to NOT find pod \"%s/%s\" in the slice of pod stats", pod1.Namespace, pod1.Name)
	}

	// Wait for the "nginx-0-X" pod to be deleted in a separate goroutine.
	// This ensures that we don't possibly miss the MODIFIED/DELETED events due to establishing the watch too late in the process.
	pod0Ch := make(chan error)
	go func() {
		// Wait for the "nginx-0-X" pod to be reported as having been deleted.
		if err := f.WaitUntilPodDeleted(pod0.Namespace, pod0.Name); err != nil {
			// Propagate the error to the outside so we can fail the test.
			pod0Ch <- err
		} else {
			// Close the pod0Ch channel, signaling we've observed deletion of the pod.
			close(pod0Ch)
		}
	}()

	// Forcibly delete the "nginx-0" pod.
	if err := f.DeletePodImmediately(pod0.Namespace, pod0.Name); err != nil {
		t.Fatal(err)
	}
	// Wait for the delete event to be ACKed.
	if err := <-pod0Ch; err != nil {
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
	if _, err := findPodInPodStats(stats, pod0); err == nil {
		t.Fatalf("expected to NOT find pod \"%s/%s\" in the slice of pod stats", pod0.Namespace, pod0.Name)
	}
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
	if err := f.WaitUntilPodReady(pod.Namespace, pod.Name); err != nil {
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
	if err := f.WaitUntilPodReady(pod.Namespace, pod.Name); err != nil {
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
