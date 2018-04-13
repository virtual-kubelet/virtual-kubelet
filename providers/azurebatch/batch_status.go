package azurebatch

import (
	"encoding/json"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/services/batch/2017-09-01.6.0/batch"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func getPodFromTask(task *batch.CloudTask) (pod *apiv1.Pod, err error) {
	if task == nil || task.EnvironmentSettings == nil {
		return nil, fmt.Errorf("invalid input task: %v", task)
	}

	ok := false
	jsonData := ""
	settings := *task.EnvironmentSettings
	for _, s := range settings {
		if *s.Name == podJsonKey {
			ok = true
			jsonData = *s.Value
		}
	}
	if !ok {
		return nil, fmt.Errorf("task doesn't have pod json stored in it: %v", task.EnvironmentSettings)
	}

	pod = &apiv1.Pod{}
	err = json.Unmarshal([]byte(jsonData), pod)
	return
}

func convertTaskToPodStatus(task *batch.CloudTask) (status *apiv1.PodStatus, err error) {

	pod, err := getPodFromTask(task)
	if err != nil {
		return
	}

	// Todo: Review indivudal container status response
	status = &apiv1.PodStatus{
		Phase:      convertTaskStatusToPodPhase(task),
		Conditions: []apiv1.PodCondition{},
		Message:    "",
		Reason:     "",
		HostIP:     "",
		PodIP:      "127.0.0.1",
		StartTime:  &pod.CreationTimestamp,
	}

	for _, container := range pod.Spec.Containers {
		containerStatus := apiv1.ContainerStatus{
			Name:         container.Name,
			State:        convertTaskStatusToContainerState(task),
			Ready:        true,
			RestartCount: 0,
			Image:        container.Image,
			ImageID:      "",
			ContainerID:  "",
		}
		status.ContainerStatuses = append(status.ContainerStatuses, containerStatus)
	}

	return
}

func convertTaskStatusToPodPhase(t *batch.CloudTask) (podPhase apiv1.PodPhase) {
	switch t.State {
	case batch.TaskStatePreparing:
		podPhase = apiv1.PodPending
	case batch.TaskStateActive:
		podPhase = apiv1.PodPending
	case batch.TaskStateRunning:
		podPhase = apiv1.PodRunning
	case batch.TaskStateCompleted:
		podPhase = apiv1.PodSucceeded
	}

	if t.State == batch.TaskStateCompleted && *t.ExecutionInfo.ExitCode != 0 {
		podPhase = apiv1.PodFailed
	}

	return
}

func convertTaskStatusToContainerState(t *batch.CloudTask) (containerState apiv1.ContainerState) {

	startTime := metav1.Time{}
	if t.ExecutionInfo != nil {
		if t.ExecutionInfo.StartTime != nil {
			startTime.Time = t.ExecutionInfo.StartTime.Time
		}
	}

	switch t.State {
	case batch.TaskStatePreparing:
		containerState = apiv1.ContainerState{
			Waiting: &apiv1.ContainerStateWaiting{
				Message: "Waiting for machine in AzureBatch",
				Reason:  "Preparing",
			},
		}
	case batch.TaskStateActive:
		containerState = apiv1.ContainerState{
			Waiting: &apiv1.ContainerStateWaiting{
				Message: "Waiting for machine in AzureBatch",
				Reason:  "active",
			},
		}
	case batch.TaskStateRunning:
		containerState = apiv1.ContainerState{
			Running: &apiv1.ContainerStateRunning{
				StartedAt: startTime,
			},
		}
	case batch.TaskStateCompleted:
		termStatus := apiv1.ContainerState{
			Terminated: &apiv1.ContainerStateTerminated{
				ExitCode: *t.ExecutionInfo.ExitCode,
				FinishedAt: metav1.Time{
					Time: t.StateTransitionTime.Time,
				},
				StartedAt: startTime,
			},
		}
		if *t.ExecutionInfo.ExitCode != 0 {
			termStatus.Terminated.Message = *t.ExecutionInfo.FailureInfo.Message
		}
	}

	return
}
