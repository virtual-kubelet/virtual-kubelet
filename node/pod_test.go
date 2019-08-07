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
	"sync/atomic"
	"testing"

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

func TestPodCreateNewPod(t *testing.T) {
	svr := newTestController()

	pod := &corev1.Pod{}
	pod.ObjectMeta.Namespace = "default"
	pod.ObjectMeta.Name = "nginx"
	pod.Spec = corev1.PodSpec{
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

	err := svr.createOrUpdatePod(context.Background(), pod)

	assert.Check(t, is.Nil(err))
	// createOrUpdate called CreatePod but did not call UpdatePod because the pod did not exist
	assert.Check(t, is.Equal(atomic.LoadUint64(&svr.mock.creates), uint64(1)))
	assert.Check(t, is.Equal(atomic.LoadUint64(&svr.mock.updates), uint64(0)))
}

func TestPodUpdateExisting(t *testing.T) {
	svr := newTestController()

	pod := &corev1.Pod{}
	pod.ObjectMeta.Namespace = "default"
	pod.ObjectMeta.Name = "nginx"
	pod.Spec = corev1.PodSpec{
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

	err := svr.provider.CreatePod(context.Background(), pod.DeepCopy())
	assert.Check(t, is.Nil(err))
	assert.Check(t, is.Equal(atomic.LoadUint64(&svr.mock.creates), uint64(1)))
	assert.Check(t, is.Equal(atomic.LoadUint64(&svr.mock.updates), uint64(0)))

	pod2 := &corev1.Pod{}
	pod2.ObjectMeta.Namespace = "default"
	pod2.ObjectMeta.Name = "nginx"
	pod2.Spec = corev1.PodSpec{
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
	}

	err = svr.createOrUpdatePod(context.Background(), pod2)
	assert.Check(t, is.Nil(err))

	// createOrUpdate didn't call CreatePod but did call UpdatePod because the spec changed
	assert.Check(t, is.Equal(atomic.LoadUint64(&svr.mock.creates), uint64(1)))
	assert.Check(t, is.Equal(atomic.LoadUint64(&svr.mock.updates), uint64(1)))
}

func TestPodNoSpecChange(t *testing.T) {
	svr := newTestController()

	pod := &corev1.Pod{}
	pod.ObjectMeta.Namespace = "default"
	pod.ObjectMeta.Name = "nginx"
	pod.Spec = corev1.PodSpec{
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

	err := svr.createOrUpdatePod(context.Background(), pod)
	assert.Check(t, is.Nil(err))
	assert.Check(t, is.Equal(atomic.LoadUint64(&svr.mock.creates), uint64(1)))
	assert.Check(t, is.Equal(atomic.LoadUint64(&svr.mock.updates), uint64(0)))

	err = svr.createOrUpdatePod(context.Background(), pod)
	assert.Check(t, is.Nil(err))

	// createOrUpdate didn't call CreatePod or UpdatePod, spec didn't change
	assert.Check(t, is.Equal(atomic.LoadUint64(&svr.mock.creates), uint64(1)))
	assert.Check(t, is.Equal(atomic.LoadUint64(&svr.mock.updates), uint64(0)))
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
			pod.Spec = corev1.PodSpec{
				Containers: []corev1.Container{
					corev1.Container{
						Name:  "nginx",
						Image: "nginx:1.15.12",
					},
				},
			}

			pc := c.client.CoreV1().Pods("default")

			p, err := pc.Create(pod)
			assert.NilError(t, err)

			ctx := context.Background()
			err = c.createOrUpdatePod(ctx, p) // make sure it's actually created
			assert.NilError(t, err)
			assert.Check(t, is.Equal(atomic.LoadUint64(&c.mock.creates), uint64(1)))

			err = c.deletePod(ctx, pod.Namespace, pod.Name)
			assert.Equal(t, pkgerrors.Cause(err), err)

			var expectDeletes int
			if tc.delErr == nil {
				expectDeletes = 1
			}
			assert.Check(t, is.Equal(atomic.LoadUint64(&c.mock.deletes), uint64(expectDeletes)))

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
