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
	"time"

	testutil "github.com/virtual-kubelet/virtual-kubelet/internal/test/util"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/fake"
)

type TestController struct {
	*PodController
	mock   *mockProvider
	client *fake.Clientset
}

func newTestController(ctx context.Context) *TestController {
	fk8s := fake.NewSimpleClientset()

	rm := testutil.FakeResourceManager()
	p := newMockProvider()

	return &TestController{
		PodController: &PodController{
			client:          fk8s.CoreV1(),
			provider:        WrapLegacyPodLifecycleHandler(context.TODO(), p, 1*time.Millisecond),
			resourceManager: rm,
			recorder:        testutil.FakeEventRecorder(5),
		},
		mock:   p,
		client: fk8s,
	}
}

func TestPodCreateNewPod(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	svr := newTestController(ctx)

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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	svr := newTestController(ctx)

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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	svr := newTestController(ctx)

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
