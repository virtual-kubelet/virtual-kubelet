package nomad

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	nomad "github.com/hashicorp/nomad/api"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// createNomadTasks takes the containers in a kubernetes pod and creates
// a list of Nomad tasks from them.
func (p *Provider) createNomadTasks(pod *v1.Pod) []*nomad.Task {
	nomadTasks := make([]*nomad.Task, 0, len(pod.Spec.Containers))

	for _, ctr := range pod.Spec.Containers {
		portMap, networkResourcess := createPortMap(ctr.Ports)
		image := ctr.Image
		labels := pod.Labels
		command := ctr.Command
		args := ctr.Args
		resources := createResources(ctr.Resources.Limits, networkResourcess)
		envVars := createEnvVars(ctr.Env)
		task := nomad.Task{
			Name:   ctr.Name,
			Driver: "docker",
			Config: map[string]interface{}{
				"image":    image,
				"port_map": portMap,
				"labels":   labels,
				// TODO: Add volumes support
				"command": strings.Join(command, ""),
				"args":    args,
			},
			Resources: resources,
			Env:       envVars,
		}
		nomadTasks = append(nomadTasks, &task)
	}

	return nomadTasks
}

func createPortMap(ports []v1.ContainerPort) ([]map[string]interface{}, []*nomad.NetworkResource) {
	var portMap []map[string]interface{}
	var dynamicPorts []nomad.Port
	var networkResources []*nomad.NetworkResource

	for i, port := range ports {
		portName := fmt.Sprintf("port_%s", strconv.Itoa(i+1))
		if port.Name != "" {
			portName = port.Name
		}
		portMap = append(portMap, map[string]interface{}{portName: port.ContainerPort})
		dynamicPorts = append(dynamicPorts, nomad.Port{Label: portName})
	}

	return portMap, append(networkResources, &nomad.NetworkResource{DynamicPorts: dynamicPorts})
}

func createResources(limits v1.ResourceList, networkResources []*nomad.NetworkResource) *nomad.Resources {
	taskMemory := int(limits.Memory().Value())
	taskCPU := int(limits.Cpu().Value())

	if taskMemory == 0 {
		taskMemory = 128
	}

	if taskCPU == 0 {
		taskCPU = 100
	}

	return &nomad.Resources{
		Networks: networkResources,
		MemoryMB: &taskMemory,
		CPU:      &taskCPU,
	}
}

func createEnvVars(podEnvVars []v1.EnvVar) map[string]string {
	envVars := map[string]string{}

	for _, v := range podEnvVars {
		envVars[v.Name] = v.Value
	}
	return envVars
}

func (p *Provider) createTaskGroups(name string, tasks []*nomad.Task) []*nomad.TaskGroup {
	count := 1
	restartDelay := 1 * time.Second
	restartMode := "delay"
	restartAttempts := 25

	return []*nomad.TaskGroup{
		&nomad.TaskGroup{
			Name:  &name,
			Count: &count,
			RestartPolicy: &nomad.RestartPolicy{
				Delay:    &restartDelay,
				Mode:     &restartMode,
				Attempts: &restartAttempts,
			},
			Tasks: tasks,
		},
	}
}

func (p *Provider) createJob(name string, datacenters []string, taskGroups []*nomad.TaskGroup) *nomad.Job {
	jobName := fmt.Sprintf("%s-%s", jobNamePrefix, name)

	// Create a new nomad job
	job := nomad.NewServiceJob(jobName, jobName, p.nomadRegion, 100)

	job.Datacenters = datacenters
	job.TaskGroups = taskGroups

	return job
}

func (p *Provider) jobToPod(job *nomad.Job, allocs []*nomad.AllocationListStub) (*v1.Pod, error) {
	containers := []v1.Container{}
	containerStatues := []v1.ContainerStatus{}
	jobStatus := *job.Status
	jobCreatedAt := *job.SubmitTime
	podCondition := convertJobStatusToPodCondition(jobStatus)
	containerStatusesMap := allocToContainerStatuses(allocs)

	// containerPorts are specified for task in a task
	// group
	var containerPorts []v1.ContainerPort
	for _, tg := range job.TaskGroups {
		for _, task := range tg.Tasks {
			for _, taskNetwork := range task.Resources.Networks {
				for _, dynamicPort := range taskNetwork.DynamicPorts {
					// TODO: Dynamic ports aren't being reported via the
					// Nomad `/jobs` endpoint.
					containerPorts = append(containerPorts, v1.ContainerPort{
						Name:     dynamicPort.Label,
						HostPort: int32(dynamicPort.Value),
						HostIP:   taskNetwork.IP,
					})
				}
			}

			containers = append(containers, v1.Container{
				Name:    task.Name,
				Image:   fmt.Sprintf("%s", task.Config["image"]),
				Command: strings.Split(fmt.Sprintf("%s", task.Config["command"]), ""),
				Args:    strings.Split(fmt.Sprintf("%s", task.Config["args"]), " "),
				Ports:   containerPorts,
			})

			containerStatus := containerStatusesMap[task.Name]
			containerStatus.Image = fmt.Sprintf("%s", task.Config["image"])
			containerStatus.ImageID = fmt.Sprintf("%s", task.Config["image"])
			containerStatues = append(containerStatues, containerStatus)
		}
	}

	pod := v1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:              *job.Name,
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(time.Unix(jobCreatedAt, 0)),
		},
		Spec: v1.PodSpec{
			NodeName:   p.nodeName,
			Volumes:    []v1.Volume{},
			Containers: containers,
		},
		Status: v1.PodStatus{
			Phase:             jobStatusToPodPhase(jobStatus),
			Conditions:        []v1.PodCondition{podCondition},
			Message:           "",
			Reason:            "",
			HostIP:            "", // TODO: find out the HostIP
			PodIP:             "", // TODO: find out the equalent for PodIP
			ContainerStatuses: containerStatues,
		},
	}

	return &pod, nil
}

func allocToContainerStatuses(allocs []*nomad.AllocationListStub) map[string]v1.ContainerStatus {
	containerStatusesMap := map[string]v1.ContainerStatus{}

	for _, alloc := range allocs {
		for name, taskState := range alloc.TaskStates {
			containerState, readyFlag := convertTaskStateToContainerState(taskState.State,
				taskState.StartedAt,
				taskState.FinishedAt,
			)
			containerStatusesMap[name] = v1.ContainerStatus{
				Name:         name,
				RestartCount: int32(taskState.Restarts),
				Ready:        readyFlag,
				State:        containerState,
			}
		}
	}

	return containerStatusesMap
}

func jobStatusToPodPhase(status string) v1.PodPhase {
	switch status {
	case "pending":
		return v1.PodPending
	case "running":
		return v1.PodRunning
	// TODO: Make sure we take PodFailed into account.
	case "dead":
		return v1.PodFailed
	}
	return v1.PodUnknown
}

func convertJobStatusToPodCondition(jobStatus string) v1.PodCondition {
	podCondition := v1.PodCondition{}

	switch jobStatus {
	case "pending":
		podCondition = v1.PodCondition{
			Type:   v1.PodInitialized,
			Status: v1.ConditionFalse,
		}
	case "running":
		podCondition = v1.PodCondition{
			Type:   v1.PodReady,
			Status: v1.ConditionTrue,
		}
	case "dead":
		podCondition = v1.PodCondition{
			Type:   v1.PodReasonUnschedulable,
			Status: v1.ConditionFalse,
		}
	default:
		podCondition = v1.PodCondition{
			Type:   v1.PodReasonUnschedulable,
			Status: v1.ConditionUnknown,
		}
	}

	return podCondition
}

func convertTaskStateToContainerState(taskState string, startedAt time.Time, finishedAt time.Time) (v1.ContainerState, bool) {
	containerState := v1.ContainerState{}
	readyFlag := false

	switch taskState {
	case "pending":
		containerState = v1.ContainerState{
			Waiting: &v1.ContainerStateWaiting{},
		}
	case "running":
		containerState = v1.ContainerState{
			Running: &v1.ContainerStateRunning{
				StartedAt: metav1.NewTime(startedAt),
			},
		}
		readyFlag = true
	// TODO: Make sure containers that are exiting with non-zero status codes
	// are accounted for using events or something similar?
	//case v1.PodSucceeded:
	//	podCondition = v1.PodCondition{
	//		Type:   v1.PodReasonUnschedulable,
	//		Status: v1.ConditionFalse,
	//	}
	//	containerState = v1.ContainerState{
	//		Terminated: &v1.ContainerStateTerminated{
	//			ExitCode:   int32(container.State.ExitCode),
	//			FinishedAt: metav1.NewTime(finishedAt),
	//		},
	//	}
	case "dead":
		containerState = v1.ContainerState{
			Terminated: &v1.ContainerStateTerminated{
				ExitCode:   0,
				FinishedAt: metav1.NewTime(finishedAt),
			},
		}
	default:
		containerState = v1.ContainerState{}
	}

	return containerState, readyFlag
}
