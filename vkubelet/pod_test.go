package vkubelet

import (
	"context"
	"testing"

	"github.com/virtual-kubelet/virtual-kubelet/providers/mock"
	testutil "github.com/virtual-kubelet/virtual-kubelet/test/util"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/fake"
)

type FakeProvider struct {
	*mock.MockProvider
	createFn func()
	updateFn func()
}

func (f *FakeProvider) CreatePod(ctx context.Context, pod *corev1.Pod) error {
	f.createFn()
	return f.MockProvider.CreatePod(ctx, pod)
}

func (f *FakeProvider) UpdatePod(ctx context.Context, pod *corev1.Pod) error {
	f.updateFn()
	return f.MockProvider.CreatePod(ctx, pod)
}

type TestServer struct {
	*Server
	mock   *FakeProvider
	client *fake.Clientset
}

func newMockProvider(t *testing.T) (*mock.MockProvider, error) {
	return mock.NewMockProviderMockConfig(
		mock.MockConfig{},
		"vk123",
		"linux",
		"127.0.0.1",
		443,
	)
}

func newTestServer(t *testing.T) *TestServer {

	mockProvider, err := newMockProvider(t)
	assert.Check(t, is.Nil(err))

	fk8s := fake.NewSimpleClientset()

	fakeProvider := &FakeProvider{
		MockProvider: mockProvider,
	}

	rm := testutil.FakeResourceManager()

	tsvr := &TestServer{
		Server: &Server{
			namespace:       "default",
			nodeName:        "vk123",
			provider:        fakeProvider,
			resourceManager: rm,
			k8sClient:       fk8s,
		},
		mock:   fakeProvider,
		client: fk8s,
	}
	return tsvr
}

func TestPodHashingEqual(t *testing.T) {
	p1 := corev1.PodSpec{
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

	h1 := hashPodSpec(p1)

	p2 := corev1.PodSpec{
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

	h2 := hashPodSpec(p2)
	assert.Check(t, is.Equal(h1, h2))
}

func TestPodHashingDifferent(t *testing.T) {
	p1 := corev1.PodSpec{
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

	h1 := hashPodSpec(p1)

	p2 := corev1.PodSpec{
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

	h2 := hashPodSpec(p2)
	assert.Check(t, h1 != h2)
}

func TestPodCreateNewPod(t *testing.T) {
	svr := newTestServer(t)

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

	created := false
	updated := false
	// The pod doesn't exist, we should invoke the CreatePod() method of the provider
	svr.mock.createFn = func() {
		created = true
	}
	svr.mock.updateFn = func() {
		updated = true
	}
	er := testutil.FakeEventRecorder(5)
	err := svr.createOrUpdatePod(context.Background(), pod, er)
	assert.Check(t, is.Nil(err))
	// createOrUpdate called CreatePod but did not call UpdatePod because the pod did not exist
	assert.Check(t, created)
	assert.Check(t, !updated)
}

func TestPodUpdateExisting(t *testing.T) {
	svr := newTestServer(t)

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

	err := svr.mock.MockProvider.CreatePod(context.Background(), pod)
	assert.Check(t, is.Nil(err))
	created := false
	updated := false
	// The pod doesn't exist, we should invoke the CreatePod() method of the provider
	svr.mock.createFn = func() {
		created = true
	}
	svr.mock.updateFn = func() {
		updated = true
	}

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

	er := testutil.FakeEventRecorder(5)
	err = svr.createOrUpdatePod(context.Background(), pod2, er)
	assert.Check(t, is.Nil(err))

	// createOrUpdate didn't call CreatePod but did call UpdatePod because the spec changed
	assert.Check(t, !created)
	assert.Check(t, updated)
}

func TestPodNoSpecChange(t *testing.T) {
	svr := newTestServer(t)

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

	err := svr.mock.MockProvider.CreatePod(context.Background(), pod)
	assert.Check(t, is.Nil(err))
	created := false
	updated := false
	// The pod doesn't exist, we should invoke the CreatePod() method of the provider
	svr.mock.createFn = func() {
		created = true
	}
	svr.mock.updateFn = func() {
		updated = true
	}

	er := testutil.FakeEventRecorder(5)
	err = svr.createOrUpdatePod(context.Background(), pod, er)
	assert.Check(t, is.Nil(err))

	// createOrUpdate didn't call CreatePod or UpdatePod, spec didn't change
	assert.Check(t, !created)
	assert.Check(t, !updated)
}
