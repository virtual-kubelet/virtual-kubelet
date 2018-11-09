package vkubelet

import (
	"context"
	"time"

	"github.com/cpuguy83/strongerrors"
	"github.com/cpuguy83/strongerrors/status/ocstatus"
	pkgerrors "github.com/pkg/errors"
	"github.com/virtual-kubelet/virtual-kubelet/log"
	"go.opencensus.io/trace"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

const maxSyncRetries = 5

func addPodAttributes(span *trace.Span, pod *corev1.Pod) {
	span.AddAttributes(
		trace.StringAttribute("uid", string(pod.GetUID())),
		trace.StringAttribute("namespace", pod.GetNamespace()),
		trace.StringAttribute("name", pod.GetName()),
		trace.StringAttribute("phase", string(pod.Status.Phase)),
		trace.StringAttribute("reason", pod.Status.Reason),
	)
}

func (s *Server) syncPod(ctx context.Context, pod *corev1.Pod) error {
	ctx, span := trace.StartSpan(ctx, "syncPod")
	defer span.End()
	addPodAttributes(span, pod)
	logger := log.G(ctx).WithField("pod", pod.GetName()).WithField("namespace", pod.GetNamespace())

	if pod.DeletionTimestamp != nil ||
		s.resourceManager.GetPod(pod.GetNamespace(), pod.GetName()) == nil {
		logger.Debugf("Deleting pod")
		if err := s.deletePod(ctx, pod); err != nil {
			span.SetStatus(ocstatus.FromError(err))
			return err
		}
		span.Annotate(nil, "pod deleted")
	} else {
		if pod.Status.Phase == corev1.PodFailed {
			logger.Debug("skipping failed pod")
			span.Annotate(nil, "skipping failed pod")
			return nil
		}

		logger.Debugf("Creating pod")
		if err := s.createPod(ctx, pod); err != nil {
			span.SetStatus(ocstatus.FromError(err))
			return err
		}
		span.Annotate(nil, "pod created")
	}

	return nil
}

func (s *Server) createPod(ctx context.Context, pod *corev1.Pod) error {
	ctx, span := trace.StartSpan(ctx, "createPod")
	defer span.End()
	addPodAttributes(span, pod)

	if err := s.populateEnvironmentVariables(pod); err != nil {
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

func (s *Server) deletePod(ctx context.Context, pod *corev1.Pod) error {
	ctx, span := trace.StartSpan(ctx, "deletePod")
	defer span.End()
	addPodAttributes(span, pod)

	var delErr error
	if delErr = s.provider.DeletePod(ctx, pod); delErr != nil {
		if strongerrors.IsNotFound(delErr) {
			span.SetStatus(ocstatus.FromError(delErr))
		}
		span.SetStatus(trace.Status{Code: trace.StatusCodeUnknown, Message: delErr.Error()})
		return delErr
	}
	span.Annotate(nil, "Deleted pod from provider")

	logger := log.G(ctx).WithField("pod", pod.GetName()).WithField("namespace", pod.GetNamespace())
	var grace int64
	if err := s.k8sClient.CoreV1().Pods(pod.GetNamespace()).Delete(pod.GetName(), &metav1.DeleteOptions{GracePeriodSeconds: &grace}); err != nil {
		if errors.IsNotFound(err) {
			span.Annotate(nil, "Pod does not exist in k8s, nothing to delete")
			return nil
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

	if !cache.WaitForCacheSync(ctx.Done(), s.podInformer.Informer().HasSynced) {
		return
	}

	// Update all the pods with the provider status.
	ls, err := s.podInformer.Lister().Pods(s.namespace).List(labels.NewSelector())
	if err != nil {
		span.SetStatus(ocstatus.FromError(err))
		return
	}

	span.AddAttributes(trace.Int64Attribute("nPods", int64(len(ls))))

	for _, pod := range ls {
		select {
		case <-ctx.Done():
			span.Annotate(nil, ctx.Err().Error())
			return
		default:
		}

		if err := s.updatePodStatus(ctx, pod); err != nil {
			logger := log.G(ctx).WithField("pod", pod.GetName()).WithField("namespace", pod.GetNamespace()).WithField("status", pod.Status.Phase).WithField("reason", pod.Status.Reason)
			logger.Error(err)
		}
	}
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
				var startedAt metav1.Time
				if c.State.Running != nil {
					startedAt = c.State.Running.StartedAt
				}
				pod.Status.ContainerStatuses[i].State.Terminated = &corev1.ContainerStateTerminated{
					ExitCode:    -137,
					Reason:      "NotFound",
					Message:     "Container was not found and was likely deleted",
					FinishedAt:  metav1.NewTime(time.Now()),
					StartedAt:   startedAt,
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
