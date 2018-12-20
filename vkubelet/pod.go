package vkubelet

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cpuguy83/strongerrors/status/ocstatus"
	pkgerrors "github.com/pkg/errors"
	"go.opencensus.io/trace"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"

	"github.com/virtual-kubelet/virtual-kubelet/log"
)

func addPodAttributes(span *trace.Span, pod *corev1.Pod) {
	span.AddAttributes(
		trace.StringAttribute("uid", string(pod.GetUID())),
		trace.StringAttribute("namespace", pod.GetNamespace()),
		trace.StringAttribute("name", pod.GetName()),
		trace.StringAttribute("phase", string(pod.Status.Phase)),
		trace.StringAttribute("reason", pod.Status.Reason),
	)
}

func (s *Server) createOrUpdatePod(ctx context.Context, pod *corev1.Pod, recorder record.EventRecorder) error {
	// Check if the pod is already known by the provider.
	// NOTE: Some providers return a non-nil error in their GetPod implementation when the pod is not found while some other don't.
	// Hence, we ignore the error and just act upon the pod if it is non-nil (meaning that the provider still knows about the pod).
	if pp, _ := s.provider.GetPod(ctx, pod.Namespace, pod.Name); pp != nil {
		// The pod has already been created in the provider.
		// Hence, we return since pod updates are not yet supported.
		log.G(ctx).Warnf("skipping update of pod %s as pod updates are not supported", pp.Name)
		return nil
	}

	ctx, span := trace.StartSpan(ctx, "createOrUpdatePod")
	defer span.End()
	addPodAttributes(span, pod)

	if err := populateEnvironmentVariables(ctx, pod, s.resourceManager, recorder); err != nil {
		span.SetStatus(trace.Status{Code: trace.StatusCodeInvalidArgument, Message: err.Error()})
		return err
	}

	logger := log.G(ctx).WithField("pod", pod.GetName()).WithField("namespace", pod.GetNamespace())

	if origErr := s.provider.CreatePod(ctx, pod); origErr != nil {
		podPhase := corev1.PodPending
		if pod.Spec.RestartPolicy == corev1.RestartPolicyNever {
			podPhase = corev1.PodFailed
		}

		pod.ResourceVersion = "" // Blank out resource version to prevent object has been modified error
		pod.Status.Phase = podPhase
		pod.Status.Reason = podStatusReasonProviderFailed
		pod.Status.Message = origErr.Error()

		_, err := s.k8sClient.CoreV1().Pods(pod.Namespace).UpdateStatus(pod)
		if err != nil {
			logger.WithError(err).Warn("Failed to update pod status")
		} else {
			span.Annotate(nil, "Updated k8s pod status")
		}

		span.SetStatus(trace.Status{Code: trace.StatusCodeUnknown, Message: origErr.Error()})
		return origErr
	}
	span.Annotate(nil, "Created pod in provider")

	logger.Info("Pod created")

	return nil
}

func (s *Server) deletePod(ctx context.Context, namespace, name string) error {
	// Grab the pod as known by the provider.
	// NOTE: Some providers return a non-nil error in their GetPod implementation when the pod is not found while some other don't.
	// Hence, we ignore the error and just act upon the pod if it is non-nil (meaning that the provider still knows about the pod).
	pod, _ := s.provider.GetPod(ctx, namespace, name)
	if pod == nil {
		// The provider is not aware of the pod, but we must still delete the Kubernetes API resource.
		return s.forceDeletePodResource(ctx, namespace, name)
	}

	ctx, span := trace.StartSpan(ctx, "deletePod")
	defer span.End()
	addPodAttributes(span, pod)

	var delErr error
	if delErr = s.provider.DeletePod(ctx, pod); delErr != nil && errors.IsNotFound(delErr) {
		span.SetStatus(trace.Status{Code: trace.StatusCodeUnknown, Message: delErr.Error()})
		return delErr
	}
	span.Annotate(nil, "Deleted pod from provider")

	logger := log.G(ctx).WithField("pod", pod.GetName()).WithField("namespace", pod.GetNamespace())
	if !errors.IsNotFound(delErr) {
		if err := s.forceDeletePodResource(ctx, namespace, name); err != nil {
			span.SetStatus(ocstatus.FromError(err))
			return err
		}
		span.Annotate(nil, "Deleted pod from k8s")
		logger.Info("Pod deleted")
	}

	return nil
}

func (s *Server) forceDeletePodResource(ctx context.Context, namespace, name string) error {
	ctx, span := trace.StartSpan(ctx, "forceDeletePodResource")
	defer span.End()
	span.AddAttributes(
		trace.StringAttribute("namespace", namespace),
		trace.StringAttribute("name", name),
	)

	var grace int64
	if err := s.k8sClient.CoreV1().Pods(namespace).Delete(name, &metav1.DeleteOptions{GracePeriodSeconds: &grace}); err != nil {
		if errors.IsNotFound(err) {
			span.Annotate(nil, "Pod does not exist in Kubernetes, nothing to delete")
			return nil
		}
		span.SetStatus(trace.Status{Code: trace.StatusCodeUnknown, Message: err.Error()})
		return fmt.Errorf("Failed to delete Kubernetes pod: %s", err)
	}
	return nil
}

// updatePodStatuses syncs the providers pod status with the kubernetes pod status.
func (s *Server) updatePodStatuses(ctx context.Context) {
	ctx, span := trace.StartSpan(ctx, "updatePodStatuses")
	defer span.End()

	// Update all the pods with the provider status.
	pods := s.resourceManager.GetPods()
	span.AddAttributes(trace.Int64Attribute("nPods", int64(len(pods))))

	sema := make(chan struct{}, s.podSyncWorkers)
	var wg sync.WaitGroup
	wg.Add(len(pods))

	for _, pod := range pods {
		go func(pod *corev1.Pod) {
			defer wg.Done()

			select {
			case <-ctx.Done():
				span.SetStatus(ocstatus.FromError(ctx.Err()))
				return
			case sema <- struct{}{}:
			}
			defer func() { <-sema }()

			if err := s.updatePodStatus(ctx, pod); err != nil {
				logger := log.G(ctx).WithField("pod", pod.GetName()).WithField("namespace", pod.GetNamespace()).WithField("status", pod.Status.Phase).WithField("reason", pod.Status.Reason)
				logger.Error(err)
			}

		}(pod)
	}

	wg.Wait()
}

func (s *Server) updatePodStatus(ctx context.Context, pod *corev1.Pod) error {
	ctx, span := trace.StartSpan(ctx, "updatePodStatus")
	defer span.End()
	addPodAttributes(span, pod)

	if pod.Status.Phase == corev1.PodSucceeded ||
		pod.Status.Phase == corev1.PodFailed ||
		pod.Status.Reason == podStatusReasonProviderFailed {
		return nil
	}

	status, err := s.provider.GetPodStatus(ctx, pod.Namespace, pod.Name)
	if err != nil {
		span.SetStatus(ocstatus.FromError(err))
		return pkgerrors.Wrap(err, "error retreiving pod status")
	}

	// Update the pod's status
	if status != nil {
		pod.Status = *status
	} else {
		// Only change the status when the pod was already up
		// Only doing so when the pod was successfully running makes sure we don't run into race conditions during pod creation.
		if pod.Status.Phase == corev1.PodRunning || pod.ObjectMeta.CreationTimestamp.Add(time.Minute).Before(time.Now()) {
			// Set the pod to failed, this makes sure if the underlying container implementation is gone that a new pod will be created.
			pod.Status.Phase = corev1.PodFailed
			pod.Status.Reason = "NotFound"
			pod.Status.Message = "The pod status was not found and may have been deleted from the provider"
			for i, c := range pod.Status.ContainerStatuses {
				pod.Status.ContainerStatuses[i].State.Terminated = &corev1.ContainerStateTerminated{
					ExitCode:    -137,
					Reason:      "NotFound",
					Message:     "Container was not found and was likely deleted",
					FinishedAt:  metav1.NewTime(time.Now()),
					StartedAt:   c.State.Running.StartedAt,
					ContainerID: c.ContainerID,
				}
				pod.Status.ContainerStatuses[i].State.Running = nil
			}
		}
	}

	if _, err := s.k8sClient.CoreV1().Pods(pod.Namespace).UpdateStatus(pod); err != nil {
		span.SetStatus(ocstatus.FromError(err))
		return pkgerrors.Wrap(err, "error while updating pod status in kubernetes")
	}

	span.Annotate([]trace.Attribute{
		trace.StringAttribute("new phase", string(pod.Status.Phase)),
		trace.StringAttribute("new reason", pod.Status.Reason),
	}, "updated pod status in kubernetes")
	return nil
}
