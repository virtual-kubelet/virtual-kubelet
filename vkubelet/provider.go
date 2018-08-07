package vkubelet

import (
	"context"
	"io"
	"time"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/remotecommand"
	stats "k8s.io/kubernetes/pkg/kubelet/apis/stats/v1alpha1"
)

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

	// ExecInContainer executes a command in a container in the pod, copying data
	// between in/out/err and the container's stdin/stdout/stderr.
	ExecInContainer(name string, uid types.UID, container string, cmd []string, in io.Reader, out, err io.WriteCloser, tty bool, resize <-chan remotecommand.TerminalSize, timeout time.Duration) error

	// GetPodStatus retrieves the status of a pod by name from the provider.
	GetPodStatus(namespace, name string) (*v1.PodStatus, error)

	// GetPods retrieves a list of all pods running on the provider (can be cached).
	GetPods() ([]*v1.Pod, error)

	// Capacity returns a resource list with the capacity constraints of the provider.
	Capacity() v1.ResourceList

	// NodeConditions returns a list of conditions (Ready, OutOfDisk, etc), which is
	// polled periodically to update the node status within Kubernetes.
	NodeConditions() []v1.NodeCondition

	// NodeAddresses returns a list of addresses for the node status
	// within Kubernetes.
	NodeAddresses() []v1.NodeAddress

	// NodeDaemonEndpoints returns NodeDaemonEndpoints for the node status
	// within Kubernetes.
	NodeDaemonEndpoints() *v1.NodeDaemonEndpoints

	// OperatingSystem returns the operating system the provider is for.
	OperatingSystem() string
}

// MetricsProvider is an optional interface that providers can implement to expose pod stats
type MetricsProvider interface {
	GetStatsSummary(context.Context) (*stats.Summary, error)
}

// PortForwarder is an optional interface that providers can declare if they support port forwarding
type PortForwarder interface {
	// PortForward forwards a port from virtual-kubelet to the pod
	PortForward(name string, uid types.UID, port int32, stream io.ReadWriteCloser) error
}
