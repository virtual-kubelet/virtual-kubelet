package node

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/virtual-kubelet/virtual-kubelet/errdefs"
	"github.com/virtual-kubelet/virtual-kubelet/log"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	_ PodLifecycleHandler = (*mockV0Provider)(nil)
	_ PodNotifier         = (*mockProvider)(nil)
)

type mockV0Provider struct {
	creates          uint64
	updates          uint64
	deletes          uint64
	attemptedDeletes uint64

	errorOnDelete error

	pods         sync.Map
	startTime    time.Time
	realNotifier func(*v1.Pod)
}

type mockProvider struct {
	*mockV0Provider
}

// NewMockProviderMockConfig creates a new mockV0Provider. Mock legacy provider does not implement the new asynchronous podnotifier interface
func newMockV0Provider() *mockV0Provider {
	provider := mockV0Provider{
		startTime: time.Now(),
	}
	// By default notifier is set to a function which is a no-op. In the event we've implemented the PodNotifier interface,
	// it will be set, and then we'll call a real underlying implementation.
	// This makes it easier in the sense we don't need to wrap each method.

	return &provider
}

// NewMockProviderMockConfig creates a new MockProvider with the given config
func newMockProvider() *mockProvider {
	return &mockProvider{mockV0Provider: newMockV0Provider()}
}

// notifier calls the callback that we got from the pod controller to notify it of updates (if it is set)
func (p *mockV0Provider) notifier(pod *v1.Pod) {
	if p.realNotifier != nil {
		p.realNotifier(pod)
	}
}

// CreatePod accepts a Pod definition and stores it in memory.
func (p *mockV0Provider) CreatePod(ctx context.Context, pod *v1.Pod) error {
	log.G(ctx).Infof("receive CreatePod %q", pod.Name)

	atomic.AddUint64(&p.creates, 1)
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
func (p *mockV0Provider) UpdatePod(ctx context.Context, pod *v1.Pod) error {
	log.G(ctx).Infof("receive UpdatePod %q", pod.Name)

	atomic.AddUint64(&p.updates, 1)
	key, err := buildKey(pod)
	if err != nil {
		return err
	}

	p.pods.Store(key, pod)
	p.notifier(pod)

	return nil
}

// DeletePod deletes the specified pod out of memory.
func (p *mockV0Provider) DeletePod(ctx context.Context, pod *v1.Pod) (err error) {
	log.G(ctx).Infof("receive DeletePod %q", pod.Name)

	atomic.AddUint64(&p.attemptedDeletes, 1)
	if p.errorOnDelete != nil {
		return p.errorOnDelete
	}

	atomic.AddUint64(&p.deletes, 1)
	key, err := buildKey(pod)
	if err != nil {
		return err
	}

	if _, exists := p.pods.Load(key); !exists {
		return errdefs.NotFound("pod not found")
	}

	now := metav1.Now()

	pod.Status.Phase = v1.PodSucceeded
	pod.Status.Reason = "MockProviderPodDeleted"

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
	if p.realNotifier == nil {
		// The pods reconciliation (GetPodStatus) should be called in under 1 minute
		time.AfterFunc(time.Minute, func() {
			p.pods.Delete(key)
		})
	} else {
		p.pods.Delete(key)
	}
	return nil
}

// GetPod returns a pod by name that is stored in memory.
func (p *mockV0Provider) GetPod(ctx context.Context, namespace, name string) (pod *v1.Pod, err error) {
	log.G(ctx).Infof("receive GetPod %q", name)

	key, err := buildKeyFromNames(namespace, name)
	if err != nil {
		return nil, err
	}

	if pod, ok := p.pods.Load(key); ok {
		return pod.(*v1.Pod), nil
	}
	return nil, errdefs.NotFoundf("pod \"%s/%s\" is not known to the provider", namespace, name)
}

// GetPodStatus returns the status of a pod by name that is "running".
// returns nil if a pod by that name is not found.
func (p *mockV0Provider) GetPodStatus(ctx context.Context, namespace, name string) (*v1.PodStatus, error) {
	log.G(ctx).Infof("receive GetPodStatus %q", name)

	pod, err := p.GetPod(ctx, namespace, name)
	if err != nil {
		return nil, err
	}

	return &pod.Status, nil
}

// GetPods returns a list of all pods known to be "running".
func (p *mockV0Provider) GetPods(ctx context.Context) ([]*v1.Pod, error) {
	log.G(ctx).Info("receive GetPods")

	var pods []*v1.Pod

	p.pods.Range(func(key, pod interface{}) bool {
		pods = append(pods, pod.(*v1.Pod))
		return true
	})

	return pods, nil
}

// NotifyPods is called to set a pod notifier callback function. This should be called before any operations are done
// within the provider.
func (p *mockProvider) NotifyPods(ctx context.Context, notifier func(*v1.Pod)) {
	p.realNotifier = notifier
}

func buildKeyFromNames(namespace string, name string) (string, error) {
	return fmt.Sprintf("%s-%s", namespace, name), nil
}

// buildKey is a helper for building the "key" for the providers pod store.
func buildKey(pod *v1.Pod) (string, error) {
	if pod.ObjectMeta.Namespace == "" {
		return "", fmt.Errorf("pod namespace not found")
	}

	if pod.ObjectMeta.Name == "" {
		return "", fmt.Errorf("pod name not found")
	}

	return buildKeyFromNames(pod.ObjectMeta.Namespace, pod.ObjectMeta.Name)
}
