package node

import (
	"context"
	"time"

	pkgerrors "github.com/pkg/errors"
	"github.com/virtual-kubelet/virtual-kubelet/log"
	"github.com/virtual-kubelet/virtual-kubelet/trace"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
)

type legacyPodLifecycleHandlerWrapper struct {
	LegacyPodLifecycleHandler
	// This channel will be closed when the notifier is set
	notifyPodsSet chan struct{}
	notifier      func(*v1.Pod)
}

func (wrapper *legacyPodLifecycleHandlerWrapper) NotifyPods(ctx context.Context, notifier func(*v1.Pod)) {
	wrapper.notifier = notifier
	// This should be called only once, but we shouldn't crash if it gets called twice
	close(wrapper.notifyPodsSet)
}

func shouldSkipPodStatusUpdate(pod *corev1.Pod) bool {
	return pod.Status.Phase == corev1.PodSucceeded ||
		pod.Status.Phase == corev1.PodFailed ||
		pod.Status.Reason == podStatusReasonProviderFailed
}

// updatePodStatuses syncs the providers pod status with the kubernetes pod status.
func (wrapper *legacyPodLifecycleHandlerWrapper) updatePodStatuses(ctx context.Context) {
	ctx, span := trace.StartSpan(ctx, "updatePodStatuses")
	defer span.End()

	pods, err := wrapper.GetPods(ctx)
	if err != nil {
		err = pkgerrors.Wrap(err, "error getting pod list")
		span.SetStatus(err)
		log.G(ctx).WithError(err).Error("Error updating pod statuses")
		return
	}

	for _, pod := range pods {
		if !shouldSkipPodStatusUpdate(pod) {
			// Notifier is idempotent.
			wrapper.notifier(pod)
		}
	}
}

func (wrapper *legacyPodLifecycleHandlerWrapper) run(ctx context.Context, sleepDuration time.Duration) {
	// Wait for notifyPods to be set
	select {
	case <-ctx.Done():
		return
	case <-wrapper.notifyPodsSet:
	}

	for {
		t := time.NewTimer(sleepDuration)
		defer t.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				t.Stop()

				ctx, span := trace.StartSpan(ctx, "syncActualState")
				wrapper.updatePodStatuses(ctx)
				span.End()

				// restart the timer
				t.Reset(sleepDuration)
			}
		}
	}
}

// WrapLegacyPodLifecycleHandler allows you to use a LegacyPodLifecycleHandler. It runs a background loop, based on
// reconcileTime. Every period it will call GetPods.
func WrapLegacyPodLifecycleHandler(ctx context.Context, handler LegacyPodLifecycleHandler, reconcileTime time.Duration) PodLifecycleHandler {
	wrapper := &legacyPodLifecycleHandlerWrapper{
		LegacyPodLifecycleHandler: handler,
		notifyPodsSet:             make(chan struct{}),
	}
	go wrapper.run(ctx, reconcileTime)
	return wrapper
}
