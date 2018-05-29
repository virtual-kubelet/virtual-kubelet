package azurebatch

import (
	"crypto/md5"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/services/batch/2017-09-01.6.0/batch"
	"github.com/Azure/go-autorest/autorest"
	"testing"

	apiv1 "k8s.io/api/core/v1"
)

func Test_deletePod(t *testing.T) {
	podNamespace := "bob"
	podName := "marley"
	concatName := []byte(fmt.Sprintf("%s-%s", podNamespace, podName))
	expectedDeleteTaskID := fmt.Sprintf("%x", md5.Sum(concatName))

	provider := Provider{}
	provider.deleteTask = func(taskID string) (autorest.Response, error) {
		if taskID != expectedDeleteTaskID {
			t.Errorf("Deleted wrong task! Expected delete: %v Actual: %v", taskID, expectedDeleteTaskID)
		}
		return autorest.Response{}, nil
	}

	pod := &apiv1.Pod{}
	pod.Name = podName
	pod.Namespace = podNamespace

	err := provider.DeletePod(pod)
	if err != nil {
		t.Error(err)
	}
}

func Test_deletePod_doesntExist(t *testing.T) {
	pod := &apiv1.Pod{}
	pod.Namespace = "bob"
	pod.Name = "marley"

	provider := Provider{}
	provider.deleteTask = func(taskID string) (autorest.Response, error) {
		return autorest.Response{}, fmt.Errorf("Task doesn't exist")
	}

	err := provider.DeletePod(pod)
	if err == nil {
		t.Error("Expected error but none seen")
	}
}

func Test_createPod(t *testing.T) {
	pod := &apiv1.Pod{}
	pod.Namespace = "bob"
	pod.Name = "marley"

	provider := Provider{}
	provider.addTask = func(task batch.TaskAddParameter) (autorest.Response, error) {
		if task.CommandLine == nil || *task.CommandLine == "" {
			t.Error("Missing commandline args")
		}

		derefVars := *task.EnvironmentSettings
		if len(derefVars) != 1 || *derefVars[0].Name != podJSONKey {
			t.Error("Missing pod json")
		}
		return autorest.Response{}, nil
	}

	err := provider.CreatePod(pod)
	if err != nil {
		t.Errorf("Unexpected error creating pod %v", err)
	}
}

func Test_createPod_errorResponse(t *testing.T) {
	pod := &apiv1.Pod{}
	pod.Namespace = "bob"
	pod.Name = "marley"

	provider := Provider{}
	provider.addTask = func(task batch.TaskAddParameter) (autorest.Response, error) {
		return autorest.Response{}, fmt.Errorf("Failed creating task")
	}

	err := provider.CreatePod(pod)
	if err == nil {
		t.Error("Expected error but none seen")
	}
}
