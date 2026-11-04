// Copyright © 2026 The virtual-kubelet authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package node

import (
	"context"
	"errors"
	"testing"

	"github.com/virtual-kubelet/virtual-kubelet/trace"

	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1apply "k8s.io/client-go/applyconfigurations/core/v1"
)

func withPodStatusFieldManager(cfg *PodControllerConfig) {
	cfg.PodStatusFieldManager = "virtual-kubelet"
}

// conditionStatuses maps each of the pod's condition types to its status.
func conditionStatuses(pod *corev1.Pod) map[corev1.PodConditionType]corev1.ConditionStatus {
	m := make(map[corev1.PodConditionType]corev1.ConditionStatus, len(pod.Status.Conditions))
	for _, c := range pod.Status.Conditions {
		m[c.Type] = c.Status
	}
	return m
}

// TestApplyPodStatusKeepsStatusOthersWrote checks that a status write sends only the status the
// provider reports. Replacing the whole status with the provider's, as UpdateStatus does, removes a
// condition another controller set and the status.resourceClaimStatuses the resource claim
// controller in kube-controller-manager writes.
func TestApplyPodStatusKeepsStatusOthersWrote(t *testing.T) {
	ctx := context.Background()
	c := newTestController(withPodStatusFieldManager)
	pods := c.client.CoreV1().Pods("default")

	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "nginx"}, Spec: newPodSpec()}
	_, err := pods.Create(ctx, pod, metav1.CreateOptions{})
	assert.NilError(t, err)
	_, err = pods.ApplyStatus(ctx, corev1apply.Pod("nginx", "default").WithStatus(corev1apply.PodStatus().
		WithConditions(corev1apply.PodCondition().WithType("example.com/Gate").WithStatus(corev1.ConditionTrue)).
		WithResourceClaimStatuses(corev1apply.PodResourceClaimStatus().WithName("gpu").WithResourceClaimName("nginx-gpu-x7k2p"))),
		metav1.ApplyOptions{FieldManager: "other-controller"})
	assert.NilError(t, err)

	writeProviderPod(ctx, t, c, withStatus(pod, corev1.PodStatus{
		Phase:      corev1.PodRunning,
		Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}},
	}))

	got, err := pods.Get(ctx, "nginx", metav1.GetOptions{})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(got.Status.Phase, corev1.PodRunning))
	assert.Check(t, is.DeepEqual(conditionStatuses(got), map[corev1.PodConditionType]corev1.ConditionStatus{
		"example.com/Gate": corev1.ConditionTrue,
		corev1.PodReady:    corev1.ConditionTrue,
	}))
	assert.Check(t, is.DeepEqual(got.Status.ResourceClaimStatuses, []corev1.PodResourceClaimStatus{
		{Name: "gpu", ResourceClaimName: new("nginx-gpu-x7k2p")},
	}))
}

// writeProviderPod records podFromProvider as the provider's latest report and has the controller write it.
func writeProviderPod(ctx context.Context, t *testing.T, c *TestController, podFromProvider *corev1.Pod) {
	t.Helper()
	key := podFromProvider.Namespace + "/" + podFromProvider.Name
	c.knownPods.Store(key, &knownPod{lastPodStatusReceivedFromProvider: podFromProvider})

	podFromKubernetes, err := c.client.CoreV1().Pods(podFromProvider.Namespace).Get(ctx, podFromProvider.Name, metav1.GetOptions{})
	assert.NilError(t, err)
	assert.NilError(t, c.updatePodStatus(ctx, podFromKubernetes, key))
}

// withStatus returns a copy of pod with status.
func withStatus(pod *corev1.Pod, status corev1.PodStatus) *corev1.Pod {
	pod = pod.DeepCopy()
	pod.Status = status
	return pod
}

// TestApplyPodStatusRemovesConditionProviderDropped checks that a condition the provider stops
// reporting is removed. The controller owns the conditions it applied, so leaving one out of the
// next write removes it; merging the provider's status into the pod's current status first would
// keep it forever.
func TestApplyPodStatusRemovesConditionProviderDropped(t *testing.T) {
	ctx := context.Background()
	c := newTestController(withPodStatusFieldManager)
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "nginx"}, Spec: newPodSpec()}
	_, err := c.client.CoreV1().Pods("default").Create(ctx, pod, metav1.CreateOptions{})
	assert.NilError(t, err)

	writeProviderPod(ctx, t, c, withStatus(pod, corev1.PodStatus{
		Phase: corev1.PodRunning,
		Conditions: []corev1.PodCondition{
			{Type: corev1.PodReady, Status: corev1.ConditionTrue},
			{Type: "example.com/Warming", Status: corev1.ConditionTrue},
		},
	}))
	writeProviderPod(ctx, t, c, withStatus(pod, corev1.PodStatus{
		Phase:      corev1.PodRunning,
		Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}},
	}))

	got, err := c.client.CoreV1().Pods("default").Get(ctx, "nginx", metav1.GetOptions{})
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(conditionStatuses(got), map[corev1.PodConditionType]corev1.ConditionStatus{
		corev1.PodReady: corev1.ConditionTrue,
	}))
}

// TestApplyPodStatusLeavesMetadataAlone checks that only status is written, not the labels and
// annotations of the pod a PodNotifier provider hands over. That pod can be older than the pod in
// Kubernetes, and writing its metadata would revert labels and annotations changed since.
func TestApplyPodStatusLeavesMetadataAlone(t *testing.T) {
	ctx := context.Background()
	c := newTestController(withPodStatusFieldManager)
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   "default",
			Name:        "nginx",
			Labels:      map[string]string{"app": "nginx", "role": "leader"},
			Annotations: map[string]string{"controller.kubernetes.io/pod-deletion-cost": "1000"},
		},
		Spec: newPodSpec(),
	}
	_, err := c.client.CoreV1().Pods("default").Create(ctx, pod, metav1.CreateOptions{})
	assert.NilError(t, err)

	// The provider's copy, from before the label was added and the annotation changed.
	podFromProvider := withStatus(pod, corev1.PodStatus{Phase: corev1.PodRunning})
	podFromProvider.Labels = map[string]string{"app": "nginx"}
	podFromProvider.Annotations = map[string]string{"controller.kubernetes.io/pod-deletion-cost": "201"}
	writeProviderPod(ctx, t, c, podFromProvider)

	got, err := c.client.CoreV1().Pods("default").Get(ctx, "nginx", metav1.GetOptions{})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(got.Status.Phase, corev1.PodRunning))
	assert.Check(t, is.DeepEqual(got.Labels, map[string]string{"app": "nginx", "role": "leader"}))
	assert.Check(t, is.DeepEqual(got.Annotations, map[string]string{"controller.kubernetes.io/pod-deletion-cost": "1000"}))
}

// TestApplyProviderErrorLeavesMetadataAlone covers the status written when the provider fails to
// create or update a pod. The pod that write starts from can be older than the pod in Kubernetes;
// writing it whole, with its resourceVersion cleared as UpdateStatus does here, reverts labels and
// annotations changed since.
func TestApplyProviderErrorLeavesMetadataAlone(t *testing.T) {
	ctx := context.Background()
	c := newTestController(withPodStatusFieldManager)
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "nginx",
			Labels:    map[string]string{"app": "nginx", "role": "leader"},
		},
		Spec: newPodSpec(),
	}
	_, err := c.client.CoreV1().Pods("default").Create(ctx, pod, metav1.CreateOptions{})
	assert.NilError(t, err)

	stalePod := pod.DeepCopy()
	stalePod.Labels = map[string]string{"app": "nginx"}
	ctx, span := trace.StartSpan(ctx, t.Name())
	defer span.End()
	c.handleProviderError(ctx, span, errors.New("quota exceeded"), stalePod)

	got, err := c.client.CoreV1().Pods("default").Get(ctx, "nginx", metav1.GetOptions{})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(got.Status.Phase, corev1.PodPending))
	assert.Check(t, is.Equal(got.Status.Reason, "ProviderFailed"))
	assert.Check(t, is.Equal(got.Status.Message, "quota exceeded"))
	assert.Check(t, is.DeepEqual(got.Labels, map[string]string{"app": "nginx", "role": "leader"}))
}

// TestApplyProviderErrorKeepsProviderStatus checks that the status written when the provider fails
// to update a pod keeps the rest of the provider's last report. The controller owns what it applied
// before, so writing only the phase, reason and message would remove the container statuses.
func TestApplyProviderErrorKeepsProviderStatus(t *testing.T) {
	ctx := context.Background()
	c := newTestController(withPodStatusFieldManager)
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "nginx"}, Spec: newPodSpec()}
	_, err := c.client.CoreV1().Pods("default").Create(ctx, pod, metav1.CreateOptions{})
	assert.NilError(t, err)
	writeProviderPod(ctx, t, c, withStatus(pod, corev1.PodStatus{
		Phase: corev1.PodRunning,
		ContainerStatuses: []corev1.ContainerStatus{{
			Name:  "nginx",
			Ready: true,
			Image: "nginx:1.15.12",
			State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
		}},
	}))

	ctx, span := trace.StartSpan(ctx, t.Name())
	defer span.End()
	c.handleProviderError(ctx, span, errors.New("quota exceeded"), pod.DeepCopy())

	got, err := c.client.CoreV1().Pods("default").Get(ctx, "nginx", metav1.GetOptions{})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(got.Status.Reason, "ProviderFailed"))
	assert.Check(t, is.DeepEqual(got.Status.ContainerStatuses, []corev1.ContainerStatus{{
		Name:  "nginx",
		Ready: true,
		Image: "nginx:1.15.12",
		State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
	}}))
}

// TestApplyProviderErrorClearedByNextReport checks that once the provider reports a status again,
// the reason and message the provider error set are gone. Writing the error under a field manager
// other than the one the provider's reports use would leave them on the pod.
func TestApplyProviderErrorClearedByNextReport(t *testing.T) {
	ctx := context.Background()
	c := newTestController(withPodStatusFieldManager)
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "nginx"}, Spec: newPodSpec()}
	_, err := c.client.CoreV1().Pods("default").Create(ctx, pod, metav1.CreateOptions{})
	assert.NilError(t, err)

	spanCtx, span := trace.StartSpan(ctx, t.Name())
	defer span.End()
	c.handleProviderError(spanCtx, span, errors.New("quota exceeded"), pod.DeepCopy())
	writeProviderPod(ctx, t, c, withStatus(pod, corev1.PodStatus{Phase: corev1.PodRunning}))

	got, err := c.client.CoreV1().Pods("default").Get(ctx, "nginx", metav1.GetOptions{})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(got.Status.Phase, corev1.PodRunning))
	assert.Check(t, is.Equal(got.Status.Reason, ""))
	assert.Check(t, is.Equal(got.Status.Message, ""))
}

// TestApplyPodStatusTakesOverFieldsOthersSet checks that the write succeeds when the provider
// reports a field another field manager set, as with the PodScheduled condition kube-scheduler
// sets. Applying without force fails with a conflict on every such write.
func TestApplyPodStatusTakesOverFieldsOthersSet(t *testing.T) {
	ctx := context.Background()
	c := newTestController(withPodStatusFieldManager)
	pods := c.client.CoreV1().Pods("default")
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "nginx"}, Spec: newPodSpec()}
	_, err := pods.Create(ctx, pod, metav1.CreateOptions{})
	assert.NilError(t, err)
	_, err = pods.ApplyStatus(ctx, corev1apply.Pod("nginx", "default").WithStatus(corev1apply.PodStatus().
		WithConditions(corev1apply.PodCondition().WithType(corev1.PodScheduled).WithStatus(corev1.ConditionFalse))),
		metav1.ApplyOptions{FieldManager: "kube-scheduler"})
	assert.NilError(t, err)

	writeProviderPod(ctx, t, c, withStatus(pod, corev1.PodStatus{
		Phase:      corev1.PodRunning,
		Conditions: []corev1.PodCondition{{Type: corev1.PodScheduled, Status: corev1.ConditionTrue}},
	}))

	got, err := pods.Get(ctx, "nginx", metav1.GetOptions{})
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(conditionStatuses(got), map[corev1.PodConditionType]corev1.ConditionStatus{
		corev1.PodScheduled: corev1.ConditionTrue,
	}))
}
