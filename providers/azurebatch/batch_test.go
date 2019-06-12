package azurebatch

import (
	"context"
	"crypto/md5"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/batch/2017-09-01.6.0/batch"
	"github.com/Azure/go-autorest/autorest"
	"github.com/virtual-kubelet/virtual-kubelet/node/api"

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

	err := provider.DeletePod(context.Background(), pod)
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

	err := provider.DeletePod(context.Background(), pod)
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

	err := provider.CreatePod(context.Background(), pod)
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

	err := provider.CreatePod(context.Background(), pod)
	if err == nil {
		t.Error("Expected error but none seen")
	}
}

func Test_readLogs_404Response_expectReturnStartupLogs(t *testing.T) {
	pod := &apiv1.Pod{}
	pod.Namespace = "bob"
	pod.Name = "marley"
	containerName := "sam"

	provider := Provider{}
	provider.getFileFromTask = func(taskID, path string) (batch.ReadCloser, error) {
		if path == "wd/sam.log" {
			// Autorest - Seriously? Can't find a better way to make a 404 :(
			return batch.ReadCloser{Response: autorest.Response{Response: &http.Response{StatusCode: 404}}}, nil
		} else if path == "stderr.txt" {
			response := ioutil.NopCloser(strings.NewReader("stderrResponse"))
			return batch.ReadCloser{Value: &response}, nil
		} else if path == "stdout.txt" {
			response := ioutil.NopCloser(strings.NewReader("stdoutResponse"))
			return batch.ReadCloser{Value: &response}, nil
		} else {
			t.Errorf("Unexpected Filepath: %v", path)
		}

		return batch.ReadCloser{}, fmt.Errorf("Failed in test mock of getFileFromTask")
	}

	logs, err := provider.GetContainerLogs(context.Background(), pod.Namespace, pod.Name, containerName, api.ContainerLogOpts{})
	if err != nil {
		t.Fatalf("GetContainerLogs return error: %v", err)
	}
	defer logs.Close()

	r, err := ioutil.ReadAll(logs)
	if err != nil {
		t.Fatal(err)
	}

	result := string(r)

	if !strings.Contains(result, "stderrResponse") || !strings.Contains(result, "stdoutResponse") {
		t.Errorf("Result didn't contain expected content have: %v", result)
	}

}

func Test_readLogs_JsonResponse_expectFormattedLogs(t *testing.T) {
	pod := &apiv1.Pod{}
	pod.Namespace = "bob"
	pod.Name = "marley"
	containerName := "sam"

	provider := Provider{}
	provider.getFileFromTask = func(taskID, path string) (batch.ReadCloser, error) {
		if path == "wd/sam.log" {
			fileReader, err := os.Open("./testdata/logresponse.json")
			if err != nil {
				t.Error(err)
			}
			readCloser := ioutil.NopCloser(fileReader)
			return batch.ReadCloser{Value: &readCloser, Response: autorest.Response{Response: &http.Response{StatusCode: 200}}}, nil
		}

		t.Errorf("Unexpected Filepath: %v", path)
		return batch.ReadCloser{}, fmt.Errorf("Failed in test mock of getFileFromTask")
	}

	logs, err := provider.GetContainerLogs(context.Background(), pod.Namespace, pod.Name, containerName, api.ContainerLogOpts{})
	if err != nil {
		t.Errorf("GetContainerLogs return error: %v", err)
	}
	defer logs.Close()

	r, err := ioutil.ReadAll(logs)
	if err != nil {
		t.Fatal(err)
	}

	result := string(r)
	if !strings.Contains(string(result), "Copy output data from the CUDA device to the host memory") || strings.Contains(result, "{") {
		t.Errorf("Result didn't contain expected content have or had json: %v", result)
	}

}
