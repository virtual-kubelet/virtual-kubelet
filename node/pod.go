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
	"fmt"
	"hash/fnv"
	"reflect"

	"github.com/davecgh/go-spew/spew"
	pkgerrors "github.com/pkg/errors"
	"github.com/virtual-kubelet/virtual-kubelet/errdefs"
	"github.com/virtual-kubelet/virtual-kubelet/log"
	"github.com/virtual-kubelet/virtual-kubelet/trace"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const (
	podStatusReasonProviderFailed = "ProviderFailed"
)

type podAction int

const (
	none podAction = iota
	create
	update
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

type podInfo struct {
	podSpec    *corev1.PodSpec
	objectMeta *metav1.ObjectMeta
}

func (pc *PodController) createOrUpdatePod(ctx context.Context, key string, pod *corev1.Pod) error {
	ctx, span := trace.StartSpan(ctx, "createOrUpdatePod")
	defer span.End()
	addPodAttributes(ctx, span, pod)

	ctx = span.WithFields(ctx, log.Fields{
		"pod":       pod.GetName(),
		"namespace": pod.GetNamespace(),
	})

	// We deepcopy the pod because it was extracted from the informer cache. The provider has free will to modify
	// the pod object we pass it, and therefore we want to operate on a copy
	pod = pod.DeepCopy()
	if err := populateEnvironmentVariables(ctx, pod, pc.resourceManager, pc.recorder); err != nil {
		span.SetStatus(err)
		return err
	}

	lastPodInfoUsedForCreateOrUpdate, ok := pc.lastPodInfoUsedForCreateOrUpdate.Load(key)
	if !ok {
		podCopyForProvider := pod.DeepCopy()
		if origErr := pc.provider.CreatePod(ctx, podCopyForProvider); origErr != nil {
			log.G(ctx).WithError(origErr).Info("Failed to create pod in provider")
			pc.handleProviderError(ctx, span, origErr, podCopyForProvider)
			return origErr
		}
		log.G(ctx).Info("Created pod in provider")
	} else if oldPodInfo := lastPodInfoUsedForCreateOrUpdate.(*podInfo); !reflect.DeepEqual(oldPodInfo.podSpec.Containers, pod.Spec.Containers) ||
		!reflect.DeepEqual(oldPodInfo.podSpec.InitContainers, pod.Spec.InitContainers) ||
		!reflect.DeepEqual(oldPodInfo.podSpec.ActiveDeadlineSeconds, pod.Spec.ActiveDeadlineSeconds) ||
		!reflect.DeepEqual(oldPodInfo.podSpec.Tolerations, pod.Spec.Tolerations) ||
		!reflect.DeepEqual(oldPodInfo.objectMeta.Labels, pod.Labels) ||
		!reflect.DeepEqual(oldPodInfo.objectMeta.Annotations, pod.Annotations) {
		// Pod Update Only Permits update of:
		// - `spec.containers[*].image`
		// - `spec.initContainers[*].image`
		// - `spec.activeDeadlineSeconds`
		// - `spec.tolerations` (only additions to existing tolerations)
		// - `objectmeta.labels`
		// - `objectmeta.annotations`
		// compare these fields
		podCopyForProvider := pod.DeepCopy()
		log.G(ctx).Debugf("Pod %s exists in our pod storage, updating pod in provider", pod.Name)
		if origErr := pc.provider.UpdatePod(ctx, podCopyForProvider); origErr != nil {
			log.G(ctx).WithError(origErr).Error("Failed to update pod in provider")
			pc.handleProviderError(ctx, span, origErr, podCopyForProvider)
			return origErr
		}
		log.G(ctx).Info("Updated pod in provider")
	} else {
		return nil
	}

	pc.lastPodInfoUsedForCreateOrUpdate.Store(key, &podInfo{
		podSpec:    pod.Spec.DeepCopy(),
		objectMeta: pod.ObjectMeta.DeepCopy(),
	})

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
	if delErr = pc.provider.DeletePod(ctx, pod); delErr != nil && !errdefs.IsNotFound(delErr) {
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

// The following functions are responsible for all pod-related work triggered by the provider
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

func (pc *PodController) updatePodStatus(ctx context.Context, pod *corev1.Pod, status *corev1.PodStatus) error {
	if shouldSkipPodStatusUpdate(status) {
		return nil
	}

	// We have to do this because UpdateStatus works on the pod level, even though we have an existing pod that we
	// got from a podinformer. We don't use the provider's pod (and store that) because it would be more costly
	// than waiting until here to do the copy
	pod = pod.DeepCopy()
	pod.Status = *status
	ctx, span := trace.StartSpan(ctx, "updatePodStatus")
	defer span.End()
	ctx = addPodAttributes(ctx, span, pod)

	if _, err := pc.client.Pods(pod.Namespace).UpdateStatus(pod); err != nil {
		span.SetStatus(err)
		return pkgerrors.Wrap(err, "error while updating pod status in kubernetes")
	}

	log.G(ctx).WithFields(log.Fields{
		"new phase":  string(status.Phase),
		"new reason": status.Reason,
	}).Debug("Updated pod status in kubernetes")

	return nil
}

func (pc *PodController) enqueuePodStatusUpdate(ctx context.Context, pod *corev1.Pod) {
	if key, err := cache.MetaNamespaceKeyFunc(pod); err != nil {
		log.G(ctx).WithError(err).WithField("method", "enqueuePodStatusUpdate").Error("Error getting pod meta namespace key")
	} else {
		podStatus := pod.Status.DeepCopy()
		log.G(ctx).WithField("pod", pod).WithField("podStatus", podStatus).Info("Enqueueing status update")
		pc.podStatusMapLock.Lock()
		defer pc.podStatusMapLock.Unlock()
		pc.podStatusMap[key] = podStatus
		pc.podStatusQueue.AddRateLimited(key)
	}
}

func (pc *PodController) podStatusHandler(ctx context.Context, key string, willRetry bool) (retErr error) {
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
		return pkgerrors.Wrap(err, "error spliting cache key")
	}

	pc.podStatusMapLock.Lock()
	podStatus, ok := pc.podStatusMap[key]
	delete(pc.podStatusMap, key)
	pc.podStatusMapLock.Unlock()
	if !ok {
		return fmt.Errorf("Could not find pod %q in pod status map", key)
	}

	pod, err := pc.podsLister.Pods(namespace).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			log.G(ctx).WithError(err).Debug("Skipping pod status update for pod missing in Kubernetes")
			return nil
		}
		return pkgerrors.Wrap(err, "error looking up pod")
	}

	err = pc.updatePodStatus(ctx, pod, podStatus)
	if err != nil && willRetry {
		// Restore the pod status, but do not overwrite it if it's been updated in the mean time while we've been working
		pc.podStatusMapLock.Lock()
		_, ok = pc.podStatusMap[key]
		if !ok {
			pc.podStatusMap[key] = podStatus
		}
		pc.podStatusMapLock.Unlock()
	}

	return err
}

func (pc *PodController) runProviderSyncWorker(ctx context.Context, workerID string) {
	log.G(ctx).WithField("workerId", workerID).Info("provider sync worker starting")
	for pc.processPodStatusUpdateFromProvider(ctx, workerID, pc.podStatusQueue) {
	}

	log.G(ctx).WithField("workerId", workerID).Info("provider sync worker bailing")
}

// processPodStatusUpdateFromProvider processes events from the provider
func (pc *PodController) processPodStatusUpdateFromProvider(ctx context.Context, workerID string, q workqueue.RateLimitingInterface) bool {
	ctx, span := trace.StartSpan(ctx, "processPodStatusUpdate")
	defer span.End()

	// Add the ID of the current worker as an attribute to the current span.
	ctx = span.WithField(ctx, "workerID", workerID)

	return handleQueueItem(ctx, q, pc.podStatusHandler)
}

func (pc *PodController) runSyncFromProvider(ctx context.Context) {
	pc.provider.NotifyPods(ctx, func(pod *corev1.Pod) {
		pc.enqueuePodStatusUpdate(ctx, pod)
	})
}
