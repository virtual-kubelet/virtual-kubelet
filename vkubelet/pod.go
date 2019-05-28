package vkubelet

import (
	"context"
	"hash/fnv"
	"time"

	"github.com/cpuguy83/strongerrors/status/ocstatus"
	"github.com/davecgh/go-spew/spew"
	pkgerrors "github.com/pkg/errors"
	"github.com/virtual-kubelet/virtual-kubelet/log"
	"github.com/virtual-kubelet/virtual-kubelet/trace"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
)

func addPodAttributes(ctx context.Context, span trace.Span, pod *corev1.Pod) context.Context {
	return span.WithFields(ctx, log.Fields{
		"uid":       string(pod.GetUID()),
		"namespace": pod.GetNamespace(),
		"name":      pod.GetName(),
		"phase":     string(pod.Status.Phase),
		"reason":    pod.Status.Reason,
	})
}

func (s *Server) createOrUpdatePod(ctx context.Context, pod *corev1.Pod, recorder record.EventRecorder) error {

	ctx, span := trace.StartSpan(ctx, "createOrUpdatePod")
	defer span.End()
	addPodAttributes(ctx, span, pod)

	ctx = span.WithFields(ctx, log.Fields{
		"pod":       pod.GetName(),
		"namespace": pod.GetNamespace(),
	})

	if err := populateEnvironmentVariables(ctx, pod, s.resourceManager, recorder); err != nil {
		span.SetStatus(ocstatus.FromError(err))
		return err
	}

	// Check if the pod is already known by the provider.
	// NOTE: Some providers return a non-nil error in their GetPod implementation when the pod is not found while some other don't.
	// Hence, we ignore the error and just act upon the pod if it is non-nil (meaning that the provider still knows about the pod).
	if pp, _ := s.provider.GetPod(ctx, pod.Namespace, pod.Name); pp != nil {
		// Pod Update Only Permits update of:
		// - `spec.containers[*].image`
		// - `spec.initContainers[*].image`
		// - `spec.activeDeadlineSeconds`
		// - `spec.tolerations` (only additions to existing tolerations)
		// compare the hashes of the pod specs to see if the specs actually changed
		expected := hashPodSpec(pp.Spec)
		if actual := hashPodSpec(pod.Spec); actual != expected {
			log.G(ctx).Debugf("Pod %s exists, updating pod in provider", pp.Name)
			if origErr := s.provider.UpdatePod(ctx, pod); origErr != nil {
				s.handleProviderError(ctx, span, origErr, pod)
				return origErr
			}
			log.G(ctx).Info("Updated pod in provider")
		}
	} else {
		if origErr := s.provider.CreatePod(ctx, pod); origErr != nil {
			s.handleProviderError(ctx, span, origErr, pod)
			return origErr
		}
		log.G(ctx).Info("Created pod in provider")
	}
	return nil
}

// This is basically the kube runtime's hash container functionality.
// VK only operates at the Pod level so this is adapted for that
func hashPodSpec(spec corev1.PodSpec) uint64 {
	hash := fnv.New32a()
	printer := spew.ConfigState{
		Indent:         " ",
		SortKeys:       true,
		DisableMethods: true,
		SpewKeys:       true,
	}
	printer.Fprintf(hash, "%#v", spec)
	return uint64(hash.Sum32())
}

func (s *Server) handleProviderError(ctx context.Context, span trace.Span, origErr error, pod *corev1.Pod) {
	podPhase := corev1.PodPending
	if pod.Spec.RestartPolicy == corev1.RestartPolicyNever {
		podPhase = corev1.PodFailed
	}

	pod.ResourceVersion = "" // Blank out resource version to prevent object has been modified error
	pod.Status.Phase = podPhase
	pod.Status.Reason = podStatusReasonProviderFailed
	pod.Status.Message = origErr.Error()

	logger := log.G(ctx).WithFields(log.Fields{
		"podPhase": podPhase,
		"reason":   pod.Status.Reason,
	})

	_, err := s.k8sClient.CoreV1().Pods(pod.Namespace).UpdateStatus(pod)
	if err != nil {
		logger.WithError(err).Warn("Failed to update pod status")
	} else {
		logger.Info("Updated k8s pod status")
	}
	span.SetStatus(ocstatus.FromError(origErr))
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
	ctx = addPodAttributes(ctx, span, pod)

	var delErr error
	if delErr = s.provider.DeletePod(ctx, pod); delErr != nil && errors.IsNotFound(delErr) {
		span.SetStatus(ocstatus.FromError(delErr))
		return delErr
	}

	log.G(ctx).Debug("Deleted pod from provider")

	if !errors.IsNotFound(delErr) {
		if err := s.forceDeletePodResource(ctx, namespace, name); err != nil {
			span.SetStatus(ocstatus.FromError(err))
			return err
		}
		log.G(ctx).Info("Deleted pod from Kubernetes")
	}

	return nil
}

func (s *Server) forceDeletePodResource(ctx context.Context, namespace, name string) error {
	ctx, span := trace.StartSpan(ctx, "forceDeletePodResource")
	defer span.End()
	ctx = span.WithFields(ctx, log.Fields{
		"namespace": namespace,
		"name":      name,
	})

	var grace int64
	if err := s.k8sClient.CoreV1().Pods(namespace).Delete(name, &metav1.DeleteOptions{GracePeriodSeconds: &grace}); err != nil {
		if errors.IsNotFound(err) {
			log.G(ctx).Debug("Pod does not exist in Kubernetes, nothing to delete")
			return nil
		}
		span.SetStatus(ocstatus.FromError(err))
		return pkgerrors.Wrap(err, "Failed to delete Kubernetes pod")
	}
	return nil
}

// updatePodStatuses syncs the providers pod status with the kubernetes pod status.
func (s *Server) updatePodStatuses(ctx context.Context, q workqueue.RateLimitingInterface) {
	ctx, span := trace.StartSpan(ctx, "updatePodStatuses")
	defer span.End()

	// Update all the pods with the provider status.
	pods, err := s.podInformer.Lister().List(labels.Everything())
	if err != nil {
		err = pkgerrors.Wrap(err, "error getting pod list")
		span.SetStatus(ocstatus.FromError(err))
		log.G(ctx).WithError(err).Error("Error updating pod statuses")
		return
	}
	ctx = span.WithField(ctx, "nPods", int64(len(pods)))

	for _, pod := range pods {
		if !shouldSkipPodStatusUpdate(pod) {
			s.enqueuePodStatusUpdate(ctx, q, pod)
		}
	}
}

func shouldSkipPodStatusUpdate(pod *corev1.Pod) bool {
	return pod.Status.Phase == corev1.PodSucceeded ||
		pod.Status.Phase == corev1.PodFailed ||
		pod.Status.Reason == podStatusReasonProviderFailed
}

func (s *Server) updatePodStatus(ctx context.Context, pod *corev1.Pod) error {
	if shouldSkipPodStatusUpdate(pod) {
		return nil
	}

	ctx, span := trace.StartSpan(ctx, "updatePodStatus")
	defer span.End()
	ctx = addPodAttributes(ctx, span, pod)

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

	log.G(ctx).WithFields(log.Fields{
		"new phase":  string(pod.Status.Phase),
		"new reason": pod.Status.Reason,
	}).Debug("Updated pod status in kubernetes")

	return nil
}

func (s *Server) enqueuePodStatusUpdate(ctx context.Context, q workqueue.RateLimitingInterface, pod *corev1.Pod) {
	if key, err := cache.MetaNamespaceKeyFunc(pod); err != nil {
		log.G(ctx).WithError(err).WithField("method", "enqueuePodStatusUpdate").Error("Error getting pod meta namespace key")
	} else {
		q.AddRateLimited(key)
	}
}

func (s *Server) podStatusHandler(ctx context.Context, key string) (retErr error) {
	ctx, span := trace.StartSpan(ctx, "podStatusHandler")
	defer span.End()
	defer func() {
		span.SetStatus(ocstatus.FromError(retErr))
	}()

	ctx = span.WithField(ctx, "key", key)

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return pkgerrors.Wrap(err, "error spliting cache key")
	}

	pod, err := s.podInformer.Lister().Pods(namespace).Get(name)
	if err != nil {
		return pkgerrors.Wrap(err, "error looking up pod")
	}

	return s.updatePodStatus(ctx, pod)
}
