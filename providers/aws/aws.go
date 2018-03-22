package aws

import (
	"log"
	"os"
	"strings"

	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/virtual-kubelet/virtual-kubelet/manager"
	"github.com/virtual-kubelet/virtual-kubelet/providers"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	defaultCPUUnits   = "256"
	defaultMemoryMiB  = "512"
	cpuAnnotation     = "ecs-tasks.amazonaws.com/cpu"
	memoryAnnotation  = "ecs-tasks.amazonaws.com/memory"
	iamRoleAnnotation = "iam.amazonaws.com/role"
)

// Provider implements the virtual-kubelet provider interface and stores pods in memory.
type Provider struct {
	resourceManager    *manager.ResourceManager
	region             *string
	cluster            *string
	nodeName           string
	operatingSystem    string
	internalIP         string
	daemonEndpointPort int32
	ecsClient          *ecs.ECS
	cloudwatchClient   *cloudwatchlogs.CloudWatchLogs
	subnets            []*string
	securityGroups     []*string
	executionRoleArn   *string
	cloudWatchLogGroup *string

	cpu    string
	memory string
	pods   string
}

// NewProvider creates a new Provider
func NewProvider(config string, rm *manager.ResourceManager, nodeName, operatingSystem string, internalIP string, daemonEndpointPort int32) (*Provider, error) {
	p := Provider{
		nodeName:           nodeName,
		operatingSystem:    operatingSystem,
		internalIP:         internalIP,
		daemonEndpointPort: daemonEndpointPort,
	}

	p.resourceManager = rm

	if config != "" {
		f, err := os.Open(config)
		if err != nil {
			return nil, err
		}
		defer f.Close()

		if err := p.loadConfig(f); err != nil {
			return nil, err
		}
	}

	p.ecsClient = ecs.New(session.New(&aws.Config{
		Region: p.region,
	}))

	p.cloudwatchClient = cloudwatchlogs.New(session.New(&aws.Config{
		Region: p.region,
	}))

	return &p, nil
}

// CreatePod accepts a Pod definition and stores it in memory.
func (p *Provider) CreatePod(pod *v1.Pod) error {
	log.Printf("receive CreatePod %q\n", pod.Name)

	task := &ecs.RegisterTaskDefinitionInput{
		Family: aws.String(toFamily(pod.Namespace, pod.Name)),

		Cpu:    aws.String(defaultCPUUnits),
		Memory: aws.String(defaultMemoryMiB),

		NetworkMode:          aws.String(ecs.NetworkModeAwsvpc),
		ContainerDefinitions: []*ecs.ContainerDefinition{},

		RequiresCompatibilities: []*string{
			aws.String(ecs.CompatibilityFargate),
		},
		ExecutionRoleArn: p.executionRoleArn,
	}

	if val, ok := pod.Annotations[cpuAnnotation]; ok {
		task.Cpu = aws.String(val)
	}

	if val, ok := pod.Annotations[memoryAnnotation]; ok {
		task.Memory = aws.String(val)
	}

	if val, ok := pod.Annotations[iamRoleAnnotation]; ok {
		task.TaskRoleArn = aws.String(val)
	}

	for _, ctr := range pod.Spec.Containers {
		var workingDir *string
		if ctr.WorkingDir != "" {
			workingDir = &ctr.WorkingDir
		}

		c := ecs.ContainerDefinition{
			Name:             aws.String(ctr.Name),
			Image:            aws.String(ctr.Image),
			EntryPoint:       toAWSStrings(ctr.Command),
			Command:          toAWSStrings(ctr.Args),
			WorkingDirectory: workingDir,
			LogConfiguration: &ecs.LogConfiguration{
				LogDriver: aws.String(ecs.LogDriverAwslogs),
				Options: map[string]*string{
					"awslogs-group":         p.cloudWatchLogGroup,
					"awslogs-region":        p.region,
					"awslogs-stream-prefix": aws.String(fmt.Sprintf("%s_%s", toFamily(pod.Namespace, pod.Name), ctr.Name)),
				},
			},
			PortMappings:      make([]*ecs.PortMapping, 0, len(ctr.Ports)),
			Cpu:               aws.Int64(ctr.Resources.Limits.Cpu().Value()),
			Memory:            aws.Int64(ctr.Resources.Limits.Memory().Value() / 1024 / 1024),
			MemoryReservation: aws.Int64(ctr.Resources.Requests.Memory().Value() / 1024 / 1024),
		}

		// TODO Support ctr.EnvFrom

		c.Environment = make([]*ecs.KeyValuePair, 0, len(ctr.Env))
		for _, e := range ctr.Env {
			c.Environment = append(c.Environment, &ecs.KeyValuePair{
				Name:  aws.String(e.Name),
				Value: aws.String(e.Value),
			})
		}

		for _, p := range ctr.Ports {
			c.PortMappings = append(c.PortMappings, &ecs.PortMapping{
				HostPort:      aws.Int64(int64(p.HostPort)),
				ContainerPort: aws.Int64(int64(p.ContainerPort)),
				Protocol:      aws.String(toECSProtocol(p.Protocol)),
			})
		}

		task.ContainerDefinitions = append(task.ContainerDefinitions, &c)
	}

	result, err := p.ecsClient.RegisterTaskDefinition(task)
	if err != nil {
		log.Printf("Error registering task definition: %s", err)

		return err
	}

	runTaskInput := &ecs.RunTaskInput{
		Cluster:        p.cluster,
		LaunchType:     aws.String(ecs.LaunchTypeFargate),
		TaskDefinition: result.TaskDefinition.TaskDefinitionArn,
		NetworkConfiguration: &ecs.NetworkConfiguration{
			AwsvpcConfiguration: &ecs.AwsVpcConfiguration{
				AssignPublicIp: aws.String(ecs.AssignPublicIpEnabled),
				SecurityGroups: p.securityGroups,
				Subnets:        p.subnets,
			},
		},
		StartedBy: aws.String(string(pod.UID)),
	}

	_, err = p.ecsClient.RunTask(runTaskInput)
	if err != nil {
		log.Printf("Error running task: %s", err)

		return err
	}

	return nil
}

// UpdatePod accepts a Pod definition and updates its reference.
func (p *Provider) UpdatePod(pod *v1.Pod) error {
	return nil
}

// DeletePod deletes the specified pod out of memory.
func (p *Provider) DeletePod(pod *v1.Pod) (err error) {
	log.Printf("receive DeletePod %q\n", pod.Name)

	task, err := p.findTaskByNamespaceAndName(pod.Namespace, pod.Name)
	if err != nil {
		log.Printf("Failed finding task: %s", err)

		return err
	}

	if task == nil {
		return nil
	}

	_, stopErr := p.ecsClient.StopTask(&ecs.StopTaskInput{
		Cluster: p.cluster,
		Task:    task.TaskArn,
	})

	if stopErr != nil {
		log.Printf("Failed stopping task: %s", err)
	}

	_, err = p.ecsClient.DeregisterTaskDefinition(&ecs.DeregisterTaskDefinitionInput{
		TaskDefinition: task.TaskDefinitionArn,
	})
	if err != nil {
		log.Printf("Error deregistering task definition: %s", err)

		return err
	}

	if stopErr != nil {
		return stopErr
	}

	return nil
}

// GetPod returns a pod by name that is stored in memory.
func (p *Provider) GetPod(namespace, name string) (pod *v1.Pod, err error) {
	key := fmt.Sprintf("%s/%s", namespace, name)

	log.Printf("receive GetPod %s\n", key)

	task, err := p.findTaskByNamespaceAndName(namespace, name)
	if err != nil {
		log.Printf("Failed finding task: %s", err)

		return nil, err
	}

	if task == nil {
		return nil, nil
	}

	return p.taskToPod(task)
}

func (p *Provider) findTaskByNamespaceAndName(namespace, name string) (*ecs.Task, error) {
	listResult, err := p.ecsClient.ListTasks(&ecs.ListTasksInput{
		Cluster: p.cluster,
		Family:  aws.String(toFamily(namespace, name)),
	})

	if err != nil {
		log.Printf("Error: Failed listing tasks: %s", err)
		return nil, err
	}

	if len(listResult.TaskArns) == 0 {
		return nil, nil
	}

	if len(listResult.TaskArns) != 1 {
		log.Printf("Error: Invalid number of tasks returned: %d", len(listResult.TaskArns))
		return nil, nil
	}

	describeResult, err := p.ecsClient.DescribeTasks(&ecs.DescribeTasksInput{
		Cluster: p.cluster,
		Tasks:   listResult.TaskArns,
	})
	if err != nil {
		log.Printf("Error: Failed describing tasks: %s", err)
		return nil, err
	}

	if len(describeResult.Tasks) == 0 {
		return nil, nil
	}

	return describeResult.Tasks[0], nil
}

// GetContainerLogs retrieves the logs of a container by name from the provider.
func (p *Provider) GetContainerLogs(namespace, podName, containerName string, tail int) (string, error) {
	prefix := fmt.Sprintf("%s_%s", toFamily(namespace, podName), containerName)
	describeResult, err := p.cloudwatchClient.DescribeLogStreams(&cloudwatchlogs.DescribeLogStreamsInput{
		LogGroupName:        p.cloudWatchLogGroup,
		LogStreamNamePrefix: aws.String(prefix),
	})
	if err != nil {
		return "", err
	}

	// Nothing logged yet
	if len(describeResult.LogStreams) == 0 {
		return "", nil
	}

	logs := ""

	err = p.cloudwatchClient.GetLogEventsPages(&cloudwatchlogs.GetLogEventsInput{
		Limit:         aws.Int64(int64(tail)),
		LogGroupName:  p.cloudWatchLogGroup,
		LogStreamName: describeResult.LogStreams[0].LogStreamName,
	}, func(page *cloudwatchlogs.GetLogEventsOutput, lastPage bool) bool {
		for _, event := range page.Events {
			logs += *event.Message
			logs += "\n"
		}

		// Due to a issue in the aws-sdk last page is never true, but the we can stop
		// as soon as no further results are returned
		// See https://github.com/aws/aws-sdk-ruby/pull/730
		return len(page.Events) > 0
	})

	if err != nil {
		return "", err
	}

	return logs, nil
}

// GetPodStatus returns the status of a pod by name that is "running".
// returns nil if a pod by that name is not found.
func (p *Provider) GetPodStatus(namespace, name string) (*v1.PodStatus, error) {
	log.Printf("receive GetPodStatus %s/%s\n", namespace, name)

	pod, err := p.GetPod(namespace, name)
	if err != nil {
		return nil, err
	}

	if pod == nil {
		return nil, nil
	}

	return &pod.Status, nil
}

// GetPods returns a list of all pods known to be "running".
func (p *Provider) GetPods() ([]*v1.Pod, error) {
	log.Printf("receive GetPods\n")

	taskArns := make([]*string, 0)

	err := p.ecsClient.ListTasksPages(&ecs.ListTasksInput{
		Cluster:    p.cluster,
		LaunchType: aws.String(ecs.LaunchTypeFargate),
	}, func(page *ecs.ListTasksOutput, lastPage bool) bool {
		taskArns = append(taskArns, page.TaskArns...)

		return !lastPage
	})

	if err != nil {
		log.Printf("Error listing tasks: %s", err)
		return nil, err
	}

	if len(taskArns) == 0 {
		return []*v1.Pod{}, nil
	}

	describeResult, err := p.ecsClient.DescribeTasks(&ecs.DescribeTasksInput{
		Cluster: p.cluster,
		Tasks:   taskArns,
	})

	if err != nil {
		log.Printf("Error describing tasks: %s", err)
		return nil, err
	}

	for _, failure := range describeResult.Failures {
		log.Printf("Failed to describe task: %s, reason: %s", *failure.Arn, *failure.Reason)
	}

	pods := make([]*v1.Pod, 0, len(describeResult.Tasks))

	for _, t := range describeResult.Tasks {
		p, err := p.taskToPod(t)
		if err != nil {
			log.Println(err)
			continue
		}
		pods = append(pods, p)
	}

	return pods, nil
}

// Capacity returns a resource list containing the capacity limits.
func (p *Provider) Capacity() v1.ResourceList {
	return v1.ResourceList{
		"cpu":    resource.MustParse(p.cpu),
		"memory": resource.MustParse(p.memory),
		"pods":   resource.MustParse(p.pods),
	}
}

// NodeConditions returns a list of conditions (Ready, OutOfDisk, etc), for updates to the node status
// within Kubernetes.
func (p *Provider) NodeConditions() []v1.NodeCondition {
	// TODO: Make this configurable
	return []v1.NodeCondition{
		{
			Type:               "Ready",
			Status:             v1.ConditionTrue,
			LastHeartbeatTime:  metav1.Now(),
			LastTransitionTime: metav1.Now(),
			Reason:             "KubeletReady",
			Message:            "kubelet is ready.",
		},
		{
			Type:               "OutOfDisk",
			Status:             v1.ConditionFalse,
			LastHeartbeatTime:  metav1.Now(),
			LastTransitionTime: metav1.Now(),
			Reason:             "KubeletHasSufficientDisk",
			Message:            "kubelet has sufficient disk space available",
		},
		{
			Type:               "MemoryPressure",
			Status:             v1.ConditionFalse,
			LastHeartbeatTime:  metav1.Now(),
			LastTransitionTime: metav1.Now(),
			Reason:             "KubeletHasSufficientMemory",
			Message:            "kubelet has sufficient memory available",
		},
		{
			Type:               "DiskPressure",
			Status:             v1.ConditionFalse,
			LastHeartbeatTime:  metav1.Now(),
			LastTransitionTime: metav1.Now(),
			Reason:             "KubeletHasNoDiskPressure",
			Message:            "kubelet has no disk pressure",
		},
		{
			Type:               "NetworkUnavailable",
			Status:             v1.ConditionFalse,
			LastHeartbeatTime:  metav1.Now(),
			LastTransitionTime: metav1.Now(),
			Reason:             "RouteCreated",
			Message:            "RouteController created a route",
		},
	}
}

// NodeAddresses returns a list of addresses for the node status
// within Kubernetes.
func (p *Provider) NodeAddresses() []v1.NodeAddress {
	return []v1.NodeAddress{
		{
			Type:    "InternalIP",
			Address: p.internalIP,
		},
	}
}

// NodeDaemonEndpoints returns NodeDaemonEndpoints for the node status
// within Kubernetes.
func (p *Provider) NodeDaemonEndpoints() *v1.NodeDaemonEndpoints {
	return &v1.NodeDaemonEndpoints{
		KubeletEndpoint: v1.DaemonEndpoint{
			Port: p.daemonEndpointPort,
		},
	}
}

// OperatingSystem returns the operating system for this provider.
// This is a noop to default to Linux for now.
func (p *Provider) OperatingSystem() string {
	return providers.OperatingSystemLinux
}

func toFamily(namespace, name string) string {
	return fmt.Sprintf("%s_%s", namespace, name)
}

func fromFamily(family string) (namespace, name string) {
	parts := strings.Split(family, "_")
	namespace = parts[0]
	name = parts[1]
	return
}

func toContainerStatus(containerDefinition *ecs.ContainerDefinition, container *ecs.Container) v1.ContainerStatus {
	var reason string

	if container.Reason != nil {
		reason = *container.Reason
	} else {
		reason = ""
	}

	var state v1.ContainerState
	var ready bool
	if *container.LastStatus == "RUNNING" {
		state = v1.ContainerState{
			Running: &v1.ContainerStateRunning{},
		}
		ready = true
	} else if *container.LastStatus == "STOPPED" {
		state = v1.ContainerState{
			Terminated: &v1.ContainerStateTerminated{
				ExitCode: int32(*container.ExitCode),
				Message:  reason,
				Reason:   "",
			},
		}
		ready = false
	} else {
		state = v1.ContainerState{
			Waiting: &v1.ContainerStateWaiting{
				Message: reason,
				Reason:  "",
			},
		}
		ready = false
	}

	return v1.ContainerStatus{
		Name:         *container.Name,
		State:        state,
		Ready:        ready,
		RestartCount: 0,
		Image:        *containerDefinition.Image,
		ImageID:      "",
		ContainerID:  "",
	}
}

func toPodStatus(task *ecs.Task, containerStatuses []v1.ContainerStatus) v1.PodStatus {
	var createdAt metav1.Time
	if task.CreatedAt != nil {
		createdAt = metav1.NewTime(*task.CreatedAt)
	}

	ip := ""
	if len(task.Attachments) > 0 {
		for _, attachment := range task.Attachments {
			if *attachment.Type != "ElasticNetworkInterface" {
				continue
			}

			for _, detail := range attachment.Details {
				if *detail.Name != "privateIPv4Address" {
					continue
				}

				ip = *detail.Value
			}
		}
	}

	phase := v1.PodUnknown
	conditions := []v1.PodCondition{}
	message := ""

	if *task.LastStatus == "PROVISIONING" {
		phase = v1.PodPending
		conditions = []v1.PodCondition{v1.PodCondition{
			Type:   v1.PodReady,
			Status: v1.ConditionFalse,
		}, v1.PodCondition{
			Type:   v1.PodInitialized,
			Status: v1.ConditionFalse,
		}, v1.PodCondition{
			Type:   v1.PodScheduled,
			Status: v1.ConditionFalse,
		}}
	} else if *task.LastStatus == "PENDING" {
		phase = v1.PodPending
		conditions = []v1.PodCondition{v1.PodCondition{
			Type:   v1.PodReady,
			Status: v1.ConditionFalse,
		}, v1.PodCondition{
			Type:   v1.PodInitialized,
			Status: v1.ConditionFalse,
		}, v1.PodCondition{
			Type:   v1.PodScheduled,
			Status: v1.ConditionTrue,
		}}
	} else if *task.LastStatus == "RUNNING" {
		phase = v1.PodRunning
		conditions = []v1.PodCondition{v1.PodCondition{
			Type:   v1.PodReady,
			Status: v1.ConditionTrue,
		}, v1.PodCondition{
			Type:   v1.PodInitialized,
			Status: v1.ConditionTrue,
		}, v1.PodCondition{
			Type:   v1.PodScheduled,
			Status: v1.ConditionTrue,
		}}
	} else if *task.LastStatus == "STOPPED" {
		phase = v1.PodSucceeded
		conditions = []v1.PodCondition{v1.PodCondition{
			Type:   v1.PodReady,
			Status: v1.ConditionTrue,
		}, v1.PodCondition{
			Type:   v1.PodInitialized,
			Status: v1.ConditionTrue,
		}, v1.PodCondition{
			Type:   v1.PodScheduled,
			Status: v1.ConditionTrue,
		}}
	}

	if task.StoppedReason != nil {
		message = *task.StoppedReason
	}

	return v1.PodStatus{
		Phase:             phase,
		Conditions:        conditions,
		Message:           message,
		Reason:            "",
		HostIP:            ip,
		PodIP:             ip,
		StartTime:         &createdAt,
		ContainerStatuses: containerStatuses,
	}
}

func (p *Provider) taskToPod(task *ecs.Task) (*v1.Pod, error) {
	definitionResult, err := p.ecsClient.DescribeTaskDefinition(&ecs.DescribeTaskDefinitionInput{
		TaskDefinition: task.TaskDefinitionArn,
	})

	if err != nil {
		log.Printf("Failed fetching definition for: %s, err: %s", *task.TaskArn, err)
		return nil, err
	}

	definition := definitionResult.TaskDefinition

	containers := make([]v1.Container, 0, len(task.Containers))
	containerStatuses := make([]v1.ContainerStatus, 0, len(task.Containers))
	for i, c := range task.Containers {
		containerDefinition := definition.ContainerDefinitions[i]

		container := v1.Container{
			Name:    *c.Name,
			Image:   *containerDefinition.Image,
			Command: toStrings(containerDefinition.EntryPoint),
			Args:    toStrings(containerDefinition.Command),
			Resources: v1.ResourceRequirements{
				Limits: v1.ResourceList{
					v1.ResourceCPU:    resource.MustParse(fmt.Sprintf("%d", *containerDefinition.Cpu)),
					v1.ResourceMemory: resource.MustParse(fmt.Sprintf("%dMi", *containerDefinition.Memory)),
				},
				Requests: v1.ResourceList{
					// TODO Support CPU soft limit
					// v1.ResourceCPU:    resource.MustParse(fmt.Sprintf("%d", int64(c.Resources.Requests.CPU))),
					v1.ResourceMemory: resource.MustParse(fmt.Sprintf("%dMi", *containerDefinition.MemoryReservation)),
				},
			},
			Ports: make([]v1.ContainerPort, 0, len(containerDefinition.PortMappings)),
			Env:   make([]v1.EnvVar, 0, len(containerDefinition.Environment)),
		}

		if containerDefinition.WorkingDirectory != nil {
			container.WorkingDir = *containerDefinition.WorkingDirectory
		}

		for _, mapping := range containerDefinition.PortMappings {
			container.Ports = append(container.Ports, v1.ContainerPort{
				ContainerPort: int32(*mapping.ContainerPort),
				HostPort:      int32(*mapping.HostPort),
				Protocol:      fromECSProtocol(*mapping.Protocol),
			})
		}

		for _, env := range containerDefinition.Environment {
			container.Env = append(container.Env, v1.EnvVar{
				Name:  *env.Name,
				Value: *env.Value,
			})
		}

		containers = append(containers, container)

		// Add to containerStatuses
		containerStatuses = append(containerStatuses, toContainerStatus(containerDefinition, c))
	}

	namespace, name := fromFamily(*definition.Family)

	annotations := make(map[string]string)

	annotations[cpuAnnotation] = *task.Cpu
	annotations[memoryAnnotation] = *task.Memory

	if definition.TaskRoleArn != nil {
		annotations[iamRoleAnnotation] = *definition.TaskRoleArn
	}

	pod := v1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   namespace,
			Name:        name,
			UID:         types.UID(*task.StartedBy),
			Annotations: annotations,
		},
		Spec: v1.PodSpec{
			NodeName:   p.nodeName,
			Volumes:    []v1.Volume{},
			Containers: containers,
		},
		Status: toPodStatus(task, containerStatuses),
	}

	return &pod, nil
}

func toECSProtocol(pro v1.Protocol) string {
	switch pro {
	case v1.ProtocolUDP:
		return "udp"
	default:
		return "tcp"
	}
}

func fromECSProtocol(pro string) v1.Protocol {
	switch pro {
	case "udp":
		return v1.ProtocolUDP
	default:
		return v1.ProtocolTCP
	}
}
