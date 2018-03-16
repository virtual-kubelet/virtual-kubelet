package aws

import (
	"fmt"
	"log"

	"github.com/virtual-kubelet/virtual-kubelet/manager"

	corev1 "k8s.io/api/core/v1"
)

// FargateProvider implements the virtual-kubelet provider interface.
type FargateProvider struct {
	resourceManager    *manager.ResourceManager
	nodeName           string
	operatingSystem    string
	internalIP         string
	daemonEndpointPort int32
}

var (
	errNotImplemented = fmt.Errorf("Not implemented by Fargate provider")
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
	log.Printf("Received UpdatePod request for %+v.\n", pod)
	return errNotImplemented
}

// DeletePod takes a Kubernetes Pod and deletes it from the provider.
func (p *FargateProvider) DeletePod(pod *corev1.Pod) error {
	log.Printf("Received DeletePod request for %+v.\n", pod)
	return errNotImplemented
}

// GetPod retrieves a pod by name from the provider (can be cached).
func (p *FargateProvider) GetPod(namespace, name string) (*corev1.Pod, error) {
	log.Printf("Received GetPod request for %s::%s.\n", namespace, name)
	return nil, errNotImplemented
}

// GetContainerLogs retrieves the logs of a container by name from the provider.
func (p *FargateProvider) GetContainerLogs(namespace, podName, containerName string, tail int) (string, error) {
	log.Printf("Received GetContainerLogs request for %s::%s::%s.\n", namespace, podName, containerName)
	return "", errNotImplemented
}

// GetPodStatus retrieves the status of a pod by name from the provider.
func (p *FargateProvider) GetPodStatus(namespace, name string) (*corev1.PodStatus, error) {
	log.Printf("Received GetPodStatus request for %s::%s.\n", namespace, name)
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
	return nil
}

// NodeConditions returns a list of conditions (Ready, OutOfDisk, etc), which is polled
// periodically to update the node status within Kubernetes.
func (p *FargateProvider) NodeConditions() []corev1.NodeCondition {
	log.Println("Received NodeConditions request.")
	return nil
}

// NodeAddresses returns a list of addresses for the node status within Kubernetes.
func (p *FargateProvider) NodeAddresses() []corev1.NodeAddress {
	log.Println("Received NodeAddresses request.")
	return nil
}

// NodeDaemonEndpoints returns NodeDaemonEndpoints for the node status within Kubernetes.
func (p *FargateProvider) NodeDaemonEndpoints() *corev1.NodeDaemonEndpoints {
	log.Println("Received NodeDaemonEndpoints request.")
	return nil
}

// OperatingSystem returns the operating system the provider is for.
func (p *FargateProvider) OperatingSystem() string {
	log.Println("Received OperatingSystem request.")
	return p.operatingSystem
}
