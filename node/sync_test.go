// Copyright © 2017 The virtual-kubelet authors
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
	"fmt"
	"testing"
	"time"

	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// TestSyncProviderWrapperDeletePodClearsRunningState covers a provider that implements only the
// synchronous PodLifecycleHandler, which is the case PodController.Run wraps in a
// syncProviderWrapper. The terminal status the wrapper reports after a delete has to leave each
// container in one state, since ContainerState is a union: updatePodStatus writes that status
// straight to the pods/status subresource, so anything left over is what a reader sees.
func TestSyncProviderWrapperDeletePodClearsRunningState(t *testing.T) {
	ctx := context.Background()
	c := newTestController()

	startedAt := metav1.NewTime(time.Now().Add(-time.Hour))

	k8sPod := &corev1.Pod{}
	k8sPod.Namespace = "default"
	k8sPod.Name = "nginx"
	k8sPod.ResourceVersion = "124"
	k8sPod.Spec = newPodSpec()
	k8sPod.Status.Phase = corev1.PodRunning
	k8sPod.Status.ContainerStatuses = []corev1.ContainerStatus{{
		Name:  "nginx",
		Ready: true,
		State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{StartedAt: startedAt}},
	}}

	fk8s := fake.NewClientset(k8sPod)
	c.client = fk8s
	c.PodController.client = fk8s.CoreV1()

	var reported *corev1.Pod
	wrapper := &syncProviderWrapper{PodLifecycleHandler: newSyncMockProvider()}
	wrapper.NotifyPods(ctx, func(pod *corev1.Pod) { reported = pod })
	c.provider = wrapper

	// The wrapped provider only knows about pods it was asked to create.
	assert.NilError(t, wrapper.CreatePod(ctx, k8sPod.DeepCopy()))
	assert.NilError(t, c.deletePod(ctx, k8sPod.DeepCopy()))
	assert.Assert(t, reported != nil, "the wrapper should have reported a terminal status")

	state := reported.Status.ContainerStatuses[0].State
	assert.Check(t, state.Terminated != nil, "the reported container should be terminated")
	assert.Check(t, is.Nil(state.Running), "a terminated container should not also be running")

	// And the same for what the controller writes to Kubernetes from that report.
	key := fmt.Sprintf("%s/%s", k8sPod.Namespace, k8sPod.Name)
	c.knownPods.Store(key, &knownPod{lastPodStatusReceivedFromProvider: reported})
	assert.NilError(t, c.updatePodStatus(ctx, k8sPod.DeepCopy(), key))

	got, err := c.client.CoreV1().Pods(k8sPod.Namespace).Get(ctx, k8sPod.Name, metav1.GetOptions{})
	assert.NilError(t, err)
	written := got.Status.ContainerStatuses[0].State
	assert.Check(t, written.Terminated != nil, "the written container status should be terminated")
	assert.Check(t, is.Nil(written.Running), "the written container status should not also be running")
	assert.Check(t, written.Terminated.StartedAt.Equal(&startedAt), "the terminated state should keep the start time")
}
