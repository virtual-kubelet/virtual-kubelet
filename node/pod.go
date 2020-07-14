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
	"math/rand"
	"time"

	"github.com/google/go-cmp/cmp"
	pkgerrors "github.com/pkg/errors"
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
	podEventCreateFailed          = "ProviderCreateFailed"
	podEventCreateSuccess         = "ProviderCreateSuccess"
	podEventDeleteFailed          = "ProviderDeleteFailed"
	podEventDeleteSuccess         = "ProviderDeleteSuccess"
	podEventUpdateFailed          = "ProviderUpdateFailed"
	podEventUpdateSuccess         = "ProviderUpdateSuccess"
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
				pc.recorder.Event(pod, corev1.EventTypeWarning, podEventUpdateFailed, origErr.Error())

				return origErr
			}
			log.G(ctx).Info("Updated pod in provider")
			pc.recorder.Event(pod, corev1.EventTypeNormal, podEventUpdateSuccess, "Update pod in provider successfully")

		}
	} else {
		if origErr := pc.provider.CreatePod(ctx, podForProvider); origErr != nil {
			pc.handleProviderError(ctx, span, origErr, pod)
			pc.recorder.Event(pod, corev1.EventTypeWarning, podEventCreateFailed, origErr.Error())
			return origErr
		}
		log.G(ctx).Info("Created pod in provider")
		pc.recorder.Event(pod, corev1.EventTypeNormal, podEventCreateSuccess, "Create pod in provider successfully")
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

func deleteGraceTimeEqual(old, new *int64) bool {
	if old == nil && new == nil {
		return true
	}
	if old != nil && new != nil {
		return *old == *new
	}
	return false
}

// podShouldEnqueue checks if two pods equal according according to podsEqual func and DeleteTimeStamp
func podShouldEnqueue(oldPod, newPod *corev1.Pod) bool {
	if !podsEqual(oldPod, newPod) {
		return true
	}
	if !deleteGraceTimeEqual(oldPod.DeletionGracePeriodSeconds, newPod.DeletionGracePeriodSeconds) {
		return true
	}
	if !oldPod.DeletionTimestamp.Equal(newPod.DeletionTimestamp) {
		return true
	}
	return false
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

	_, err := pc.client.Pods(pod.Namespace).UpdateStatus(ctx, pod, metav1.UpdateOptions{})
	if err != nil {
		logger.WithError(err).Warn("Failed to update pod status")
	} else {
		logger.Info("Updated k8s pod status")
	}
	span.SetStatus(origErr)
}

func (pc *PodController) deletePod(ctx context.Context, pod *corev1.Pod) error {
	ctx, span := trace.StartSpan(ctx, "deletePod")
	defer span.End()
	ctx = addPodAttributes(ctx, span, pod)

	err := pc.provider.DeletePod(ctx, pod.DeepCopy())
	if err != nil {
		span.SetStatus(err)
		pc.recorder.Event(pod, corev1.EventTypeWarning, podEventDeleteFailed, err.Error())
		return err
	}
	pc.recorder.Event(pod, corev1.EventTypeNormal, podEventDeleteSuccess, "Delete pod in provider successfully")
	log.G(ctx).Debug("Deleted pod from provider")

	return nil
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
	podFromProvider := kPod.lastPodStatusReceivedFromProvider.DeepCopy()
	kPod.Unlock()
	if cmp.Equal(podFromKubernetes.Status, podFromProvider.Status) {
		return nil
	}
	// We need to do this because the other parts of the pod can be updated elsewhere. Since we're only updating
	// the pod status, and we should be the sole writers of the pod status, we can blind overwrite it. Therefore
	// we need to copy the pod and set ResourceVersion to 0.
	podFromProvider.ResourceVersion = "0"
	if _, err := pc.client.Pods(podFromKubernetes.Namespace).UpdateStatus(ctx, podFromProvider, metav1.UpdateOptions{}); err != nil {
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

// enqueuePodStatusUpdate updates our pod status map, and marks the pod as dirty in the workqueue. The pod must be DeepCopy'd
// prior to enqueuePodStatusUpdate.
func (pc *PodController) enqueuePodStatusUpdate(ctx context.Context, q workqueue.RateLimitingInterface, pod *corev1.Pod) {
	// TODO (Sargun): Make this asynchronousish. Right now, if we are not cache synced, and we receive notifications from
	// the provider for pods that do not exist yet in our known pods map, we can get into an awkward situation.
	l := log.G(ctx).WithField("method", "enqueuePodStatusUpdate")
	key, err := cache.MetaNamespaceKeyFunc(pod)
	if err != nil {
		l.WithError(err).Error("Error getting pod meta namespace key")
		return
	}
	// This doesn't wait for all of the callbacks to finish. We should check if the pod exists in K8s. If the pod
	// does not exist in K8s, then we can bail. Alternatively, if the
	var obj interface{}
	var ok bool
retry:
	obj, ok = pc.knownPods.Load(key)
	if !ok {
		// Blind wait for sync. If we haven't synced yet, that's okay too.
		cache.WaitForCacheSync(ctx.Done(), pc.podsInformer.Informer().HasSynced)

		_, err = pc.podsLister.Pods(pod.Namespace).Get(pod.Name)
		if errors.IsNotFound(err) {
			return
		}
		if err != nil {
			l.WithError(err).Error("Received error from pod lister while trying to see if pod exists")
			return
		}
		// err is nil, and therefore the pod exists in k8s, but does not exist in our known pods map. Sleep
		// and retry for somewhere between 1 and 1000 microseconds.
		time.Sleep(time.Microsecond * time.Duration(rand.Intn(1000)))
		goto retry
	}

	kpod := obj.(*knownPod)
	kpod.Lock()
	if cmp.Equal(kpod.lastPodStatusReceivedFromProvider, pod) {
		kpod.Unlock()
		return
	}
	kpod.lastPodStatusReceivedFromProvider = pod
	kpod.Unlock()
	q.AddRateLimited(key)
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

func (pc *PodController) deletePodHandler(ctx context.Context, key string) (retErr error) {
	ctx, span := trace.StartSpan(ctx, "processDeletionReconcilationWorkItem")
	defer span.End()

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	ctx = span.WithFields(ctx, log.Fields{
		"namespace": namespace,
		"name":      name,
	})

	if err != nil {
		// Log the error as a warning, but do not requeue the key as it is invalid.
		log.G(ctx).Warn(pkgerrors.Wrapf(err, "invalid resource key: %q", key))
		span.SetStatus(err)
		return nil
	}

	defer func() {
		if retErr == nil {
			if w, ok := pc.provider.(syncWrapper); ok {
				w._deletePodKey(ctx, key)
			}
		}
	}()

	// If the pod has been deleted from API server, we don't need to do anything.
	k8sPod, err := pc.podsLister.Pods(namespace).Get(name)
	if errors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		span.SetStatus(err)
		return err
	}

	if running(&k8sPod.Status) {
		log.G(ctx).Error("Force deleting pod in running state")
	}

	// We don't check with the provider before doing this delete. At this point, even if an outstanding pod status update
	// was in progress,
	err = pc.client.Pods(namespace).Delete(ctx, name, *metav1.NewDeleteOptions(0))
	if errors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		span.SetStatus(err)
		return err
	}
	return nil
}
