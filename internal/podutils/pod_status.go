// Copyright © 2021 The virtual-kubelet authors
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

package podutils

import (
	"context"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
)

// PodConditionsByProvider is the list of pod conditions owned by provider
var PodConditionsByProvider = []corev1.PodConditionType{
	corev1.PodScheduled,
	corev1.PodReady,
	corev1.PodInitialized,
	corev1.PodReasonUnschedulable,
	corev1.ContainersReady,
}

// podConditionByProvider returns if the pod condition type is owned by provider
func podConditionByProvider(conditionType corev1.PodConditionType) bool {
	for _, c := range PodConditionsByProvider {
		if c == conditionType {
			return true
		}
	}
	return false
}

// PatchPodStatus patches pod status.
func PatchPodStatus(c corev1client.PodsGetter,
	namespace, name string, oldPodStatus, newPodStatus corev1.PodStatus) (*corev1.Pod, []byte, error) {
	patchBytes, err := preparePatchBytesForPodStatus(namespace, name, oldPodStatus,
		mergePodStatus(oldPodStatus, newPodStatus))
	if err != nil {
		return nil, nil, err
	}

	updatedPod, err := c.Pods(namespace).Patch(context.TODO(),
		name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{}, "status")
	if err != nil {
		return nil, nil, err
	}
	return updatedPod, patchBytes, nil
}

func preparePatchBytesForPodStatus(namespace, name string, oldPodStatus, newPodStatus corev1.PodStatus) ([]byte, error) {
	oldData, err := json.Marshal(corev1.Pod{
		Status: oldPodStatus,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to Marshal oldData for pod %q/%q: %v", namespace, name, err)
	}

	newData, err := json.Marshal(corev1.Pod{
		Status: newPodStatus,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to Marshal newData for pod %q/%q: %v", namespace, name, err)
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldData, newData, corev1.Pod{})
	if err != nil {
		return nil, fmt.Errorf("failed to CreateTwoWayMergePatch for pod %q/%q: %v", namespace, name, err)
	}
	return patchBytes, nil
}

// mergePodStatus merges oldPodStatus and newPodStatus where pod conditions
// not owned by provider is preserved from oldPodStatus
func mergePodStatus(oldPodStatus, newPodStatus corev1.PodStatus) corev1.PodStatus {
	var podConditions []corev1.PodCondition
	for _, c := range oldPodStatus.Conditions {
		if !podConditionByProvider(c.Type) {
			podConditions = append(podConditions, c)
		}
	}

	for _, c := range newPodStatus.Conditions {
		if podConditionByProvider(c.Type) {
			podConditions = append(podConditions, c)
		}
	}
	newPodStatus.Conditions = podConditions
	return newPodStatus
}
