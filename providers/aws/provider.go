package aws

import (
	"fmt"
	"log"
	"time"

	"github.com/virtual-kubelet/virtual-kubelet/manager"

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

	capacity                capacity
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

	log.Printf("Created Fargate provider: %+v.", p)

	return &p, nil
}

// CreatePod takes a Kubernetes Pod and deploys it within the Fargate provider.
func (p *FargateProvider) CreatePod(pod *corev1.Pod) error {
	log.Printf("Received CreatePod request for %+v.\n", pod)
	return errNotImplemented
}

// UpdatePod takes a Kubernetes Pod and updates it within the provider.
func (p *FargateProvider) UpdatePod(pod *corev1.Pod) error {
	log.Printf("Received UpdatePod request for %s/%s.\n", pod.Namespace, pod.Name)
	return errNotImplemented
}

// DeletePod takes a Kubernetes Pod and deletes it from the provider.
func (p *FargateProvider) DeletePod(pod *corev1.Pod) error {
	log.Printf("Received DeletePod request for %s/%s.\n", pod.Namespace, pod.Name)
	return errNotImplemented
}

// GetPod retrieves a pod by name from the provider (can be cached).
func (p *FargateProvider) GetPod(namespace, name string) (*corev1.Pod, error) {
	log.Printf("Received GetPod request for %s/%s.\n", namespace, name)
	return nil, errNotImplemented
}

// GetContainerLogs retrieves the logs of a container by name from the provider.
func (p *FargateProvider) GetContainerLogs(namespace, podName, containerName string, tail int) (string, error) {
	log.Printf("Received GetContainerLogs request for %s/%s/%s.\n", namespace, podName, containerName)
	return "", errNotImplemented
}

// GetPodStatus retrieves the status of a pod by name from the provider.
func (p *FargateProvider) GetPodStatus(namespace, name string) (*corev1.PodStatus, error) {
	log.Printf("Received GetPodStatus request for %s/%s.\n", namespace, name)
	return nil, errNotImplemented
}

// GetPods retrieves a list of all pods running on the provider (can be cached).
func (p *FargateProvider) GetPods() ([]*corev1.Pod, error) {
	log.Println("Received GetPods request.")
	return nil, errNotImplemented
}

// Capacity returns a resource list with the capacity constraints of the provider.
func (p *FargateProvider) Capacity() corev1.ResourceList {
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
func (p *FargateProvider) NodeConditions() []corev1.NodeCondition {
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
			Type:               corev1.NodeConfigOK,
			Status:             corev1.ConditionTrue,
			LastHeartbeatTime:  lastHeartbeatTime,
			LastTransitionTime: lastTransitionTime,
			Reason:             lastTransitionReason,
			Message:            lastTransitionMessage,
		},
	}
}

// NodeAddresses returns a list of addresses for the node status within Kubernetes.
func (p *FargateProvider) NodeAddresses() []corev1.NodeAddress {
	log.Println("Received NodeAddresses request.")

	return []corev1.NodeAddress{
		{
			Type:    corev1.NodeInternalIP,
			Address: p.internalIP,
		},
	}
}

// NodeDaemonEndpoints returns NodeDaemonEndpoints for the node status within Kubernetes.
func (p *FargateProvider) NodeDaemonEndpoints() *corev1.NodeDaemonEndpoints {
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
