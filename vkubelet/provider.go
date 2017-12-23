package vkubelet

import (
	"github.com/virtual-kubelet/virtual-kubelet/providers/azure"
	"github.com/virtual-kubelet/virtual-kubelet/providers/hypersh"
	"github.com/virtual-kubelet/virtual-kubelet/providers/web"
	"k8s.io/api/core/v1"
)

// Compile time proof that our implementations meet the Provider interface.
var _ Provider = (*azure.ACIProvider)(nil)
var _ Provider = (*hypersh.HyperProvider)(nil)
var _ Provider = (*web.BrokerProvider)(nil)

// Provider contains the methods required to implement a virtual-kubelet provider.
type Provider interface {
	// CreatePod takes a Kubernetes Pod and deploys it within the provider.
	CreatePod(pod *v1.Pod) error

	// UpdatePod takes a Kubernetes Pod and updates it within the provider.
	UpdatePod(pod *v1.Pod) error

	// DeletePod takes a Kubernetes Pod and deletes it from the provider.
	DeletePod(pod *v1.Pod) error

	// GetPod retrieves a pod by name from the provider (can be cached).
	GetPod(namespace, name string) (*v1.Pod, error)

	// GetContainerLogs retrieves the logs of a container by name from the provider.
	GetContainerLogs(namespace, podName, containerName string, tail int) (string, error)

	// GetPodStatus retrieves the status of a pod by name from the provider.
	GetPodStatus(namespace, name string) (*v1.PodStatus, error)

	// GetPods retrieves a list of all pods running on the provider (can be cached).
	GetPods() ([]*v1.Pod, error)

	// Capacity returns a resource list with the capacity constraints of the provider.
	Capacity() v1.ResourceList

	// NodeConditions returns a list of conditions (Ready, OutOfDisk, etc), which is polled periodically to update the node status
	// within Kuberentes.
	NodeConditions() []v1.NodeCondition

	// NodeAddresses returns a list of addresses for the node status
	// within Kuberentes.
	NodeAddresses() []v1.NodeAddress

	// NodeDaemonEndpoints returns NodeDaemonEndpoints for the node status
	// within Kuberentes.
	NodeDaemonEndpoints() *v1.NodeDaemonEndpoints

	// OperatingSystem returns the operating system the provider is for.
	OperatingSystem() string
}
