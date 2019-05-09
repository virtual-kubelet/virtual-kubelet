package vkubelet

import (
	"context"
	"io"
	"sync"
	"time"

	"github.com/cpuguy83/strongerrors/status/ocstatus"
	pkgerrors "github.com/pkg/errors"
	"github.com/virtual-kubelet/virtual-kubelet/log"
	"github.com/virtual-kubelet/virtual-kubelet/providers"
	"github.com/virtual-kubelet/virtual-kubelet/trace"
	"k8s.io/api/core/v1"
)

var (
	_ providers.Provider    = (*providerSync)(nil)
	_ providers.PodNotifier = (*providerSync)(nil)
)

// providerSync takes a legacy provider without notifyPods into one that periodically reconciles
// This wraps the entire rest of the provider interface, and lock around it while we do our updates
// Because the PSM can fetch pod state from the provider directly, things can change underneath
type providerSync struct {
	p        providers.Provider
	lock     sync.RWMutex
	notifier func(*v1.Pod)
}

func (ps *providerSync) CreatePod(ctx context.Context, pod *v1.Pod) error {
	ps.lock.Lock()
	defer ps.lock.Unlock()
	return ps.p.CreatePod(ctx, pod)
}

func (ps *providerSync) UpdatePod(ctx context.Context, pod *v1.Pod) error {
	ps.lock.Lock()
	defer ps.lock.Unlock()
	return ps.p.UpdatePod(ctx, pod)
}

func (ps *providerSync) DeletePod(ctx context.Context, pod *v1.Pod) error {
	ps.lock.Lock()
	defer ps.lock.Unlock()
	return ps.p.DeletePod(ctx, pod)
}

func (ps *providerSync) GetPod(ctx context.Context, namespace, name string) (*v1.Pod, error) {
	ps.lock.RLock()
	defer ps.lock.RUnlock()
	return ps.p.GetPod(ctx, namespace, name)
}

func (ps *providerSync) GetContainerLogs(ctx context.Context, namespace, podName, containerName string, opts providers.ContainerLogOpts) (io.ReadCloser, error) {
	return ps.p.GetContainerLogs(ctx, namespace, podName, containerName, opts)
}

func (ps *providerSync) RunInContainer(ctx context.Context, namespace, podName, containerName string, cmd []string, attach providers.AttachIO) error {
	return ps.p.RunInContainer(ctx, namespace, podName, containerName, cmd, attach)
}

func (ps *providerSync) GetPodStatus(ctx context.Context, namespace, name string) (*v1.PodStatus, error) {
	ps.lock.RLock()
	defer ps.lock.RUnlock()
	return ps.GetPodStatus(ctx, namespace, name)
}

func (ps *providerSync) GetPods(ctx context.Context) ([]*v1.Pod, error) {
	ps.lock.RLock()
	defer ps.lock.RUnlock()
	return ps.p.GetPods(ctx)
}

func (ps *providerSync) Capacity(ctx context.Context) v1.ResourceList {
	return ps.p.Capacity(ctx)
}

func (ps *providerSync) NodeConditions(ctx context.Context) []v1.NodeCondition {
	return ps.p.NodeConditions(ctx)
}

func (ps *providerSync) NodeAddresses(ctx context.Context) []v1.NodeAddress {
	return ps.p.NodeAddresses(ctx)
}

func (ps *providerSync) NodeDaemonEndpoints(ctx context.Context) *v1.NodeDaemonEndpoints {
	return ps.p.NodeDaemonEndpoints(ctx)
}

func (ps *providerSync) OperatingSystem() string {
	return ps.p.OperatingSystem()
}

func (ps *providerSync) NotifyPods(ctx context.Context, notifier func(*v1.Pod)) {
	// This kicks off a goroutine that starts to periodically reconcile the pods in the provider
	go ps.syncLoop(ctx, notifier)
}

func (ps *providerSync) syncLoop(ctx context.Context, notifier func(*v1.Pod)) {
	const sleepTime = 5 * time.Second

	t := time.NewTimer(sleepTime)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			t.Stop()

			ctx, span := trace.StartSpan(ctx, "syncloop")
			ps.doSync(ctx, notifier)
			span.End()

			// restart the timer
			t.Reset(sleepTime)
		}
	}
}

func (ps *providerSync) doSync(ctx context.Context, notifier func(*v1.Pod)) {
	ctx, span := trace.StartSpan(ctx, "doSync")
	defer span.End()

	// Update all the pods with the provider status.
	pods, err := ps.GetPods(ctx)
	if err != nil {
		err = pkgerrors.Wrap(err, "error getting pod list")
		span.SetStatus(ocstatus.FromError(err))
		log.G(ctx).WithError(err).Error("Error updating pod statuses")
		return
	}
	ctx = span.WithField(ctx, "nPods", int64(len(pods)))

	for _, pod := range pods {
		notifier(pod)
	}
}
