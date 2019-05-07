package vkubelet

import (
	"context"
	"encoding/json"
	"time"

	"github.com/cpuguy83/strongerrors/status/ocstatus"
	pkgerrors "github.com/pkg/errors"
	"github.com/virtual-kubelet/virtual-kubelet/log"
	"github.com/virtual-kubelet/virtual-kubelet/providers"
	"github.com/virtual-kubelet/virtual-kubelet/trace"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
)

const softDeleteKey = "virtual-kubelet.io/softDelete"

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
	addPodAttributes(ctx, span, pod)

	if err := populateEnvironmentVariables(ctx, pod, s.resourceManager, recorder); err != nil {
		span.SetStatus(ocstatus.FromError(err))
		return err
	}

	ctx = span.WithFields(ctx, log.Fields{
		"pod":       pod.GetName(),
		"namespace": pod.GetNamespace(),
	})

	if origErr := s.provider.CreatePod(ctx, pod); origErr != nil {
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
		return origErr
	}

	log.G(ctx).Info("Created pod in provider")

	return nil
}

func (s *Server) deletePodFromProvider(ctx context.Context, namespace, name string) error {
	// Grab the pod as known by the provider.
	// NOTE: Some providers return a non-nil error in their GetPod implementation when the pod is not found while some other don't.
	// Hence, we ignore the error and just act upon the pod if it is non-nil (meaning that the provider still knows about the pod).
	pod, _ := s.provider.GetPod(ctx, namespace, name)
	if pod == nil {
		return nil
	}
	ctx, span := trace.StartSpan(ctx, "deletePodFromProvider")
	defer span.End()
	ctx = addPodAttributes(ctx, span, pod)

	if delErr := s.provider.DeletePod(ctx, pod); delErr != nil && errors.IsNotFound(delErr) {
		span.SetStatus(ocstatus.FromError(delErr))
		return delErr
	}

	log.G(ctx).Debug("Deleted pod from provider")

	return nil
}

// terminatePod terminates the pod in the provider. It assumes the API
// server pod has been marked for deletion
func (s *Server) terminatePod(ctx context.Context, pod *corev1.Pod) error {
	ctx, span := trace.StartSpan(ctx, "terminatePod")
	defer span.End()
	ctx = addPodAttributes(ctx, span, pod)

	ctx = span.WithField(ctx, "softDeletes", s.softDeletes())
	ctx = log.WithLogger(ctx, log.G(ctx).WithField("softDeletes", s.softDeletes()))
	if !s.softDeletes() {
		return s.deletePod(ctx, pod.Namespace, pod.Name)
	}

	// Check if the pod has already been soft-deleted
	if _, ok := pod.ObjectMeta.Annotations[softDeleteKey]; ok {
		return nil
	}

	// If the pod deletion was successful, or the provider did not know about the pod
	// then go ahead and tombstone the pod, otherwise we allow the pod to persist in
	// API server.
	//
	// There is a risk factor here that if the provider returns a non-error for deletion,
	// but deletion failed, the future reconciliation loops will never catch it.

	// We prepare the patch first
	patch := []struct {
		Op    string            `json:"op"`
		Path  string            `json:"path"`
		Value map[string]string `json:"value"`
	}{
		{
			Op:   "add",
			Path: "/metadata/annotations",
			Value: map[string]string{
				softDeleteKey: "true",
			},
		},
	}

	patchData, err := json.Marshal(patch)
	if err != nil {
		span.SetStatus(ocstatus.FromError(err))
		return err
	}

	// Then do the delete
	log.G(ctx).Debug("Prepare for soft-delete -- deleting pod from provider")
	err = s.deletePodFromProvider(ctx, pod.Namespace, pod.Name)
	if err != nil && !errors.IsNotFound(err) {
		log.G(ctx).WithError(err).Warn("Failed to delete pod from provider")
		span.SetStatus(ocstatus.FromError(err))
		return err
	}

	// And then run the patch
	_, err = s.k8sClient.CoreV1().Pods(pod.Namespace).Patch(pod.Name, types.JSONPatchType, patchData)

	// If the pod is deleted from the server, it's okay.
	if errors.IsNotFound(err) {
		return nil
	}
	log.G(ctx).WithError(err).Info("Performed soft delete, and tombstoned pod")
	span.SetStatus(ocstatus.FromError(err))
	return err
}

func (s *Server) deletePod(ctx context.Context, namespace, name string) error {
	ctx, span := trace.StartSpan(ctx, "deletePod")
	defer span.End()

	log.G(ctx).Debug("Performing blind pod deletion from provider")
	if err := s.deletePodFromProvider(ctx, namespace, name); err != nil {
		return err
	}

	log.G(ctx).Debug("Force deleting pod from apiserver")
	if err := s.forceDeletePodResource(ctx, namespace, name); err != nil {
		span.SetStatus(ocstatus.FromError(err))
		return err
	}
	log.G(ctx).Info("Force deleted pod from Kubernetes")

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
func (s *Server) updatePodStatuses(ctx context.Context, q []workqueue.RateLimitingInterface) {
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

func (s *Server) updateRawPodStatus(ctx context.Context, pod *corev1.Pod) error {
	ctx, span := trace.StartSpan(ctx, "updateRawPodStatus")

	// Since our patch only applies to the status subtype, we should be safe in doing this
	// We don't really have a better option, as this method is only called from the async pod notifier,
	// which provides (ordered)
	pod.ObjectMeta.ResourceVersion = ""
	patch, err := json.Marshal(pod)
	if err != nil {
		span.SetStatus(ocstatus.FromError(err))
		return pkgerrors.Wrap(err, "Unable to serialize patch JSON")
	}

	newPod, err := s.k8sClient.CoreV1().Pods(pod.Namespace).Patch(pod.Name, types.MergePatchType, patch, "status")
	if err != nil {
		span.SetStatus(ocstatus.FromError(err))
		return pkgerrors.Wrap(err, "error while patching pod status in kubernetes")
	}

	log.G(ctx).WithFields(log.Fields{
		"old phase":  string(pod.Status.Phase),
		"old reason": pod.Status.Reason,
		"new phase":  string(newPod.Status.Phase),
		"new reason": newPod.Status.Reason,
	}).Debug("Updated pod status in kubernetes")

	return nil
}

func (s *Server) enqueuePodStatusUpdate(ctx context.Context, q []workqueue.RateLimitingInterface, pod *corev1.Pod) {
	if key, err := cache.MetaNamespaceKeyFunc(pod); err != nil {
		log.G(ctx).WithError(err).WithField("method", "enqueuePodStatusUpdate").Error("Error getting pod meta namespace key")
	} else {
		addItem(q, key, s.wrapPodStatusHandler(key))
	}
}

func (s *Server) enqueuePodStatusUpdateWithPod(ctx context.Context, q []workqueue.RateLimitingInterface, pod *corev1.Pod) {
	if key, err := cache.MetaNamespaceKeyFunc(pod); err != nil {
		log.G(ctx).WithError(err).WithField("method", "enqueuePodStatusUpdate").Error("Error getting pod meta namespace key")
	} else {
		addItem(q, key, s.wrapUpdateRawPodStatus(pod))
	}
}

func (s *Server) wrapUpdateRawPodStatus(pod *corev1.Pod) workItem {
	return func(ctx context.Context) error {
		err := s.updateRawPodStatus(ctx, pod)
		if errors.IsNotFound(pkgerrors.Cause(err)) {
			return nil
		}
		return err
	}
}

func (s *Server) wrapPodStatusHandler(key string) workItem {
	return func(ctx context.Context) error {
		return s.podStatusHandler(ctx, key)
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
		err = pkgerrors.Wrap(err, "error spliting cache key")
		span.SetStatus(ocstatus.FromError(retErr))
		return err
	}

	pod, err := s.podInformer.Lister().Pods(namespace).Get(name)
	if err != nil {
		err = pkgerrors.Wrap(err, "error looking up pod")
		span.SetStatus(ocstatus.FromError(retErr))
		return err
	}

	return s.updatePodStatus(ctx, pod)
}

func (s *Server) softDeletes() bool {
	softDeleteProvider, ok := s.provider.(providers.SoftDeletes)
	if !ok {
		return false
	}
	return softDeleteProvider.SoftDeletes()
}
