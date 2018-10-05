package vkubelet

import (
	"context"
	"fmt"

	"github.com/virtual-kubelet/virtual-kubelet/log"
	"go.opencensus.io/trace"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func addPodAttributes(span *trace.Span, pod *corev1.Pod) {
	span.AddAttributes(
		trace.StringAttribute("uid", string(pod.UID)),
		trace.StringAttribute("namespace", pod.Namespace),
		trace.StringAttribute("name", pod.Name),
	)
}

func (s *Server) createPod(ctx context.Context, pod *corev1.Pod) error {
	ctx, span := trace.StartSpan(ctx, "createPod")
	defer span.End()
	addPodAttributes(span, pod)

	if err := s.populateEnvironmentVariables(pod); err != nil {
		span.SetStatus(trace.Status{Code: trace.StatusCodeInvalidArgument, Message: err.Error()})
		return err
	}

	logger := log.G(ctx).WithField("pod", pod.Name)

	if origErr := s.provider.CreatePod(ctx, pod); origErr != nil {
		podPhase := corev1.PodPending
		if pod.Spec.RestartPolicy == corev1.RestartPolicyNever {
			podPhase = corev1.PodFailed
		}

		pod.ResourceVersion = "" // Blank out resource version to prevent object has been modified error
		pod.Status.Phase = podPhase
		pod.Status.Reason = PodStatusReason_ProviderFailed
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

func (s *Server) deletePod(ctx context.Context, pod *corev1.Pod) error {
	ctx, span := trace.StartSpan(ctx, "deletePod")
	defer span.End()
	addPodAttributes(span, pod)

	var delErr error
	if delErr = s.provider.DeletePod(ctx, pod); delErr != nil && errors.IsNotFound(delErr) {
		span.SetStatus(trace.Status{Code: trace.StatusCodeUnknown, Message: delErr.Error()})
		return delErr
	}
	span.Annotate(nil, "Deleted pod from provider")

	logger := log.G(ctx).WithField("pod", pod.Name)
	if !errors.IsNotFound(delErr) {
		var grace int64
		if err := s.k8sClient.CoreV1().Pods(pod.Namespace).Delete(pod.Name, &metav1.DeleteOptions{GracePeriodSeconds: &grace}); err != nil && errors.IsNotFound(err) {
			if errors.IsNotFound(err) {
				span.Annotate(nil, "Pod does not exist in k8s, nothing to delete")
				return nil
			}

			span.SetStatus(trace.Status{Code: trace.StatusCodeUnknown, Message: err.Error()})
			return fmt.Errorf("Failed to delete kubernetes pod: %s", err)
		}
		span.Annotate(nil, "Deleted pod from k8s")

		s.resourceManager.DeletePod(pod)
		span.Annotate(nil, "Deleted pod from internal state")
		logger.Info("Pod deleted")
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

	for _, pod := range pods {
		if pod.Status.Phase == corev1.PodSucceeded ||
			pod.Status.Phase == corev1.PodFailed ||
			pod.Status.Reason == PodStatusReason_ProviderFailed {
			continue
		}

		status, err := s.provider.GetPodStatus(ctx, pod.Namespace, pod.Name)
		if err != nil {
			log.G(ctx).WithField("pod", pod.Name).WithField("namespace", pod.Namespace).Error("Error retrieving pod status")
			return
		}

		// Update the pod's status
		if status != nil {
			pod.Status = *status
			s.k8sClient.CoreV1().Pods(pod.Namespace).UpdateStatus(pod)
		}
	}
}
