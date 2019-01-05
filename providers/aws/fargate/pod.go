package fargate

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecs"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sTypes "k8s.io/apimachinery/pkg/types"
)

const (
	// Prefixes for objects created in Fargate.
	taskDefFamilyPrefix = "vk-podspec"
	taskTagPrefix       = "vk-pod"

	// Task status strings.
	taskStatusProvisioning = "PROVISIONING"
	taskStatusPending      = "PENDING"
	taskStatusRunning      = "RUNNING"
	taskStatusStopped      = "STOPPED"

	// Task attachment types.
	taskAttachmentENI                   = "ElasticNetworkInterface"
	taskAttachmentENIPrivateIPv4Address = "privateIPv4Address"

	// Reason used for task state changes.
	taskGenericReason = "Initiated by user"

	// Annotation to configure the task role.
	taskRoleAnnotation = "iam.amazonaws.com/role"
)

// Pod is the representation of a Kubernetes pod in Fargate.
type Pod struct {
	// Kubernetes pod properties.
	namespace string
	name      string
	uid       k8sTypes.UID

	// Fargate task properties.
	cluster         *Cluster
	taskDefArn      string
	taskArn         string
	taskRoleArn     string
	taskStatus      string
	taskRefreshTime time.Time
	taskCPU         int64
	taskMemory      int64
	containers      map[string]*container
}

// NewPod creates a new Kubernetes pod on Fargate.
func NewPod(cluster *Cluster, pod *corev1.Pod) (*Pod, error) {
	api := client.api

	// Initialize the pod.
	fgPod := &Pod{
		namespace:  pod.Namespace,
		name:       pod.Name,
		uid:        pod.UID,
		cluster:    cluster,
		containers: make(map[string]*container),
	}

	tag := fgPod.buildTaskDefinitionTag()

	// Create a task definition matching the pod spec.
	taskDef := &ecs.RegisterTaskDefinitionInput{
		Family:                  aws.String(tag),
		RequiresCompatibilities: []*string{aws.String(ecs.CompatibilityFargate)},
		NetworkMode:             aws.String(ecs.NetworkModeAwsvpc),
		ContainerDefinitions:    []*ecs.ContainerDefinition{},
	}

	// For each container in the pod...
	for _, containerSpec := range pod.Spec.Containers {
		// Create a container definition.
		cntr, err := newContainer(&containerSpec)
		if err != nil {
			return nil, err
		}

		// Volumes from could be defined in an annotation in the form volumesFrom: user1=sharer1,user2=sharer2
		cntr.definition.VolumesFrom = getVolumesFrom(*cntr.definition.Name, pod.Annotations["volumesFrom"])

		// Configure container logs to be sent to CloudWatch Logs if enabled.
		if cluster.cloudWatchLogGroupName != "" {
			cntr.configureLogs(cluster.region, cluster.cloudWatchLogGroupName, tag)
		}

		// Add the container's resource requirements to its pod's total resource requirements.
		fgPod.taskCPU += *cntr.definition.Cpu
		fgPod.taskMemory += *cntr.definition.Memory

		// Insert the container to its pod.
		fgPod.containers[containerSpec.Name] = cntr

		// Insert container definition to the task definition.
		taskDef.ContainerDefinitions = append(taskDef.ContainerDefinitions, &cntr.definition)
	}

	// This is something of a hack - it merely marks initContainers as not "essential"
	// so the Fargate task won't stop when these stop. There's no guarantee an initContainer will start up first.
	for _, containerSpec := range pod.Spec.InitContainers {
		// Create a container definition.
		cntr, err := newContainer(&containerSpec)
		if err != nil {
			return nil, err
		}

		// Set the Essential flag off for init Containers
		cntr.definition.Essential = aws.Bool(false)

		// Configure container logs to be sent to CloudWatch Logs if enabled.
		if cluster.cloudWatchLogGroupName != "" {
			cntr.configureLogs(cluster.region, cluster.cloudWatchLogGroupName, tag)
		}

		// Add the container's resource requirements to its pod's total resource requirements.
		fgPod.taskCPU += *cntr.definition.Cpu
		fgPod.taskMemory += *cntr.definition.Memory

		// Insert the container to its pod.
		fgPod.containers[containerSpec.Name] = cntr

		// Insert container definition to the task definition.
		taskDef.ContainerDefinitions = append(taskDef.ContainerDefinitions, &cntr.definition)
	}

	// Set task resource limits.
	err := fgPod.mapTaskSize()
	if err != nil {
		return nil, err
	}

	taskDef.Cpu = aws.String(strconv.Itoa(int(fgPod.taskCPU)))
	taskDef.Memory = aws.String(strconv.Itoa(int(fgPod.taskMemory)))

	// Set a custom task execution IAM role if configured.
	if cluster.executionRoleArn != "" {
		taskDef.ExecutionRoleArn = aws.String(cluster.executionRoleArn)
	}

	// Set a custom task IAM role if configured.
	if val, ok := pod.Annotations[taskRoleAnnotation]; ok {
		taskDef.TaskRoleArn = aws.String(val)
		fgPod.taskRoleArn = val
	}

	// Register the task definition with Fargate.
	log.Printf("RegisterTaskDefinition input:%+v", taskDef)
	output, err := api.RegisterTaskDefinition(taskDef)
	log.Printf("RegisterTaskDefinition err:%+v output:%+v", err, output)
	if err != nil {
		err = fmt.Errorf("failed to register task definition: %v", err)
		return nil, err
	}

	// Save the registered task definition ARN.
	fgPod.taskDefArn = *output.TaskDefinition.TaskDefinitionArn

	if cluster != nil {
		cluster.InsertPod(fgPod, tag)
	}

	return fgPod, nil
}

// getVolumesFrom finds any VolumesFrom annotations relating to the container with this name
func getVolumesFrom(name string, annotation string) (v []*ecs.VolumeFrom) {
	for _, vfItem := range strings.Split(annotation, ",") {
		vf := strings.Split(vfItem, "=")
		if len(vf) != 2 {
			log.Printf("Ignoring unparseable volumesFrom annotation: %s", vf)
		} else {
			user := strings.TrimSpace(vf[0])
			sharer := strings.TrimSpace(vf[1])
			if user == name {
				log.Printf("Container %s shares volumes from %s", user, sharer)
				volumeFrom := ecs.VolumeFrom{SourceContainer: aws.String(sharer)}
				v = append(v, &volumeFrom)
			}
		}
	}

	return v
}

// NewPodFromTag creates a new pod identified by a tag.
func NewPodFromTag(cluster *Cluster, tag string) (*Pod, error) {
	data := strings.Split(tag, "_")

	if len(data) < 4 ||
		data[0] != taskDefFamilyPrefix ||
		data[1] != cluster.name {
		return nil, fmt.Errorf("invalid tag")
	}

	pod := &Pod{
		namespace:  data[2],
		name:       data[3],
		cluster:    cluster,
		containers: make(map[string]*container),
	}

	return pod, nil
}

// Start deploys and runs a Kubernetes pod on Fargate.
func (pod *Pod) Start() error {
	api := client.api

	// Pods always get an ENI with a private IPv4 address in customer subnet.
	// Assign a public IPv4 address to the ENI only if requested.
	assignPublicIPAddress := ecs.AssignPublicIpDisabled
	if pod.cluster.assignPublicIPv4Address {
		assignPublicIPAddress = ecs.AssignPublicIpEnabled
	}

	// Start the task.
	runTaskInput := &ecs.RunTaskInput{
		Cluster:    aws.String(pod.cluster.name),
		Count:      aws.Int64(1),
		LaunchType: aws.String(ecs.LaunchTypeFargate),
		NetworkConfiguration: &ecs.NetworkConfiguration{
			AwsvpcConfiguration: &ecs.AwsVpcConfiguration{
				AssignPublicIp: aws.String(assignPublicIPAddress),
				SecurityGroups: aws.StringSlice(pod.cluster.securityGroups),
				Subnets:        aws.StringSlice(pod.cluster.subnets),
			},
		},
		PlatformVersion: aws.String(pod.cluster.platformVersion),
		StartedBy:       aws.String(pod.buildTaskTag()),
		TaskDefinition:  aws.String(pod.taskDefArn),
	}

	log.Printf("RunTask input:%+v", runTaskInput)
	runTaskOutput, err := api.RunTask(runTaskInput)
	log.Printf("RunTask err:%+v output:%+v", err, runTaskOutput)
	if err != nil || len(runTaskOutput.Tasks) == 0 {
		if len(runTaskOutput.Failures) != 0 {
			err = fmt.Errorf("reason: %s", *runTaskOutput.Failures[0].Reason)
		}
		err = fmt.Errorf("failed to run task: %v", err)
		return err
	}

	// Save the task ARN.
	pod.taskArn = *runTaskOutput.Tasks[0].TaskArn

	return nil
}

// Stop stops a running Kubernetes pod on Fargate.
func (pod *Pod) Stop() error {
	api := client.api

	// Stop the task.
	stopTaskInput := &ecs.StopTaskInput{
		Cluster: aws.String(pod.cluster.name),
		Reason:  aws.String(taskGenericReason),
		Task:    aws.String(pod.taskArn),
	}

	log.Printf("StopTask input:%+v", stopTaskInput)
	stopTaskOutput, err := api.StopTask(stopTaskInput)
	log.Printf("StopTask err:%+v output:%+v", err, stopTaskOutput)
	if err != nil {
		err = fmt.Errorf("failed to stop task: %v", err)
		return err
	}

	// Deregister the task definition.
	_, err = api.DeregisterTaskDefinition(&ecs.DeregisterTaskDefinitionInput{
		TaskDefinition: aws.String(pod.taskDefArn),
	})
	if err != nil {
		log.Printf("Failed to deregister task definition: %v", err)
	}

	// Remove the pod from its cluster.
	if pod.cluster != nil {
		pod.cluster.RemovePod(pod.buildTaskDefinitionTag())
	}

	return nil
}

// GetSpec returns the specification of a Kubernetes pod on Fargate.
func (pod *Pod) GetSpec() (*corev1.Pod, error) {
	task, err := pod.describe()
	if err != nil {
		return nil, err
	}

	return pod.getSpec(task)
}

// GetStatus returns the status of a Kubernetes pod on Fargate.
func (pod *Pod) GetStatus() corev1.PodStatus {
	task, err := pod.describe()
	if err != nil {
		return corev1.PodStatus{Phase: corev1.PodUnknown}
	}

	return pod.getStatus(task)
}

// BuildTaskDefinitionTag returns the task definition tag for this pod.
func (pod *Pod) buildTaskDefinitionTag() string {
	return buildTaskDefinitionTag(pod.cluster.name, pod.namespace, pod.name)
}

// buildTaskDefinitionTag builds a task definition tag from its components.
func buildTaskDefinitionTag(clusterName string, namespace string, name string) string {
	// vk-podspec_cluster_namespacae_podname
	return fmt.Sprintf("%s_%s_%s_%s", taskDefFamilyPrefix, clusterName, namespace, name)
}

// BuildTaskTag returns the pod's task tag, used for mapping a task back to its pod.
func (pod *Pod) buildTaskTag() string {
	return fmt.Sprintf("%s", pod.uid)
}

// mapTaskSize maps Kubernetes pod resource requirements to a Fargate task size.
func (pod *Pod) mapTaskSize() error {
	//
	// Kubernetes pods do not have explicit resource requirements; their containers do. Pod resource
	// requirements are the sum of the pod's containers' requirements.
	//
	// Fargate tasks have explicit CPU and memory limits. Both are required and specify the maximum
	// amount of resources for the task. The limits must match a task size on taskSizeTable.
	//
	var cpu int64
	var memory int64

	// Find the smallest Fargate task size that can satisfy the total resource request.
	for _, row := range taskSizeTable {
		if pod.taskCPU <= row.cpu {
			for mem := row.memory.min; mem <= row.memory.max; mem += row.memory.inc {
				if pod.taskMemory <= mem/MiB {
					cpu = row.cpu
					memory = mem / MiB
					break
				}
			}

			if cpu != 0 {
				break
			}
		}
	}

	log.Printf("Mapped resource requirements (cpu:%v, memory:%v) to task size (cpu:%v, memory:%v)",
		pod.taskCPU, pod.taskMemory, cpu, memory)

	// Fail if the resource requirements cannot be satisfied by any Fargate task size.
	if cpu == 0 {
		return fmt.Errorf("resource requirements (cpu:%v, memory:%v) are too high",
			pod.taskCPU, pod.taskMemory)
	}

	// Fargate task CPU size is specified in vCPU/1024s and memory size is specified in MiBs.
	pod.taskCPU = cpu
	pod.taskMemory = memory

	return nil
}

// Describe retrieves the status of a Kubernetes pod from Fargate.
func (pod *Pod) describe() (*ecs.Task, error) {
	api := client.api

	// Describe the task.
	describeTasksInput := &ecs.DescribeTasksInput{
		Cluster: aws.String(pod.cluster.name),
		Tasks:   []*string{aws.String(pod.taskArn)},
	}

	describeTasksOutput, err := api.DescribeTasks(describeTasksInput)
	if err != nil || len(describeTasksOutput.Tasks) == 0 {
		if len(describeTasksOutput.Failures) != 0 {
			err = fmt.Errorf("reason: %s", *describeTasksOutput.Failures[0].Reason)
		}
		err = fmt.Errorf("failed to describe task: %v", err)
		return nil, err
	}

	task := describeTasksOutput.Tasks[0]

	pod.taskStatus = *task.LastStatus
	pod.taskRefreshTime = time.Now()

	return task, nil
}

// GetSpec returns the specification of a Kubernetes pod on Fargate.
func (pod *Pod) getSpec(task *ecs.Task) (*corev1.Pod, error) {
	containers := make([]corev1.Container, 0, len(task.Containers))

	for _, c := range task.Containers {
		cntrDef := pod.containers[*c.Name].definition

		cntr := corev1.Container{
			Name:    *c.Name,
			Image:   *cntrDef.Image,
			Command: aws.StringValueSlice(cntrDef.EntryPoint),
			Args:    aws.StringValueSlice(cntrDef.Command),
			Resources: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse(fmt.Sprintf("%d", *cntrDef.Cpu)),
					corev1.ResourceMemory: resource.MustParse(fmt.Sprintf("%dMi", *cntrDef.Memory)),
				},
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse(fmt.Sprintf("%d", *cntrDef.Cpu)),
					corev1.ResourceMemory: resource.MustParse(fmt.Sprintf("%dMi", *cntrDef.MemoryReservation)),
				},
			},
			Ports: make([]corev1.ContainerPort, 0, len(cntrDef.PortMappings)),
			Env:   make([]corev1.EnvVar, 0, len(cntrDef.Environment)),
		}

		if cntrDef.WorkingDirectory != nil {
			cntr.WorkingDir = *cntrDef.WorkingDirectory
		}

		for _, mapping := range cntrDef.PortMappings {
			cntr.Ports = append(cntr.Ports, corev1.ContainerPort{
				ContainerPort: int32(*mapping.ContainerPort),
				HostPort:      int32(*mapping.HostPort),
				Protocol:      corev1.ProtocolTCP,
			})
		}

		for _, env := range cntrDef.Environment {
			cntr.Env = append(cntr.Env, corev1.EnvVar{
				Name:  *env.Name,
				Value: *env.Value,
			})
		}

		containers = append(containers, cntr)
	}

	annotations := make(map[string]string)

	if pod.taskRoleArn != "" {
		annotations[taskRoleAnnotation] = pod.taskRoleArn
	}

	podSpec := corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   pod.namespace,
			Name:        pod.name,
			UID:         pod.uid,
			Annotations: annotations,
		},
		Spec: corev1.PodSpec{
			NodeName:   pod.cluster.nodeName,
			Volumes:    []corev1.Volume{},
			Containers: containers,
		},
		Status: pod.getStatus(task),
	}

	return &podSpec, nil
}

// GetStatus returns the status of a Kubernetes pod on Fargate.
func (pod *Pod) getStatus(task *ecs.Task) corev1.PodStatus {
	// Translate task status to pod phase.
	phase := corev1.PodUnknown

	switch pod.taskStatus {
	case taskStatusProvisioning:
		phase = corev1.PodPending
	case taskStatusPending:
		phase = corev1.PodPending
	case taskStatusRunning:
		phase = corev1.PodRunning
	case taskStatusStopped:
		phase = corev1.PodSucceeded
	}

	// Set pod conditions based on task's last known status.
	isScheduled := corev1.ConditionFalse
	isInitialized := corev1.ConditionFalse
	isReady := corev1.ConditionFalse

	switch pod.taskStatus {
	case taskStatusProvisioning:
		isScheduled = corev1.ConditionTrue
	case taskStatusPending:
		isScheduled = corev1.ConditionTrue
	case taskStatusRunning:
		isScheduled = corev1.ConditionTrue
		isInitialized = corev1.ConditionTrue
		isReady = corev1.ConditionTrue
	case taskStatusStopped:
		isScheduled = corev1.ConditionTrue
		isInitialized = corev1.ConditionTrue
		isReady = corev1.ConditionTrue
	}

	conditions := []corev1.PodCondition{
		corev1.PodCondition{
			Type:   corev1.PodScheduled,
			Status: isScheduled,
		},
		corev1.PodCondition{
			Type:   corev1.PodInitialized,
			Status: isInitialized,
		},
		corev1.PodCondition{
			Type:   corev1.PodReady,
			Status: isReady,
		},
	}

	// Set the pod start time as the task creation time.
	var startTime metav1.Time
	if task.CreatedAt != nil {
		startTime = metav1.NewTime(*task.CreatedAt)
	}

	// Set the pod IP address from the task ENI information.
	privateIPv4Address := ""
	for _, attachment := range task.Attachments {
		if *attachment.Type == taskAttachmentENI {
			for _, detail := range attachment.Details {
				if *detail.Name == taskAttachmentENIPrivateIPv4Address {
					privateIPv4Address = *detail.Value
				}
			}
		}
	}

	// Get statuses from all containers in this pod.
	containerStatuses := make([]corev1.ContainerStatus, 0, len(task.Containers))
	for _, cntr := range task.Containers {
		containerStatuses = append(containerStatuses, pod.containers[*cntr.Name].getStatus(cntr))
	}

	// Build the pod status structure to be reported.
	status := corev1.PodStatus{
		Phase:                 phase,
		Conditions:            conditions,
		Message:               "",
		Reason:                "",
		HostIP:                privateIPv4Address,
		PodIP:                 privateIPv4Address,
		StartTime:             &startTime,
		InitContainerStatuses: nil,
		ContainerStatuses:     containerStatuses,
		QOSClass:              corev1.PodQOSBestEffort,
	}

	return status
}
