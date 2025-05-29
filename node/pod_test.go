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
	"reflect"
	"testing"
	"time"

	"golang.org/x/time/rate"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	core "k8s.io/client-go/testing"
	"k8s.io/client-go/util/workqueue"

	testutil "github.com/virtual-kubelet/virtual-kubelet/internal/test/util"
)

type TestController struct {
	*PodController
	mock   *mockProviderAsync
	client *fake.Clientset
}

func newTestController() *TestController {
	fk8s := fake.NewSimpleClientset()

	rm := testutil.FakeResourceManager()
	p := newMockProvider()
	iFactory := kubeinformers.NewSharedInformerFactoryWithOptions(fk8s, 10*time.Minute)
	podController, err := NewPodController(PodControllerConfig{
		PodClient:         fk8s.CoreV1(),
		PodInformer:       iFactory.Core().V1().Pods(),
		EventRecorder:     testutil.FakeEventRecorder(5),
		Provider:          p,
		ConfigMapInformer: iFactory.Core().V1().ConfigMaps(),
		SecretInformer:    iFactory.Core().V1().Secrets(),
		ServiceInformer:   iFactory.Core().V1().Services(),
		SyncPodsFromKubernetesRateLimiter: workqueue.NewTypedMaxOfRateLimiter[any](
			// The default upper bound is 1000 seconds. Let's not use that.
			workqueue.NewTypedItemExponentialFailureRateLimiter[any](5*time.Millisecond, 10*time.Millisecond),
			&workqueue.TypedBucketRateLimiter[any]{Limiter: rate.NewLimiter(rate.Limit(10), 100)},
		),
		SyncPodStatusFromProviderRateLimiter: workqueue.NewTypedMaxOfRateLimiter[any](
			// The default upper bound is 1000 seconds. Let's not use that.
			workqueue.NewTypedItemExponentialFailureRateLimiter[any](5*time.Millisecond, 10*time.Millisecond),
			&workqueue.TypedBucketRateLimiter[any]{Limiter: rate.NewLimiter(rate.Limit(10), 100)},
		),
		DeletePodsFromKubernetesRateLimiter: workqueue.NewTypedMaxOfRateLimiter[any](
			// The default upper bound is 1000 seconds. Let's not use that.
			workqueue.NewTypedItemExponentialFailureRateLimiter[any](5*time.Millisecond, 10*time.Millisecond),
			&workqueue.TypedBucketRateLimiter[any]{Limiter: rate.NewLimiter(rate.Limit(10), 100)},
		),
	})

	if err != nil {
		panic(err)
	}
	// Override the resource manager in the contructor with our own.
	podController.resourceManager = rm

	return &TestController{
		PodController: podController,
		mock:          p,
		client:        fk8s,
	}
}

// Run starts the informer and runs the pod controller
func (tc *TestController) Run(ctx context.Context, n int) error {
	go tc.podsInformer.Informer().Run(ctx.Done())
	return tc.PodController.Run(ctx, n)
}

func TestPodsEqual(t *testing.T) {
	p1 := &corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "nginx",
					Image: "nginx:1.15.12-perl",
					Ports: []corev1.ContainerPort{
						{
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

func TestPodShouldEnqueueDifferentDeleteTimeStamp(t *testing.T) {
	p1 := &corev1.Pod{
		Spec: newPodSpec(),
	}

	p2 := p1.DeepCopy()
	now := v1.NewTime(time.Now())
	p2.DeletionTimestamp = &now
	assert.Assert(t, podShouldEnqueue(p1, p2))
}

func TestPodShouldEnqueueDifferentLabel(t *testing.T) {
	p1 := &corev1.Pod{
		Spec: newPodSpec(),
	}

	p2 := p1.DeepCopy()
	p2.Labels = map[string]string{"test": "test"}
	assert.Assert(t, podShouldEnqueue(p1, p2))
}

func TestPodShouldEnqueueDifferentAnnotation(t *testing.T) {
	p1 := &corev1.Pod{
		Spec: newPodSpec(),
	}

	p2 := p1.DeepCopy()
	p2.Annotations = map[string]string{"test": "test"}
	assert.Assert(t, podShouldEnqueue(p1, p2))
}

func TestPodShouldNotEnqueueDifferentStatus(t *testing.T) {
	p1 := &corev1.Pod{
		Spec: newPodSpec(),
	}

	p2 := p1.DeepCopy()
	p2.Status.Phase = corev1.PodSucceeded
	assert.Assert(t, !podShouldEnqueue(p1, p2))
}

func TestPodShouldEnqueueDifferentDeleteGraceTime(t *testing.T) {
	p1 := &corev1.Pod{
		Spec: newPodSpec(),
	}

	p2 := p1.DeepCopy()
	oldTime := v1.NewTime(time.Now().Add(5))
	newTime := v1.NewTime(time.Now().Add(10))
	oldGraceTime := int64(5)
	newGraceTime := int64(10)
	p1.DeletionGracePeriodSeconds = &oldGraceTime
	p2.DeletionTimestamp = &oldTime

	p2.DeletionGracePeriodSeconds = &newGraceTime
	p2.DeletionTimestamp = &newTime
	assert.Assert(t, podShouldEnqueue(p1, p2))
}

func TestPodShouldEnqueueGraceTimeChanged(t *testing.T) {
	p1 := &corev1.Pod{
		Spec: newPodSpec(),
	}

	p2 := p1.DeepCopy()
	graceTime := int64(30)
	p2.DeletionGracePeriodSeconds = &graceTime
	assert.Assert(t, podShouldEnqueue(p1, p2))
}

func TestPodCreateNewPod(t *testing.T) {
	svr := newTestController()

	pod := &corev1.Pod{}
	pod.Namespace = "default" //nolint:goconst
	pod.Name = "nginx"        //nolint:goconst
	pod.Spec = newPodSpec()

	err := svr.createOrUpdatePod(context.Background(), pod.DeepCopy())

	assert.Check(t, is.Nil(err))
	// createOrUpdate called CreatePod but did not call UpdatePod because the pod did not exist
	assert.Check(t, is.Equal(svr.mock.creates.read(), 1))
	assert.Check(t, is.Equal(svr.mock.updates.read(), 0))
}

func TestPodCreateNewPodWithNoDownwardAPIResolution(t *testing.T) {
	svr := newTestController()
	svr.skipDownwardAPIResolution = true

	pod := &corev1.Pod{}
	pod.Namespace = "default" //nolint:goconst
	pod.Name = "nginx"        //nolint:goconst
	pod.Spec = newPodSpec()
	pod.Spec.Containers[0].Env = []corev1.EnvVar{
		{
			Name: "MY_NODE_NAME",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "spec.nodeName",
				},
			},
		},
	}

	err := svr.createOrUpdatePod(context.Background(), pod.DeepCopy())
	assert.Check(t, is.Nil(err))

	// createOrUpdate called CreatePod but did not call UpdatePod because the pod did not exist
	assert.Check(t, is.Equal(svr.mock.creates.read(), 1))
	assert.Check(t, is.Equal(svr.mock.updates.read(), 0))

	// make sure the downward API field in env was left alone
	key, err := buildKey(pod)
	assert.Check(t, is.Nil(err))
	createdPod, ok := svr.mock.pods.Load(key)
	assert.Check(t, ok)

	// createdPod went through the pod controller logic, make sure the downward API wasn't resolved
	assert.Check(t, is.DeepEqual(createdPod.(*corev1.Pod).Spec.Containers[0].Env, pod.Spec.Containers[0].Env))
}

func TestPodUpdateExisting(t *testing.T) {
	svr := newTestController()

	pod := &corev1.Pod{}
	pod.Namespace = "default"
	pod.Name = "nginx"
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
	pod.Namespace = "default"
	pod.Name = "nginx"
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

func TestPodStatusDelete(t *testing.T) {
	ctx := context.Background()
	c := newTestController()
	pod := &corev1.Pod{}
	pod.Namespace = "default"
	pod.Name = "nginx"
	pod.Spec = newPodSpec()
	fk8s := fake.NewSimpleClientset(pod)
	c.client = fk8s
	c.PodController.client = fk8s.CoreV1()
	podCopy := pod.DeepCopy()
	deleteTime := v1.Time{Time: time.Now().Add(30 * time.Second)}
	podCopy.DeletionTimestamp = &deleteTime
	key := fmt.Sprintf("%s/%s", pod.Namespace, pod.Name)
	c.knownPods.Store(key, &knownPod{lastPodStatusReceivedFromProvider: podCopy})

	// test pod in provider delete
	err := c.updatePodStatus(ctx, pod, key)
	if err != nil {
		t.Fatal("pod updated failed")
	}
	newPod, err := c.client.CoreV1().Pods(pod.Namespace).Get(ctx, pod.Name, v1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		t.Fatalf("Get pod %v failed", key)
	}

	if newPod != nil && !reflect.ValueOf(*newPod).IsZero() && newPod.DeletionTimestamp == nil {
		t.Fatalf("Pod %v delete failed", key)
	}
	t.Logf("pod delete success")

	// test pod in provider delete
	pod.DeletionTimestamp = &deleteTime
	if _, err = c.client.CoreV1().Pods(pod.Namespace).Create(ctx, pod, v1.CreateOptions{}); err != nil {
		t.Fatal("Parepare pod in k8s failed")
	}
	podCopy.Status.ContainerStatuses = []corev1.ContainerStatus{
		{
			State: corev1.ContainerState{
				Waiting: nil,
				Running: nil,
				Terminated: &corev1.ContainerStateTerminated{
					ExitCode: 1,
					Message:  "Exit",
				},
			},
		},
	}
	c.knownPods.Store(key, &knownPod{lastPodStatusReceivedFromProvider: podCopy})
	err = c.updatePodStatus(ctx, pod, key)
	if err != nil {
		t.Fatalf("pod updated failed %v", err)
	}
	newPod, err = c.client.CoreV1().Pods(pod.Namespace).Get(ctx, pod.Name, v1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		t.Fatalf("Get pod %v failed", key)
	}
	if newPod.DeletionTimestamp == nil {
		t.Fatalf("Pod %v delete failed", key)
	}
	if newPod.Status.ContainerStatuses[0].State.Terminated == nil {
		t.Fatalf("Pod status %v update failed", key)
	}
	t.Logf("pod updated, container status: %+v, pod delete Time: %v", newPod.Status.ContainerStatuses[0].State.Terminated, newPod.DeletionTimestamp)
}

func TestReCreatePodRace(t *testing.T) {
	ctx := context.Background()
	c := newTestController()
	pod := &corev1.Pod{}
	pod.Namespace = "default"
	pod.Name = "nginx"
	pod.Spec = newPodSpec()
	pod.UID = "aaaaa"
	podCopy := pod.DeepCopy()
	podCopy.UID = "bbbbb"

	// test conflict
	fk8s := &fake.Clientset{}
	c.client = fk8s
	c.PodController.client = fk8s.CoreV1()
	key := fmt.Sprintf("%s/%s/%s", pod.Namespace, pod.Name, pod.UID)
	c.knownPods.Store(key, &knownPod{lastPodStatusReceivedFromProvider: podCopy})
	c.deletePodsFromKubernetes.Enqueue(ctx, key)
	if err := c.podsInformer.Informer().GetStore().Add(pod); err != nil {
		t.Fatal(err)
	}
	c.client.AddReactor("delete", "pods", func(action core.Action) (handled bool, ret runtime.Object, err error) {
		name := action.(core.DeleteAction).GetName()
		t.Logf("deleted pod %s", name)
		return true, nil, errors.NewConflict(schema.GroupResource{Group: "", Resource: "pods"}, "nginx", fmt.Errorf("test conflict"))
	})
	c.client.AddReactor("get", "pods", func(action core.Action) (handled bool, ret runtime.Object, err error) {
		name := action.(core.GetAction).GetName()
		t.Logf("get pod %s", name)
		return true, podCopy, nil
	})

	err := c.deletePodsFromKubernetesHandler(ctx, key)
	if err != nil {
		t.Error("Failed")
	}
	p, err := c.client.CoreV1().Pods(podCopy.Namespace).Get(ctx, podCopy.Name, v1.GetOptions{})
	if err != nil {
		t.Fatalf("Pod not exist, %v", err)
	}
	if p.UID != podCopy.UID {
		t.Errorf("Desired uid: %v, get: %v", podCopy.UID, p.UID)
	}
	t.Log("pod conflict test success")

	// test not found
	c = newTestController()
	fk8s = &fake.Clientset{}
	c.client = fk8s
	c.knownPods.Store(key, &knownPod{lastPodStatusReceivedFromProvider: podCopy})
	c.deletePodsFromKubernetes.Enqueue(ctx, key)
	if err = c.podsInformer.Informer().GetStore().Add(pod); err != nil {
		t.Fatal(err)
	}
	c.client.AddReactor("delete", "pods", func(action core.Action) (handled bool, ret runtime.Object, err error) {
		name := action.(core.DeleteAction).GetName()
		t.Logf("deleted pod %s", name)
		return true, nil, errors.NewNotFound(schema.GroupResource{Group: "", Resource: "pods"}, "nginx")
	})

	c.client.AddReactor("get", "pods", func(action core.Action) (handled bool, ret runtime.Object, err error) {
		name := action.(core.GetAction).GetName()
		t.Logf("get pod %s", name)
		return true, nil, errors.NewNotFound(schema.GroupResource{Group: "", Resource: "pods"}, "nginx")
	})

	err = c.deletePodsFromKubernetesHandler(ctx, key)
	if err != nil {
		t.Error("Failed")
	}
	_, err = c.client.CoreV1().Pods(podCopy.Namespace).Get(ctx, podCopy.Name, v1.GetOptions{})
	if err == nil {
		t.Log("delete success")
		return
	}
	if !errors.IsNotFound(err) {
		t.Fatal("Desired pod not exist")
	}
	t.Log("pod not found test success")

	// test uid not equal before query
	c = newTestController()
	fk8s = &fake.Clientset{}
	c.client = fk8s
	c.knownPods.Store(key, &knownPod{lastPodStatusReceivedFromProvider: podCopy})
	c.deletePodsFromKubernetes.Enqueue(ctx, key)
	// add new pod
	if err = c.podsInformer.Informer().GetStore().Add(podCopy); err != nil {
		t.Fatal(err)
	}
	c.client.AddReactor("delete", "pods", func(action core.Action) (handled bool, ret runtime.Object, err error) {
		name := action.(core.DeleteAction).GetName()
		t.Logf("deleted pod %s", name)
		return true, nil, errors.NewNotFound(schema.GroupResource{Group: "", Resource: "pods"}, "nginx")
	})

	c.client.AddReactor("get", "pods", func(action core.Action) (handled bool, ret runtime.Object, err error) {
		name := action.(core.GetAction).GetName()
		t.Logf("get pod %s", name)
		return true, nil, errors.NewNotFound(schema.GroupResource{Group: "", Resource: "pods"}, "nginx")
	})

	err = c.deletePodsFromKubernetesHandler(ctx, key)
	if err != nil {
		t.Error("Failed")
	}
	_, err = c.client.CoreV1().Pods(podCopy.Namespace).Get(ctx, podCopy.Name, v1.GetOptions{})
	if err == nil {
		t.Log("delete success")
		return
	}
	if !errors.IsNotFound(err) {
		t.Fatal("Desired pod not exist")
	}
	t.Log("pod uid conflict test success")
}

func newPodSpec() corev1.PodSpec {
	return corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name:  "nginx",
				Image: "nginx:1.15.12",
				Ports: []corev1.ContainerPort{
					{
						ContainerPort: 443,
						Protocol:      "tcp",
					},
				},
			},
		},
	}
}
