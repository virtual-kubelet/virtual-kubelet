package azurebatch

import (
	"reflect"
	"testing"

	"github.com/Azure/go-autorest/autorest/to"

	"github.com/Azure/azure-sdk-for-go/services/batch/2017-09-01.6.0/batch"
	apiv1 "k8s.io/api/core/v1"
)

func Test_getPodFromTask(t *testing.T) {
	type args struct {
		task *batch.CloudTask
	}
	tests := []struct {
		name    string
		task    batch.CloudTask
		wantPod *apiv1.Pod
		wantErr bool
	}{
		{
			name: "SimplePod",
			task: batch.CloudTask{
				EnvironmentSettings: &[]batch.EnvironmentSetting{
					{
						Name:  to.StringPtr(podJSONKey),
						Value: to.StringPtr(`{"metadata":{"creationTimestamp":null},"spec":{"containers":[{"name":"web","image":"nginx:1.12","ports":[{"name":"http","containerPort":80,"protocol":"TCP"}],"resources":{}}]},"status":{}}`),
					},
				},
			},
			wantPod: &apiv1.Pod{
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{
							Name:  "web",
							Image: "nginx:1.12",
							Ports: []apiv1.ContainerPort{
								{
									Name:          "http",
									Protocol:      apiv1.ProtocolTCP,
									ContainerPort: 80,
								},
							},
						},
					},
				},
			},
		},
		{
			name: "InvalidJson",
			task: batch.CloudTask{
				EnvironmentSettings: &[]batch.EnvironmentSetting{
					{
						Name:  to.StringPtr(podJSONKey),
						Value: to.StringPtr("---notjson--"),
					},
				},
			},
			wantErr: true,
		},
		{
			name: "NilEnvironment",
			task: batch.CloudTask{
				EnvironmentSettings: nil,
			},
			wantErr: true,
		},
		{
			name: "NilString",
			task: batch.CloudTask{
				EnvironmentSettings: &[]batch.EnvironmentSetting{
					{
						Name:  to.StringPtr(podJSONKey),
						Value: nil,
					},
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPod, err := getPodFromTask(&tt.task)
			if (err != nil) != tt.wantErr {
				t.Errorf("getPodFromTask() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !reflect.DeepEqual(gotPod, tt.wantPod) {

				t.Errorf("getPodFromTask() = %v, want %v", gotPod, tt.wantPod)
			}
		})
	}
}

func Test_convertTaskStatusToPodPhase(t *testing.T) {
	type args struct {
		t *batch.CloudTask
	}
	tests := []struct {
		name         string
		task         batch.CloudTask
		wantPodPhase apiv1.PodPhase
	}{
		{
			name: "PreparingTask",
			task: batch.CloudTask{
				State: batch.TaskStatePreparing,
			},
			wantPodPhase: apiv1.PodPending,
		},
		{
			//Active tasks are sitting in a queue waiting for a node
			// so maps best to pending state
			name: "ActiveTask",
			task: batch.CloudTask{
				State: batch.TaskStateActive,
			},
			wantPodPhase: apiv1.PodPending,
		},
		{
			name: "RunningTask",
			task: batch.CloudTask{
				State: batch.TaskStateRunning,
			},
			wantPodPhase: apiv1.PodRunning,
		},
		{
			name: "CompletedTask_ExitCode0",
			task: batch.CloudTask{
				State: batch.TaskStateCompleted,
				ExecutionInfo: &batch.TaskExecutionInformation{
					ExitCode: to.Int32Ptr(0),
				},
			},
			wantPodPhase: apiv1.PodSucceeded,
		},
		{
			name: "CompletedTask_ExitCode127",
			task: batch.CloudTask{
				State: batch.TaskStateCompleted,
				ExecutionInfo: &batch.TaskExecutionInformation{
					ExitCode: to.Int32Ptr(127),
				},
			},
			wantPodPhase: apiv1.PodFailed,
		},
		{
			name: "CompletedTask_nilExecInfo",
			task: batch.CloudTask{
				State:         batch.TaskStateCompleted,
				ExecutionInfo: nil,
			},
			wantPodPhase: apiv1.PodFailed,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotPodPhase := convertTaskStatusToPodPhase(&tt.task); !reflect.DeepEqual(gotPodPhase, tt.wantPodPhase) {
				t.Errorf("convertTaskStatusToPodPhase() = %v, want %v", gotPodPhase, tt.wantPodPhase)
			}
		})
	}
}
