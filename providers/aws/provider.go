package aws

import (
	"context"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/virtual-kubelet/virtual-kubelet/manager"
	"github.com/virtual-kubelet/virtual-kubelet/providers/aws/fargate"
	"github.com/virtual-kubelet/virtual-kubelet/vkubelet/api"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// FargateProvider implements the virtual-kubelet provider interface.
type FargateProvider struct {
	resourceManager    *manager.ResourceManager
	nodeName           string
	operatingSystem    string
	internalIP         string
	daemonEndpointPort int32

	// AWS resources.
	region         string
	subnets        []string
	securityGroups []string

	// Fargate resources.
	cluster                 *fargate.Cluster
	clusterName             string
	capacity                capacity
	assignPublicIPv4Address bool
	executionRoleArn        string
	cloudWatchLogGroupName  string
	platformVersion         string
	lastTransitionTime      time.Time
}

// Capacity represents the provisioned capacity on a Fargate cluster.
type capacity struct {
	cpu     string
	memory  string
	storage string
	pods    string
}

var (
	errNotImplemented = fmt.Errorf("not implemented by Fargate provider")
)

// NewFargateProvider creates a new Fargate provider.
func NewFargateProvider(
	config string,
	rm *manager.ResourceManager,
	nodeName string,
	operatingSystem string,
	internalIP string,
	daemonEndpointPort int32) (*FargateProvider, error) {

	// Create the Fargate provider.
	log.Println("Creating Fargate provider.")

	p := FargateProvider{
		resourceManager:    rm,
		nodeName:           nodeName,
		operatingSystem:    operatingSystem,
		internalIP:         internalIP,
		daemonEndpointPort: daemonEndpointPort,
	}

	// Read the Fargate provider configuration file.
	err := p.loadConfigFile(config)
	if err != nil {
		err = fmt.Errorf("failed to load configuration file %s: %v", config, err)
		return nil, err
	}

	log.Printf("Loaded provider configuration file %s.", config)

	// Find or create the configured Fargate cluster.
	clusterConfig := fargate.ClusterConfig{
		Region:                  p.region,
		Name:                    p.clusterName,
		NodeName:                nodeName,
		Subnets:                 p.subnets,
		SecurityGroups:          p.securityGroups,
		AssignPublicIPv4Address: p.assignPublicIPv4Address,
		ExecutionRoleArn:        p.executionRoleArn,
		CloudWatchLogGroupName:  p.cloudWatchLogGroupName,
		PlatformVersion:         p.platformVersion,
	}

	p.cluster, err = fargate.NewCluster(&clusterConfig)
	if err != nil {
		err = fmt.Errorf("failed to create Fargate cluster: %v", err)
		return nil, err
	}

	p.lastTransitionTime = time.Now()

	log.Printf("Created Fargate provider: %+v.", p)

	return &p, nil
}

// CreatePod takes a Kubernetes Pod and deploys it within the Fargate provider.
func (p *FargateProvider) CreatePod(ctx context.Context, pod *corev1.Pod) error {
	log.Printf("Received CreatePod request for %+v.\n", pod)

	fgPod, err := fargate.NewPod(p.cluster, pod)
	if err != nil {
		log.Printf("Failed to create pod: %v.\n", err)
		return err
	}

	err = fgPod.Start()
	if err != nil {
		log.Printf("Failed to start pod: %v.\n", err)
		return err
	}

	return nil
}

// UpdatePod takes a Kubernetes Pod and updates it within the provider.
func (p *FargateProvider) UpdatePod(ctx context.Context, pod *corev1.Pod) error {
	log.Printf("Received UpdatePod request for %s/%s.\n", pod.Namespace, pod.Name)
	return errNotImplemented
}

// DeletePod takes a Kubernetes Pod and deletes it from the provider.
func (p *FargateProvider) DeletePod(ctx context.Context, pod *corev1.Pod) error {
	log.Printf("Received DeletePod request for %s/%s.\n", pod.Namespace, pod.Name)

	fgPod, err := p.cluster.GetPod(pod.Namespace, pod.Name)
	if err != nil {
		log.Printf("Failed to get pod: %v.\n", err)
		return err
	}

	err = fgPod.Stop()
	if err != nil {
		log.Printf("Failed to stop pod: %v.\n", err)
		return err
	}

	return nil
}

// GetPod retrieves a pod by name from the provider (can be cached).
func (p *FargateProvider) GetPod(ctx context.Context, namespace, name string) (*corev1.Pod, error) {
	log.Printf("Received GetPod request for %s/%s.\n", namespace, name)

	pod, err := p.cluster.GetPod(namespace, name)
	if err != nil {
		log.Printf("Failed to get pod: %v.\n", err)
		return nil, err
	}

	spec, err := pod.GetSpec()
	if err != nil {
		log.Printf("Failed to get pod spec: %v.\n", err)
		return nil, err
	}

	log.Printf("Responding to GetPod: %+v.\n", spec)

	return spec, nil
}

// GetContainerLogs retrieves the logs of a container by name from the provider.
func (p *FargateProvider) GetContainerLogs(ctx context.Context, namespace, podName, containerName string, opts api.ContainerLogOpts) (io.ReadCloser, error) {
	log.Printf("Received GetContainerLogs request for %s/%s/%s.\n", namespace, podName, containerName)
	return p.cluster.GetContainerLogs(namespace, podName, containerName, opts)
}

// GetPodFullName retrieves the full pod name as defined in the provider context.
func (p *FargateProvider) GetPodFullName(namespace string, pod string) string {
	return ""
}

// RunInContainer executes a command in a container in the pod, copying data
// between in/out/err and the container's stdin/stdout/stderr.
func (p *FargateProvider) RunInContainer(ctx context.Context, namespace, podName, containerName string, cmd []string, attach api.AttachIO) error {
	return errNotImplemented
}

// GetPodStatus retrieves the status of a pod by name from the provider.
func (p *FargateProvider) GetPodStatus(ctx context.Context, namespace, name string) (*corev1.PodStatus, error) {
	log.Printf("Received GetPodStatus request for %s/%s.\n", namespace, name)

	pod, err := p.cluster.GetPod(namespace, name)
	if err != nil {
		log.Printf("Failed to get pod: %v.\n", err)
		return nil, err
	}

	status := pod.GetStatus()

	log.Printf("Responding to GetPodStatus: %+v.\n", status)

	return &status, nil
}

// GetPods retrieves a list of all pods running on the provider (can be cached).
func (p *FargateProvider) GetPods(ctx context.Context) ([]*corev1.Pod, error) {
	log.Println("Received GetPods request.")

	pods, err := p.cluster.GetPods()
	if err != nil {
		log.Printf("Failed to get pods: %v.\n", err)
		return nil, err
	}

	var result []*corev1.Pod

	for _, pod := range pods {
		spec, err := pod.GetSpec()
		if err != nil {
			log.Printf("Failed to get pod spec: %v.\n", err)
			continue
		}

		result = append(result, spec)
	}

	log.Printf("Responding to GetPods: %+v.\n", result)

	return result, nil
}

// Capacity returns a resource list with the capacity constraints of the provider.
func (p *FargateProvider) Capacity(ctx context.Context) corev1.ResourceList {
	log.Println("Received Capacity request.")

	return corev1.ResourceList{
		corev1.ResourceCPU:     resource.MustParse(p.capacity.cpu),
		corev1.ResourceMemory:  resource.MustParse(p.capacity.memory),
		corev1.ResourceStorage: resource.MustParse(p.capacity.storage),
		corev1.ResourcePods:    resource.MustParse(p.capacity.pods),
	}
}

// NodeConditions returns a list of conditions (Ready, OutOfDisk, etc), which is polled
// periodically to update the node status within Kubernetes.
func (p *FargateProvider) NodeConditions(ctx context.Context) []corev1.NodeCondition {
	log.Println("Received NodeConditions request.")

	lastHeartbeatTime := metav1.Now()
	lastTransitionTime := metav1.NewTime(p.lastTransitionTime)
	lastTransitionReason := "Fargate cluster is ready"
	lastTransitionMessage := "ok"

	// Return static thumbs-up values for all conditions.
	return []corev1.NodeCondition{
		{
			Type:               corev1.NodeReady,
			Status:             corev1.ConditionTrue,
			LastHeartbeatTime:  lastHeartbeatTime,
			LastTransitionTime: lastTransitionTime,
			Reason:             lastTransitionReason,
			Message:            lastTransitionMessage,
		},
		{
			Type:               corev1.NodeOutOfDisk,
			Status:             corev1.ConditionFalse,
			LastHeartbeatTime:  lastHeartbeatTime,
			LastTransitionTime: lastTransitionTime,
			Reason:             lastTransitionReason,
			Message:            lastTransitionMessage,
		},
		{
			Type:               corev1.NodeMemoryPressure,
			Status:             corev1.ConditionFalse,
			LastHeartbeatTime:  lastHeartbeatTime,
			LastTransitionTime: lastTransitionTime,
			Reason:             lastTransitionReason,
			Message:            lastTransitionMessage,
		},
		{
			Type:               corev1.NodeDiskPressure,
			Status:             corev1.ConditionFalse,
			LastHeartbeatTime:  lastHeartbeatTime,
			LastTransitionTime: lastTransitionTime,
			Reason:             lastTransitionReason,
			Message:            lastTransitionMessage,
		},
		{
			Type:               corev1.NodeNetworkUnavailable,
			Status:             corev1.ConditionFalse,
			LastHeartbeatTime:  lastHeartbeatTime,
			LastTransitionTime: lastTransitionTime,
			Reason:             lastTransitionReason,
			Message:            lastTransitionMessage,
		},
		{
			Type:               "KubeletConfigOk",
			Status:             corev1.ConditionTrue,
			LastHeartbeatTime:  lastHeartbeatTime,
			LastTransitionTime: lastTransitionTime,
			Reason:             lastTransitionReason,
			Message:            lastTransitionMessage,
		},
	}
}

// NodeAddresses returns a list of addresses for the node status within Kubernetes.
func (p *FargateProvider) NodeAddresses(ctx context.Context) []corev1.NodeAddress {
	log.Println("Received NodeAddresses request.")

	return []corev1.NodeAddress{
		{
			Type:    corev1.NodeInternalIP,
			Address: p.internalIP,
		},
	}
}

// NodeDaemonEndpoints returns NodeDaemonEndpoints for the node status within Kubernetes.
func (p *FargateProvider) NodeDaemonEndpoints(ctx context.Context) *corev1.NodeDaemonEndpoints {
	log.Println("Received NodeDaemonEndpoints request.")

	return &corev1.NodeDaemonEndpoints{
		KubeletEndpoint: corev1.DaemonEndpoint{
			Port: p.daemonEndpointPort,
		},
	}
}

// OperatingSystem returns the operating system the provider is for.
func (p *FargateProvider) OperatingSystem() string {
	log.Println("Received OperatingSystem request.")

	return p.operatingSystem
}
