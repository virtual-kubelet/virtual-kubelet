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
	PodLifecycleHandlerV0
	// This channel will be closed when the notifier is set
	notifyPodsSet chan struct{}
	notifier      func(*v1.Pod)
	reconcileTime time.Duration
}

func (wrapper *legacyPodLifecycleHandlerWrapper) NotifyPods(ctx context.Context, notifier func(*v1.Pod)) {
	wrapper.notifier = notifier
	// This should be called only once, but we shouldn't crash if it gets called twice
	close(wrapper.notifyPodsSet)
}

func shouldSkipPodStatusUpdate(status *corev1.PodStatus) bool {
	return status.Phase == corev1.PodSucceeded ||
		status.Phase == corev1.PodFailed ||
		status.Reason == podStatusReasonProviderFailed
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
		if !shouldSkipPodStatusUpdate(&pod.Status) {
			// Notifier is idempotent.
			wrapper.notifier(pod)
		}
	}
}

func (wrapper *legacyPodLifecycleHandlerWrapper) run(ctx context.Context) {
	// Wait for notifyPods to be set
	select {
	case <-ctx.Done():
		return
	case <-wrapper.notifyPodsSet:
	}

	for {
		t := time.NewTimer(wrapper.reconcileTime)
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
				t.Reset(wrapper.reconcileTime)
			}
		}
	}
}

type WrappedPodLifecycleHandler interface {
	PodLifecycleHandler
	run(ctx context.Context)
}

// WrapLegacyPodLifecycleHandler allows you to use a LegacyPodLifecycleHandler. It runs a background loop, based on
// reconcileTime. Every period it will call GetPods.
func WrapLegacyPodLifecycleHandler(ctx context.Context, handler PodLifecycleHandlerV0, reconcileTime time.Duration) WrappedPodLifecycleHandler {
	wrapper := &legacyPodLifecycleHandlerWrapper{
		PodLifecycleHandlerV0: handler,
		notifyPodsSet:         make(chan struct{}),
		reconcileTime:         reconcileTime,
	}
	return wrapper
}
