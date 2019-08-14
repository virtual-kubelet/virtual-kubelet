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
)

var (
	_ PodLifecycleHandler = (*mockV0Provider)(nil)
	_ PodNotifier         = (*mockProvider)(nil)
)

type waitableInt struct {
	cond *sync.Cond
	val  int
}

func newWaitableInt() *waitableInt {
	return &waitableInt{
		cond: sync.NewCond(&sync.Mutex{}),
	}
}

func (w *waitableInt) read() int {
	defer w.cond.L.Unlock()
	w.cond.L.Lock()
	return w.val
}

func (w *waitableInt) until(ctx context.Context, f func(int) bool) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		<-ctx.Done()
		w.cond.Broadcast()
	}()

	w.cond.L.Lock()
	defer w.cond.L.Unlock()

	for !f(w.val) {
		if err := ctx.Err(); err != nil {
			return err
		}
		w.cond.Wait()
	}
	return nil
}

func (w *waitableInt) increment() {
	w.cond.L.Lock()
	defer w.cond.L.Unlock()
	w.val += 1
	w.cond.Broadcast()
}

type mockV0Provider struct {
	creates          *waitableInt
	updates          *waitableInt
	deletes          *waitableInt
	attemptedDeletes *waitableInt

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
		startTime:        time.Now(),
		creates:          newWaitableInt(),
		updates:          newWaitableInt(),
		deletes:          newWaitableInt(),
		attemptedDeletes: newWaitableInt(),
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
func (p *mockV0Provider) UpdatePod(ctx context.Context, pod *v1.Pod) error {
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

// DeletePod deletes the specified pod out of memory.
func (p *mockV0Provider) DeletePod(ctx context.Context, pod *v1.Pod) (err error) {
	log.G(ctx).Infof("receive DeletePod %q", pod.Name)

	p.attemptedDeletes.increment()
	if p.errorOnDelete != nil {
		return p.errorOnDelete
	}

	p.deletes.increment()
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
	// TODO (Sargun): Eventually delete the pod from the map. We cannot right now, because GetPodStatus can / will
	// be called momentarily later.
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
		return pod.(*v1.Pod).DeepCopy(), nil
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

	return pod.Status.DeepCopy(), nil
}

// GetPods returns a list of all pods known to be "running".
func (p *mockV0Provider) GetPods(ctx context.Context) ([]*v1.Pod, error) {
	log.G(ctx).Info("receive GetPods")

	var pods []*v1.Pod

	p.pods.Range(func(key, pod interface{}) bool {
		pods = append(pods, pod.(*v1.Pod).DeepCopy())
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
