// Copyright © 2017 The virtual-kubelet authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package node

import (
	"context"
	"hash/fnv"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/google/go-cmp/cmp"
	pkgerrors "github.com/pkg/errors"
	"github.com/virtual-kubelet/virtual-kubelet/errdefs"
	"github.com/virtual-kubelet/virtual-kubelet/log"
	"github.com/virtual-kubelet/virtual-kubelet/trace"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const (
	podStatusReasonProviderFailed = "ProviderFailed"
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

func (pc *PodController) createOrUpdatePod(ctx context.Context, pod *corev1.Pod) error {

	ctx, span := trace.StartSpan(ctx, "createOrUpdatePod")
	defer span.End()
	addPodAttributes(ctx, span, pod)

	ctx = span.WithFields(ctx, log.Fields{
		"pod":       pod.GetName(),
		"namespace": pod.GetNamespace(),
	})

	// We do this so we don't mutate the pod from the informer cache
	pod = pod.DeepCopy()
	if err := populateEnvironmentVariables(ctx, pod, pc.resourceManager, pc.recorder); err != nil {
		span.SetStatus(err)
		return err
	}

	// We have to use a  different pod that we pass to the provider than the one that gets used in handleProviderError
	// because the provider  may manipulate the pod in a separate goroutine while we were doing work
	podForProvider := pod.DeepCopy()

	// Check if the pod is already known by the provider.
	// NOTE: Some providers return a non-nil error in their GetPod implementation when the pod is not found while some other don't.
	// Hence, we ignore the error and just act upon the pod if it is non-nil (meaning that the provider still knows about the pod).
	if podFromProvider, _ := pc.provider.GetPod(ctx, pod.Namespace, pod.Name); podFromProvider != nil {
		if !podsEqual(podFromProvider, podForProvider) {
			log.G(ctx).Debugf("Pod %s exists, updating pod in provider", podFromProvider.Name)
			if origErr := pc.provider.UpdatePod(ctx, podForProvider); origErr != nil {
				pc.handleProviderError(ctx, span, origErr, pod)
				return origErr
			}
			log.G(ctx).Info("Updated pod in provider")
		}
	} else {
		if origErr := pc.provider.CreatePod(ctx, podForProvider); origErr != nil {
			pc.handleProviderError(ctx, span, origErr, pod)
			return origErr
		}
		log.G(ctx).Info("Created pod in provider")
	}
	return nil
}

// podsEqual checks if two pods are equal according to the fields we know that are allowed
// to be modified after startup time.
func podsEqual(pod1, pod2 *corev1.Pod) bool {
	// Pod Update Only Permits update of:
	// - `spec.containers[*].image`
	// - `spec.initContainers[*].image`
	// - `spec.activeDeadlineSeconds`
	// - `spec.tolerations` (only additions to existing tolerations)
	// - `objectmeta.labels`
	// - `objectmeta.annotations`
	// compare the values of the pods to see if the values actually changed

	return cmp.Equal(pod1.Spec.Containers, pod2.Spec.Containers) &&
		cmp.Equal(pod1.Spec.InitContainers, pod2.Spec.InitContainers) &&
		cmp.Equal(pod1.Spec.ActiveDeadlineSeconds, pod2.Spec.ActiveDeadlineSeconds) &&
		cmp.Equal(pod1.Spec.Tolerations, pod2.Spec.Tolerations) &&
		cmp.Equal(pod1.ObjectMeta.Labels, pod2.Labels) &&
		cmp.Equal(pod1.ObjectMeta.Annotations, pod2.Annotations)

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

func (pc *PodController) handleProviderError(ctx context.Context, span trace.Span, origErr error, pod *corev1.Pod) {
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

	_, err := pc.client.Pods(pod.Namespace).UpdateStatus(pod)
	if err != nil {
		logger.WithError(err).Warn("Failed to update pod status")
	} else {
		logger.Info("Updated k8s pod status")
	}
	span.SetStatus(origErr)
}

func (pc *PodController) deletePod(ctx context.Context, namespace, name string) error {
	ctx, span := trace.StartSpan(ctx, "deletePod")
	defer span.End()

	pod, err := pc.provider.GetPod(ctx, namespace, name)
	if err != nil {
		if errdefs.IsNotFound(err) {
			// The provider is not aware of the pod, but we must still delete the Kubernetes API resource.
			return pc.forceDeletePodResource(ctx, namespace, name)
		}
		return err
	}

	// NOTE: Some providers return a non-nil error in their GetPod implementation when the pod is not found while some other don't.
	if pod == nil {
		// The provider is not aware of the pod, but we must still delete the Kubernetes API resource.
		return pc.forceDeletePodResource(ctx, namespace, name)
	}

	ctx = addPodAttributes(ctx, span, pod)

	var delErr error
	if delErr = pc.provider.DeletePod(ctx, pod.DeepCopy()); delErr != nil && !errdefs.IsNotFound(delErr) {
		span.SetStatus(delErr)
		return delErr
	}

	log.G(ctx).Debug("Deleted pod from provider")

	if err := pc.forceDeletePodResource(ctx, namespace, name); err != nil {
		span.SetStatus(err)
		return err
	}
	log.G(ctx).Info("Deleted pod from Kubernetes")

	return nil
}

func (pc *PodController) forceDeletePodResource(ctx context.Context, namespace, name string) error {
	ctx, span := trace.StartSpan(ctx, "forceDeletePodResource")
	defer span.End()
	ctx = span.WithFields(ctx, log.Fields{
		"namespace": namespace,
		"name":      name,
	})

	var grace int64
	if err := pc.client.Pods(namespace).Delete(name, &metav1.DeleteOptions{GracePeriodSeconds: &grace}); err != nil {
		if errors.IsNotFound(err) {
			log.G(ctx).Debug("Pod does not exist in Kubernetes, nothing to delete")
			return nil
		}
		span.SetStatus(err)
		return pkgerrors.Wrap(err, "Failed to delete Kubernetes pod")
	}
	return nil
}

// fetchPodStatusesFromProvider syncs the providers pod status with the kubernetes pod status.
func (pc *PodController) fetchPodStatusesFromProvider(ctx context.Context, q workqueue.RateLimitingInterface) {
	ctx, span := trace.StartSpan(ctx, "fetchPodStatusesFromProvider")
	defer span.End()

	// Update all the pods with the provider status.
	pods, err := pc.podsLister.List(labels.Everything())
	if err != nil {
		err = pkgerrors.Wrap(err, "error getting pod list from kubernetes")
		span.SetStatus(err)
		log.G(ctx).WithError(err).Error("Error updating pod statuses")
		return
	}
	ctx = span.WithField(ctx, "nPods", int64(len(pods)))

	for _, pod := range pods {
		if !shouldSkipPodStatusUpdate(pod) {
			pc.fetchPodStatusFromProvider(ctx, q, pod)
		}
	}
}
func (pc *PodController) fetchPodStatusFromProvider(ctx context.Context, q workqueue.RateLimitingInterface, podFromKubernetes *corev1.Pod) {
	podStatus, err := pc.provider.GetPodStatus(ctx, podFromKubernetes.Namespace, podFromKubernetes.Name)
	if errdefs.IsNotFound(err) || (err == nil && podStatus == nil) {
		// Only change the status when the pod was already up
		// Only doing so when the pod was successfully running makes sure we don't run into race conditions during pod creation.
		if podFromKubernetes.Status.Phase == corev1.PodRunning || time.Since(podFromKubernetes.ObjectMeta.CreationTimestamp.Time) > time.Minute {
			// Set the pod to failed, this makes sure if the underlying container implementation is gone that a new pod will be created.
			podStatus = podFromKubernetes.Status.DeepCopy()
			podStatus.Phase = corev1.PodFailed
			podStatus.Reason = "NotFound"
			podStatus.Message = "The pod status was not found and may have been deleted from the provider"
			now := metav1.NewTime(time.Now())
			for i, c := range podStatus.ContainerStatuses {
				if c.State.Running == nil {
					continue
				}
				podStatus.ContainerStatuses[i].State.Terminated = &corev1.ContainerStateTerminated{
					ExitCode:    -137,
					Reason:      "NotFound",
					Message:     "Container was not found and was likely deleted",
					FinishedAt:  now,
					StartedAt:   c.State.Running.StartedAt,
					ContainerID: c.ContainerID,
				}
				podStatus.ContainerStatuses[i].State.Running = nil
			}
		}
	} else if err != nil {
		l := log.G(ctx).WithFields(map[string]interface{}{
			"name":      podFromKubernetes.Name,
			"namespace": podFromKubernetes.Namespace,
		})
		l.WithError(err).Error("Could not fetch pod status")
		return
	}

	pod := podFromKubernetes.DeepCopy()
	pod.Status = *podStatus
	pc.enqueuePodStatusUpdate(ctx, q, pod)
}

func shouldSkipPodStatusUpdate(pod *corev1.Pod) bool {
	return pod.Status.Phase == corev1.PodSucceeded ||
		pod.Status.Phase == corev1.PodFailed ||
		pod.Status.Reason == podStatusReasonProviderFailed
}

func (pc *PodController) updatePodStatus(ctx context.Context, podFromKubernetes *corev1.Pod, key string) error {
	if shouldSkipPodStatusUpdate(podFromKubernetes) {
		return nil
	}

	ctx, span := trace.StartSpan(ctx, "updatePodStatus")
	defer span.End()
	ctx = addPodAttributes(ctx, span, podFromKubernetes)

	obj, ok := pc.knownPods.Load(key)
	if !ok {
		// This means there was a race and the pod has been deleted from K8s
		return nil
	}
	kPod := obj.(*knownPod)
	kPod.Lock()
	podFromProvider := kPod.lastPodStatusReceivedFromProvider
	kPod.Unlock()

	if _, err := pc.client.Pods(podFromKubernetes.Namespace).UpdateStatus(podFromProvider); err != nil {
		span.SetStatus(err)
		return pkgerrors.Wrap(err, "error while updating pod status in kubernetes")
	}

	log.G(ctx).WithFields(log.Fields{
		"new phase":  string(podFromProvider.Status.Phase),
		"new reason": podFromProvider.Status.Reason,
		"old phase":  string(podFromKubernetes.Status.Phase),
		"old reason": podFromKubernetes.Status.Reason,
	}).Debug("Updated pod status in kubernetes")

	return nil
}

func (pc *PodController) enqueuePodStatusUpdate(ctx context.Context, q workqueue.RateLimitingInterface, pod *corev1.Pod) {
	if key, err := cache.MetaNamespaceKeyFunc(pod); err != nil {
		log.G(ctx).WithError(err).WithField("method", "enqueuePodStatusUpdate").Error("Error getting pod meta namespace key")
	} else {
		if obj, ok := pc.knownPods.Load(key); ok {
			kpod := obj.(*knownPod)
			kpod.Lock()
			kpod.lastPodStatusReceivedFromProvider = pod.DeepCopy()
			kpod.Unlock()
			q.AddRateLimited(key)
		}
	}
}

func (pc *PodController) podStatusHandler(ctx context.Context, key string) (retErr error) {
	ctx, span := trace.StartSpan(ctx, "podStatusHandler")
	defer span.End()

	ctx = span.WithField(ctx, "key", key)
	log.G(ctx).Debug("processing pod status update")
	defer func() {
		span.SetStatus(retErr)
		if retErr != nil {
			log.G(ctx).WithError(retErr).Error("Error processing pod status update")
		}
	}()

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return pkgerrors.Wrap(err, "error splitting cache key")
	}

	pod, err := pc.podsLister.Pods(namespace).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			log.G(ctx).WithError(err).Debug("Skipping pod status update for pod missing in Kubernetes")
			return nil
		}
		return pkgerrors.Wrap(err, "error looking up pod")
	}

	return pc.updatePodStatus(ctx, pod, key)
}
