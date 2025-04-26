package node

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/virtual-kubelet/virtual-kubelet/errdefs"
	"github.com/virtual-kubelet/virtual-kubelet/log"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	mockProviderPodDeletedReason = "MockProviderPodDeleted"
)

var (
	_ PodLifecycleHandler    = (*mockProvider)(nil)
	_ PodUIDLifecycleHandler = (*mockProvider)(nil)
)

type mockProvider struct {
	creates          *waitableInt
	updates          *waitableInt
	deletes          *waitableInt
	attemptedDeletes *waitableInt

	errorOnDelete error

	pods         sync.Map
	startTime    time.Time
	realNotifier func(*v1.Pod)
}

// newMockProvider creates a new mockProvider.
func newMockProvider() *mockProviderAsync {
	provider := newSyncMockProvider()
	// By default notifier is set to a function which is a no-op. In the event we've implemented the PodNotifier interface,
	// it will be set, and then we'll call a real underlying implementation.
	// This makes it easier in the sense we don't need to wrap each method.
	return &mockProviderAsync{provider}
}

func newSyncMockProvider() *mockProvider {
	provider := mockProvider{
		startTime:        time.Now(),
		creates:          newWaitableInt(),
		updates:          newWaitableInt(),
		deletes:          newWaitableInt(),
		attemptedDeletes: newWaitableInt(),
	}
	return &provider
}

// notifier calls the callback that we got from the pod controller to notify it of updates (if it is set)
func (p *mockProvider) notifier(pod *v1.Pod) {
	if p.realNotifier != nil {
		p.realNotifier(pod)
	}
}

// CreatePod accepts a Pod definition and stores it in memory.
func (p *mockProvider) CreatePod(ctx context.Context, pod *v1.Pod) error {
	log.G(ctx).Infof("receive CreatePod %q", pod.Name)

	p.creates.increment()
	key, err := buildKey(pod)
	if err != nil {
		return err
	}

	now := metav1.NewTime(time.Now())
	pod.Status = v1.PodStatus{
		Phase:     v1.PodRunning,
		HostIP:    "1.2.3.4",
		PodIP:     "5.6.7.8",
		StartTime: &now,
		Conditions: []v1.PodCondition{
			{
				Type:   v1.PodInitialized,
				Status: v1.ConditionTrue,
			},
			{
				Type:   v1.PodReady,
				Status: v1.ConditionTrue,
			},
			{
				Type:   v1.PodScheduled,
				Status: v1.ConditionTrue,
			},
		},
	}

	for _, container := range pod.Spec.Containers {
		pod.Status.ContainerStatuses = append(pod.Status.ContainerStatuses, v1.ContainerStatus{
			Name:         container.Name,
			Image:        container.Image,
			Ready:        true,
			RestartCount: 0,
			State: v1.ContainerState{
				Running: &v1.ContainerStateRunning{
					StartedAt: now,
				},
			},
		})
	}

	p.pods.Store(key, pod)
	p.notifier(pod)

	return nil
}

// UpdatePod accepts a Pod definition and updates its reference.
func (p *mockProvider) UpdatePod(ctx context.Context, pod *v1.Pod) error {
	log.G(ctx).Infof("receive UpdatePod %q", pod.Name)

	p.updates.increment()
	key, err := buildKey(pod)
	if err != nil {
		return err
	}

	p.pods.Store(key, pod)
	p.notifier(pod)

	return nil
}

// DeletePod deletes the specified pod out of memory. The PodController deepcopies the pod object
// for us, so we don't have to worry about mutation.
func (p *mockProvider) DeletePod(ctx context.Context, pod *v1.Pod) (err error) {
	log.G(ctx).Infof("receive DeletePod %q", pod.Name)

	p.attemptedDeletes.increment()
	key, err := buildKey(pod)
	if err != nil {
		return err
	}

	if errdefs.IsNotFound(p.errorOnDelete) {
		p.pods.Delete(key)
	}
	if p.errorOnDelete != nil {
		return p.errorOnDelete
	}

	p.deletes.increment()

	if _, exists := p.pods.Load(key); !exists {
		return errdefs.NotFound("pod not found")
	}

	now := metav1.Now()

	pod.Status.Phase = v1.PodSucceeded
	pod.Status.Reason = mockProviderPodDeletedReason

	for idx := range pod.Status.ContainerStatuses {
		pod.Status.ContainerStatuses[idx].Ready = false
		pod.Status.ContainerStatuses[idx].State = v1.ContainerState{
			Terminated: &v1.ContainerStateTerminated{
				Message:    "Mock provider terminated container upon deletion",
				FinishedAt: now,
				Reason:     "MockProviderPodContainerDeleted",
				StartedAt:  pod.Status.ContainerStatuses[idx].State.Running.StartedAt,
			},
		}
	}

	p.notifier(pod)
	p.pods.Delete(key)

	return nil
}

func (p *mockProvider) GetPod(ctx context.Context, namespace, name string) (pod *v1.Pod, err error) {
	panic("GetPod not called when GetPodByUID implemented")
}

// GetPodByUID returns a pod by name that is stored in memory.
func (p *mockProvider) GetPodByUID(ctx context.Context, namespace, name string, uid types.UID) (pod *v1.Pod, err error) {
	log.G(ctx).Infof("receive GetPodByUID %q", name)

	key, err := buildKeyFromNames(namespace, name, uid)
	if err != nil {
		return nil, err
	}

	if pod, ok := p.pods.Load(key); ok {
		return pod.(*v1.Pod).DeepCopy(), nil
	}
	return nil, errdefs.NotFoundf("pod \"%s/%s\" is not known to the provider", namespace, name)
}

func (p *mockProvider) GetPodStatus(ctx context.Context, namespace, name string) (pod *v1.PodStatus, err error) {
	panic("GetPodStatus not called when GetPodByUID implemented")
}

// GetPodStatusByUID returns the status of a pod by name that is "running".
// returns nil if a pod by that name is not found.
func (p *mockProvider) GetPodStatusByUID(ctx context.Context, namespace, name string, uid types.UID) (*v1.PodStatus, error) {
	log.G(ctx).Infof("receive GetPodStatusByUID %q", name)

	pod, err := p.GetPodByUID(ctx, namespace, name, uid)
	if err != nil {
		return nil, err
	}

	return pod.Status.DeepCopy(), nil
}

// GetPods returns a list of all pods known to be "running".
func (p *mockProvider) GetPods(ctx context.Context) ([]*v1.Pod, error) {
	log.G(ctx).Info("receive GetPods")

	var pods []*v1.Pod

	p.pods.Range(func(key, pod interface{}) bool {
		pods = append(pods, pod.(*v1.Pod).DeepCopy())
		return true
	})

	return pods, nil
}

func (p *mockProvider) setErrorOnDelete(err error) {
	p.errorOnDelete = err
}

func (p *mockProvider) getAttemptedDeletes() *waitableInt {
	return p.attemptedDeletes
}

func (p *mockProvider) getCreates() *waitableInt {
	return p.creates
}

func (p *mockProvider) getDeletes() *waitableInt {
	return p.deletes
}

func (p *mockProvider) getUpdates() *waitableInt {
	return p.updates
}

func buildKeyFromNames(namespace string, name string, uid types.UID) (string, error) {
	return fmt.Sprintf("%s/%s/%s", namespace, name, uid), nil
}

// buildKey is a helper for building the "key" for the providers pod store.
func buildKey(pod *v1.Pod) (string, error) {
	if pod.ObjectMeta.Namespace == "" {
		return "", fmt.Errorf("pod namespace not found")
	}

	if pod.ObjectMeta.Name == "" {
		return "", fmt.Errorf("pod name not found")
	}

	if pod.ObjectMeta.UID == "" {
		return "", fmt.Errorf("pod UID not found")
	}

	return buildKeyFromNames(pod.ObjectMeta.Namespace, pod.ObjectMeta.Name, pod.ObjectMeta.UID)
}

type mockProviderAsync struct {
	*mockProvider
}

// NotifyPods is called to set a pod notifier callback function. This should be called before any operations are done
// within the provider.
func (p *mockProviderAsync) NotifyPods(ctx context.Context, notifier func(*v1.Pod)) {
	p.realNotifier = notifier
}

type testingProvider interface {
	PodLifecycleHandler
	setErrorOnDelete(error)
	getAttemptedDeletes() *waitableInt
	getDeletes() *waitableInt
	getCreates() *waitableInt
	getUpdates() *waitableInt
}
