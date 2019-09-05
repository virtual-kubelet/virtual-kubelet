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
	"testing"
	"time"

	pkgerrors "github.com/pkg/errors"
	"github.com/virtual-kubelet/virtual-kubelet/errdefs"
	testutil "github.com/virtual-kubelet/virtual-kubelet/internal/test/util"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

type TestController struct {
	*PodController
	mock   *mockProvider
	client *fake.Clientset
}

func newTestController() *TestController {
	fk8s := fake.NewSimpleClientset()

	rm := testutil.FakeResourceManager()
	p := newMockProvider()

	return &TestController{
		PodController: &PodController{
			client:          fk8s.CoreV1(),
			provider:        p,
			resourceManager: rm,
			recorder:        testutil.FakeEventRecorder(5),
		},
		mock:   p,
		client: fk8s,
	}
}

func TestPodsEqual(t *testing.T) {
	p1 := &corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				corev1.Container{
					Name:  "nginx",
					Image: "nginx:1.15.12-perl",
					Ports: []corev1.ContainerPort{
						corev1.ContainerPort{
							ContainerPort: 443,
							Protocol:      "tcp",
						},
					},
				},
			},
		},
	}

	p2 := p1.DeepCopy()

	assert.Assert(t, podsEqual(p1, p2))
}

func TestPodsDifferent(t *testing.T) {
	p1 := &corev1.Pod{
		Spec: newPodSpec(),
	}

	p2 := p1.DeepCopy()
	p2.Spec.Containers[0].Image = "nginx:1.15.12-perl"

	assert.Assert(t, !podsEqual(p1, p2))
}

func TestPodsDifferentIgnoreValue(t *testing.T) {
	p1 := &corev1.Pod{
		Spec: newPodSpec(),
	}

	p2 := p1.DeepCopy()
	p2.Status.Phase = corev1.PodFailed

	assert.Assert(t, podsEqual(p1, p2))
}

func TestPodCreateNewPod(t *testing.T) {
	svr := newTestController()

	pod := &corev1.Pod{}
	pod.ObjectMeta.Namespace = "default" //nolint:goconst
	pod.ObjectMeta.Name = "nginx"        //nolint:goconst
	pod.Spec = newPodSpec()

	err := svr.createOrUpdatePod(context.Background(), pod.DeepCopy())

	assert.Check(t, is.Nil(err))
	// createOrUpdate called CreatePod but did not call UpdatePod because the pod did not exist
	assert.Check(t, is.Equal(svr.mock.creates.read(), 1))
	assert.Check(t, is.Equal(svr.mock.updates.read(), 0))
}

func TestPodUpdateExisting(t *testing.T) {
	svr := newTestController()

	pod := &corev1.Pod{}
	pod.ObjectMeta.Namespace = "default"
	pod.ObjectMeta.Name = "nginx"
	pod.Spec = newPodSpec()

	err := svr.createOrUpdatePod(context.Background(), pod.DeepCopy())
	assert.Check(t, is.Nil(err))
	assert.Check(t, is.Equal(svr.mock.creates.read(), 1))
	assert.Check(t, is.Equal(svr.mock.updates.read(), 0))

	pod2 := pod.DeepCopy()
	pod2.Spec.Containers[0].Image = "nginx:1.15.12-perl"

	err = svr.createOrUpdatePod(context.Background(), pod2.DeepCopy())
	assert.Check(t, is.Nil(err))

	// createOrUpdate didn't call CreatePod but did call UpdatePod because the spec changed
	assert.Check(t, is.Equal(svr.mock.creates.read(), 1))
	assert.Check(t, is.Equal(svr.mock.updates.read(), 1))
}

func TestPodNoSpecChange(t *testing.T) {
	svr := newTestController()

	pod := &corev1.Pod{}
	pod.ObjectMeta.Namespace = "default"
	pod.ObjectMeta.Name = "nginx"
	pod.Spec = newPodSpec()

	err := svr.createOrUpdatePod(context.Background(), pod.DeepCopy())
	assert.Check(t, is.Nil(err))
	assert.Check(t, is.Equal(svr.mock.creates.read(), 1))
	assert.Check(t, is.Equal(svr.mock.updates.read(), 0))

	err = svr.createOrUpdatePod(context.Background(), pod.DeepCopy())
	assert.Check(t, is.Nil(err))

	// createOrUpdate didn't call CreatePod or UpdatePod, spec didn't change
	assert.Check(t, is.Equal(svr.mock.creates.read(), 1))
	assert.Check(t, is.Equal(svr.mock.updates.read(), 0))
}

func TestPodDelete(t *testing.T) {
	type testCase struct {
		desc   string
		delErr error
	}

	cases := []testCase{
		{desc: "no error on delete", delErr: nil},
		{desc: "not found error on delete", delErr: errdefs.NotFound("not found")},
		{desc: "unknown error on delete", delErr: pkgerrors.New("random error")},
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			c := newTestController()
			c.mock.errorOnDelete = tc.delErr

			pod := &corev1.Pod{}
			pod.ObjectMeta.Namespace = "default"
			pod.ObjectMeta.Name = "nginx"
			pod.Spec = newPodSpec()

			pc := c.client.CoreV1().Pods("default")

			p, err := pc.Create(pod)
			assert.NilError(t, err)

			ctx := context.Background()
			err = c.createOrUpdatePod(ctx, p.DeepCopy()) // make sure it's actually created
			assert.NilError(t, err)
			assert.Check(t, is.Equal(c.mock.creates.read(), 1))

			err = c.deletePod(ctx, pod.Namespace, pod.Name)
			assert.Equal(t, pkgerrors.Cause(err), err)

			var expectDeletes int
			if tc.delErr == nil {
				expectDeletes = 1
			}
			assert.Check(t, is.Equal(c.mock.deletes.read(), expectDeletes))

			expectDeleted := tc.delErr == nil || errdefs.IsNotFound(tc.delErr)

			_, err = pc.Get(pod.Name, metav1.GetOptions{})
			if expectDeleted {
				assert.Assert(t, errors.IsNotFound(err))
			} else {
				assert.NilError(t, err)
			}
		})
	}
}

func TestPodUpdateStatus(t *testing.T) {
	startedAt := metav1.NewTime(time.Now())
	finishedAt := metav1.NewTime(startedAt.Add(time.Second * 10))
	containerStateRunning := &corev1.ContainerStateRunning{StartedAt: startedAt}
	containerStateTerminated := &corev1.ContainerStateTerminated{StartedAt: startedAt, FinishedAt: finishedAt}
	containerStateWaiting := &corev1.ContainerStateWaiting{}

	testCases := []struct {
		desc               string
		running            *corev1.ContainerStateRunning
		terminated         *corev1.ContainerStateTerminated
		waiting            *corev1.ContainerStateWaiting
		expectedStartedAt  metav1.Time
		expectedFinishedAt metav1.Time
	}{
		{desc: "container in running state", running: containerStateRunning, expectedStartedAt: startedAt},
		{desc: "container in terminated state", terminated: containerStateTerminated, expectedStartedAt: startedAt, expectedFinishedAt: finishedAt},
		{desc: "container in waiting state", waiting: containerStateWaiting},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			c := newTestController()
			pod := &corev1.Pod{}
			pod.ObjectMeta.Namespace = "default"
			pod.ObjectMeta.Name = "nginx"
			pod.Status.Phase = corev1.PodRunning
			containerStatus := corev1.ContainerStatus{}
			if tc.running != nil {
				containerStatus.State.Running = tc.running
			} else if tc.terminated != nil {
				containerStatus.State.Terminated = tc.terminated
			} else if tc.waiting != nil {
				containerStatus.State.Waiting = tc.waiting
			}
			pod.Status.ContainerStatuses = []corev1.ContainerStatus{containerStatus}

			pc := c.client.CoreV1().Pods("default")
			p, err := pc.Create(pod)
			assert.NilError(t, err)

			ctx := context.Background()
			err = c.updatePodStatus(ctx, p.DeepCopy())
			assert.NilError(t, err)

			updatedPod, err := pc.Get(pod.Name, metav1.GetOptions{})
			assert.Equal(t, updatedPod.Status.Phase, corev1.PodFailed)
			assert.Assert(t, is.Len(updatedPod.Status.ContainerStatuses, 1))
			assert.Assert(t, updatedPod.Status.ContainerStatuses[0].State.Running == nil)

			// Test cases for running and terminated container state
			if tc.running != nil || tc.terminated != nil {
				// Ensure that the container is in terminated state and other states are nil
				assert.Assert(t, updatedPod.Status.ContainerStatuses[0].State.Terminated != nil)
				assert.Assert(t, updatedPod.Status.ContainerStatuses[0].State.Waiting == nil)

				terminated := updatedPod.Status.ContainerStatuses[0].State.Terminated
				assert.Equal(t, terminated.StartedAt, tc.expectedStartedAt)
				assert.Assert(t, terminated.StartedAt.Before(&terminated.FinishedAt))
				if tc.terminated != nil {
					assert.Equal(t, terminated.FinishedAt, tc.expectedFinishedAt)
				}
			} else {
				// Test case for waiting container state
				assert.Assert(t, updatedPod.Status.ContainerStatuses[0].State.Terminated == nil)
				assert.Assert(t, updatedPod.Status.ContainerStatuses[0].State.Waiting != nil)
			}
		})
	}
}

func newPodSpec() corev1.PodSpec {
	return corev1.PodSpec{
		Containers: []corev1.Container{
			corev1.Container{
				Name:  "nginx",
				Image: "nginx:1.15.12",
				Ports: []corev1.ContainerPort{
					corev1.ContainerPort{
						ContainerPort: 443,
						Protocol:      "tcp",
					},
				},
			},
		},
	}
}
