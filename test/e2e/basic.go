package e2e

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/prometheus/common/expfmt"
	"github.com/virtual-kubelet/virtual-kubelet/internal/podutils"
	"gotest.tools/assert"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	stats "k8s.io/kubelet/pkg/apis/stats/v1alpha1"
)

const (
	// deleteGracePeriodForProvider is the maximum amount of time we allow for the provider to react to deletion of a pod
	// before proceeding to assert that the pod has been deleted.
	deleteGracePeriodForProvider = 1 * time.Second
)

// TestGetPods tests that the /pods endpoint works, and only returns pods for our kubelet
func (ts *EndToEndTestSuite) TestGetPods(t *testing.T) {
	ctx := context.Background()

	// Create a pod with prefix "nginx-" having a single container.
	podSpec := f.CreateDummyPodObjectWithPrefix(t.Name(), "nginx", "foo")
	podSpec.Spec.NodeName = f.NodeName

	nginx, err := f.CreatePod(ctx, podSpec)
	if err != nil {
		t.Fatal(err)
	}
	// Delete the pod after the test finishes.
	defer func() {
		if err := f.DeletePodImmediately(ctx, nginx.Namespace, nginx.Name); err != nil && !apierrors.IsNotFound(err) {
			t.Error(err)
		}
	}()
	t.Logf("Created pod: %s", nginx.Name)

	// Wait for the "nginx-" pod to be reported as running and ready.
	if _, err := f.WaitUntilPodReady(nginx.Namespace, nginx.Name); err != nil {
		t.Fatal(err)
	}
	t.Logf("Pod %s ready", nginx.Name)

	k8sPods, err := f.GetRunningPodsFromKubernetes(ctx)
	if err != nil {
		t.Fatal(err)
	}

	podFound := false
	for _, pod := range k8sPods.Items {
		if pod.Spec.NodeName != f.NodeName {
			t.Fatalf("Found pod with node name %s, whereas expected %s", pod.Spec.NodeName, f.NodeName)
		}
		if pod.UID == nginx.UID {
			podFound = true
		}
	}
	if !podFound {
		t.Fatal("Nginx pod not found")
	}
}

// TestGetStatsSummary creates a pod having two containers and queries the /stats/summary endpoint of the virtual-kubelet.
// It expects this endpoint to return stats for the current node, as well as for the aforementioned pod and each of its two containers.
func (ts *EndToEndTestSuite) TestGetStatsSummary(t *testing.T) {
	ctx := context.Background()

	// Create a pod with prefix "nginx-" having three containers.
	pod, err := f.CreatePod(ctx, f.CreateDummyPodObjectWithPrefix(t.Name(), "nginx", "foo", "bar", "baz"))
	if err != nil {
		t.Fatal(err)
	}
	// Delete the "nginx-0-X" pod after the test finishes.
	defer func() {
		if err := f.DeletePodImmediately(ctx, pod.Namespace, pod.Name); err != nil && !apierrors.IsNotFound(err) {
			t.Error(err)
		}
	}()

	// Wait for the "nginx-" pod to be reported as running and ready.
	if _, err := f.WaitUntilPodReady(pod.Namespace, pod.Name); err != nil {
		t.Fatal(err)
	}

	// Grab the stats from the provider.
	stats, err := f.GetStatsSummary(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// Make sure that we've got stats for the current node.
	if stats.Node.NodeName != f.NodeName {
		t.Fatalf("expected stats for node %s, got stats for node %s", f.NodeName, stats.Node.NodeName)
	}

	// Make sure the "nginx-" pod exists in the slice of PodStats.
	idx, err := findPodInPodStats(stats, pod)
	if err != nil {
		t.Fatal(err)
	}

	// Make sure that we've got stats for all the containers in the "nginx-" pod.
	desiredContainerStatsCount := len(pod.Spec.Containers)
	currentContainerStatsCount := len(stats.Pods[idx].Containers)
	if currentContainerStatsCount != desiredContainerStatsCount {
		t.Fatalf("expected stats for %d containers, got stats for %d containers", desiredContainerStatsCount, currentContainerStatsCount)
	}
}

// TestGetMetricsResource creates a pod having two containers and queries the /metrics/resource endpoint of the virtual-kubelet.
// It expects this endpoint to return stats for the current node, as well as for the aforementioned pod and each of its two containers.
func (ts *EndToEndTestSuite) TestGetMetricsResource(t *testing.T) {
	ctx := context.Background()

	// Create a pod with prefix "nginx-" having three containers.
	pod, err := f.CreatePod(ctx, f.CreateDummyPodObjectWithPrefix(t.Name(), "nginx", "foo", "bar", "baz"))
	if err != nil {
		t.Fatal(err)
	}
	// Delete the "nginx-0-X" pod after the test finishes.
	defer func() {
		if err := f.DeletePodImmediately(ctx, pod.Namespace, pod.Name); err != nil && !apierrors.IsNotFound(err) {
			t.Error(err)
		}
	}()

	// Wait for the "nginx-" pod to be reported as running and ready.
	if _, err := f.WaitUntilPodReady(pod.Namespace, pod.Name); err != nil {
		t.Fatal(err)
	}

	// Grab the stats from the provider.
	metricsResourceResponse, err := f.GetMetricsResource(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// decode metrics response bytes to metric family
	reader := bytes.NewReader(metricsResourceResponse)
	parser := expfmt.TextParser{}
	metricsFamilyMap, err := parser.TextToMetricFamilies(reader)
	if err != nil {
		t.Fatal(err)
	}

	// Make sure the "nginx-" pod exists in the metrics returned.
	currentContainerStatsCount := 0
	found := false
	for metricName, metricFamily := range metricsFamilyMap {
		if metricName == "pod_cpu_usage_seconds_total" {
			for _, metric := range metricFamily.Metric {
				if *metric.Label[1].Value == pod.Name {
					found = true
				}
			}
		}
		if metricName == "container_cpu_usage_seconds_total" {
			for _, metric := range metricFamily.Metric {
				if *metric.Label[1].Value == pod.Name {
					currentContainerStatsCount += 1
				}
			}
		}
	}
	if !found {
		t.Fatalf("Pod %s not found in metrics", pod.Name)
	}

	// Make sure that we've got stats for all the containers in the "nginx-" pod.
	desiredContainerStatsCount := len(pod.Spec.Containers)
	if currentContainerStatsCount != desiredContainerStatsCount {
		t.Fatalf("expected stats for %d containers, got stats for %d containers", desiredContainerStatsCount, currentContainerStatsCount)
	}
}

// TestPodLifecycleGracefulDelete creates a pod and verifies that the provider has been asked to create it.
// Then, it deletes the pods and verifies that the provider has been asked to delete it.
// These verifications are made using the /stats/summary endpoint of the virtual-kubelet, by checking for the presence or absence of the pods.
// Hence, the provider being tested must implement the PodMetricsProvider interface.
func (ts *EndToEndTestSuite) TestPodLifecycleGracefulDelete(t *testing.T) {
	ctx := context.Background()

	// Create a pod with prefix "nginx-" having a single container.
	podSpec := f.CreateDummyPodObjectWithPrefix(t.Name(), "nginx", "foo")
	podSpec.Spec.NodeName = f.NodeName

	pod, err := f.CreatePod(ctx, podSpec)
	if err != nil {
		t.Fatal(err)
	}
	// Delete the pod after the test finishes.
	defer func() {
		if err := f.DeletePodImmediately(ctx, pod.Namespace, pod.Name); err != nil && !apierrors.IsNotFound(err) {
			t.Error(err)
		}
	}()
	t.Logf("Created pod: %s", pod.Name)

	// Wait for the "nginx-" pod to be reported as running and ready.
	if _, err := f.WaitUntilPodReady(pod.Namespace, pod.Name); err != nil {
		t.Fatal(err)
	}
	t.Logf("Pod %s ready", pod.Name)

	// Grab the pods from the provider.
	pods, err := f.GetRunningPodsFromProvider(ctx)
	assert.NilError(t, err)

	// Check if the pod exists in the slice of PodStats.
	assert.NilError(t, findPodInPods(pods, pod))

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

	// Gracefully delete the "nginx-" pod.
	if err := f.DeletePod(ctx, pod.Namespace, pod.Name); err != nil {
		t.Fatal(err)
	}
	t.Logf("Deleted pod: %s", pod.Name)

	// Wait for the delete event to be ACKed.
	if err := <-podCh; err != nil {
		t.Fatal(err)
	}

	time.Sleep(deleteGracePeriodForProvider)
	// Give the provider some time to react to the MODIFIED/DELETED events before proceeding.
	// Grab the pods from the provider.
	pods, err = f.GetRunningPodsFromProvider(ctx)
	assert.NilError(t, err)

	// Make sure the pod DOES NOT exist in the provider's set of running pods
	assert.Assert(t, findPodInPods(pods, pod) != nil)

	// Make sure we saw the delete event, and the delete event was graceful
	assert.Assert(t, podLast != nil)
	assert.Assert(t, podLast.GetDeletionGracePeriodSeconds() != nil)
	assert.Assert(t, *podLast.GetDeletionGracePeriodSeconds() > 0)
}

// TestPodLifecycleForceDelete creates one podsand verifies that the provider has created them
// and put them in the running lifecycle. It then does a force delete on the pod, and verifies the provider
// has deleted it.
func (ts *EndToEndTestSuite) TestPodLifecycleForceDelete(t *testing.T) {
	ctx := context.Background()

	podSpec := f.CreateDummyPodObjectWithPrefix(t.Name(), "nginx", "foo")
	// Create a pod with prefix having a single container.
	pod, err := f.CreatePod(ctx, podSpec)
	if err != nil {
		t.Fatal(err)
	}
	// Delete the pod after the test finishes.
	defer func() {
		if err := f.DeletePodImmediately(ctx, pod.Namespace, pod.Name); err != nil && !apierrors.IsNotFound(err) {
			t.Error(err)
		}
	}()
	t.Logf("Created pod: %s", pod.Name)

	// Wait for the "nginx-" pod to be reported as running and ready.
	if _, err := f.WaitUntilPodReady(pod.Namespace, pod.Name); err != nil {
		t.Fatal(err)
	}
	t.Logf("Pod %s ready", pod.Name)

	// Grab the pods from the provider.
	pods, err := f.GetRunningPodsFromProvider(ctx)
	assert.NilError(t, err)

	// Check if the pod exists in the slice of Pods.
	assert.NilError(t, findPodInPods(pods, pod))

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

	time.Sleep(deleteGracePeriodForProvider)
	// Forcibly delete the pod.
	if err := f.DeletePodImmediately(ctx, pod.Namespace, pod.Name); err != nil {
		t.Logf("Last saw pod in state: %+v", podLast)
		t.Fatal(err)
	}
	t.Log("Force deleted pod: ", pod.Name)

	// Wait for the delete event to be ACKed.
	if err := <-podCh; err != nil {
		t.Logf("Last saw pod in state: %+v", podLast)
		t.Fatal(err)
	}
	// Give the provider some time to react to the MODIFIED/DELETED events before proceeding.
	time.Sleep(deleteGracePeriodForProvider)

	// Grab the pods from the provider.
	pods, err = f.GetRunningPodsFromProvider(ctx)
	assert.NilError(t, err)

	// Make sure the "nginx-" pod DOES NOT exist in the slice of Pods anymore.
	assert.Assert(t, findPodInPods(pods, pod) != nil)

	t.Logf("Pod ended as phase: %+v", podLast.Status.Phase)

}

// TestCreatePodWithOptionalInexistentSecrets tries to create a pod referencing optional, inexistent secrets.
// It then verifies that the pod is created successfully.
func (ts *EndToEndTestSuite) TestCreatePodWithOptionalInexistentSecrets(t *testing.T) {
	ctx := context.Background()

	// Create a pod with a single container referencing optional, inexistent secrets.
	pod, err := f.CreatePod(ctx, f.CreatePodObjectWithOptionalSecretKey(t.Name()))
	if err != nil {
		t.Fatal(err)
	}

	// Delete the pod after the test finishes.
	defer func() {
		if err := f.DeletePodImmediately(ctx, pod.Namespace, pod.Name); err != nil && !apierrors.IsNotFound(err) {
			t.Error(err)
		}
	}()

	// Wait for the pod to be reported as running and ready.
	if _, err := f.WaitUntilPodReady(pod.Namespace, pod.Name); err != nil {
		t.Fatal(err)
	}

	// Wait for an event concerning the missing secret to be reported on the pod.
	if err := f.WaitUntilPodEventWithReason(pod, podutils.ReasonOptionalSecretNotFound); err != nil {
		t.Fatal(err)
	}

	// Grab the pods from the provider.
	pods, err := f.GetRunningPodsFromProvider(ctx)
	assert.NilError(t, err)

	// Check if the pod exists in the slice of Pods.
	assert.NilError(t, findPodInPods(pods, pod))
}

// TestCreatePodWithMandatoryInexistentSecrets tries to create a pod referencing inexistent secrets.
// It then verifies that the pod is not created.
func (ts *EndToEndTestSuite) TestCreatePodWithMandatoryInexistentSecrets(t *testing.T) {
	ctx := context.Background()

	// Create a pod with a single container referencing inexistent secrets.
	pod, err := f.CreatePod(ctx, f.CreatePodObjectWithMandatorySecretKey(t.Name()))
	if err != nil {
		t.Fatal(err)
	}

	// Delete the pod after the test finishes.
	defer func() {
		if err := f.DeletePodImmediately(ctx, pod.Namespace, pod.Name); err != nil && !apierrors.IsNotFound(err) {
			t.Error(err)
		}
	}()

	// Wait for an event concerning the missing secret to be reported on the pod.
	if err := f.WaitUntilPodEventWithReason(pod, podutils.ReasonMandatorySecretNotFound); err != nil {
		t.Fatal(err)
	}

	// Grab the pods from the provider.
	pods, err := f.GetRunningPodsFromProvider(ctx)
	assert.NilError(t, err)

	// Check if the pod exists in the slice of PodStats.
	assert.Assert(t, findPodInPods(pods, pod) != nil)
}

// TestCreatePodWithOptionalInexistentConfigMap tries to create a pod referencing optional, inexistent config map.
// It then verifies that the pod is created successfully.
func (ts *EndToEndTestSuite) TestCreatePodWithOptionalInexistentConfigMap(t *testing.T) {
	ctx := context.Background()

	// Create a pod with a single container referencing optional, inexistent config map.
	pod, err := f.CreatePod(ctx, f.CreatePodObjectWithOptionalConfigMapKey(t.Name()))
	if err != nil {
		t.Fatal(err)
	}

	// Delete the pod after the test finishes.
	defer func() {
		if err := f.DeletePodImmediately(ctx, pod.Namespace, pod.Name); err != nil && !apierrors.IsNotFound(err) {
			t.Error(err)
		}
	}()

	// Wait for the pod to be reported as running and ready.
	if _, err := f.WaitUntilPodReady(pod.Namespace, pod.Name); err != nil {
		t.Fatal(err)
	}

	// Wait for an event concerning the missing config map to be reported on the pod.
	if err := f.WaitUntilPodEventWithReason(pod, podutils.ReasonOptionalConfigMapNotFound); err != nil {
		t.Fatal(err)
	}

	// Grab the pods from the provider.
	pods, err := f.GetRunningPodsFromProvider(ctx)
	assert.NilError(t, err)

	// Check if the pod exists in the slice of PodStats.
	assert.NilError(t, findPodInPods(pods, pod))
}

// TestCreatePodWithMandatoryInexistentConfigMap tries to create a pod referencing inexistent secrets.
// It then verifies that the pod is not created.
func (ts *EndToEndTestSuite) TestCreatePodWithMandatoryInexistentConfigMap(t *testing.T) {
	ctx := context.Background()

	// Create a pod with a single container referencing inexistent config map.
	pod, err := f.CreatePod(ctx, f.CreatePodObjectWithMandatoryConfigMapKey(t.Name()))
	if err != nil {
		t.Fatal(err)
	}

	// Delete the pod after the test finishes.
	defer func() {
		if err := f.DeletePodImmediately(ctx, pod.Namespace, pod.Name); err != nil && !apierrors.IsNotFound(err) {
			t.Error(err)
		}
	}()

	// Wait for an event concerning the missing config map to be reported on the pod.
	if err := f.WaitUntilPodEventWithReason(pod, podutils.ReasonMandatoryConfigMapNotFound); err != nil {
		t.Fatal(err)
	}

	// Grab the pods from the provider.
	pods, err := f.GetRunningPodsFromProvider(ctx)
	assert.NilError(t, err)

	// Check if the pod exists in the slice of PodStats.
	assert.Assert(t, findPodInPods(pods, pod) != nil)
}

// findPodInPodStats returns the index of the specified pod in the .pods field of the specified Summary object.
// It returns an error if the specified pod is not found.
func findPodInPodStats(summary *stats.Summary, pod *v1.Pod) (int, error) {
	for i, p := range summary.Pods {
		if p.PodRef.Namespace == pod.Namespace && p.PodRef.Name == pod.Name && p.PodRef.UID == string(pod.UID) {
			return i, nil
		}
	}
	return -1, fmt.Errorf("failed to find pod \"%s/%s\" in the slice of pod stats", pod.Namespace, pod.Name)
}

// findPodInPodStats returns the index of the specified pod in the .pods field of the specified PodList object.
// It returns error if the pod doesn't exist in the podlist
func findPodInPods(pods *v1.PodList, pod *v1.Pod) error {
	for _, p := range pods.Items {
		if p.Namespace == pod.Namespace && p.Name == pod.Name && string(p.UID) == string(pod.UID) {
			return nil
		}
	}
	return fmt.Errorf("failed to find pod \"%s/%s\" in the slice of pod list", pod.Namespace, pod.Name)
}
