package mock

import (
	"log"
	"time"

	"github.com/virtual-kubelet/virtual-kubelet/providers"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MockProvider implements the virtual-kubelet provider interface and stores pods in memory.
type MockProvider struct {
	nodeName           string
	operatingSystem    string
	daemonEndpointPort int32
	pods               []*v1.Pod
}

// NewMockProvider creates a new MockProvider
func NewMockProvider(nodeName, operatingSystem string, daemonEndpointPort int32) (*MockProvider, error) {
	provider := MockProvider{
		nodeName:           nodeName,
		operatingSystem:    operatingSystem,
		daemonEndpointPort: daemonEndpointPort,
	}

	return &provider, nil
}

// CreatePod accepts a Pod definition and stores it in memory.
func (p *MockProvider) CreatePod(pod *v1.Pod) error {
	log.Printf("receive CreatePod %q\n", pod.Name)
	p.pods = append(p.pods, pod)
	return nil
}

// UpdatePod accepts a Pod definition and updates its reference.
func (p *MockProvider) UpdatePod(update *v1.Pod) error {
	for delta, pod := range p.pods {
		if pod.Namespace == update.Namespace && pod.Name == update.Name {
			p.pods[delta] = update
		}
	}

	return nil
}

// DeletePod deletes the specified pod out of memory.
func (p *MockProvider) DeletePod(pod *v1.Pod) (err error) {
	log.Printf("receive DeletePod %q\n", pod.Name)
	// @todo, Remove from pods.
	return nil
}

// GetPod returns a pod by name that is stored in memory.
func (p *MockProvider) GetPod(namespace, name string) (pod *v1.Pod, err error) {
	for _, pod := range p.pods {
		if pod.Namespace == namespace && pod.Name == name {
			return pod, nil
		}
	}

	return nil, nil
}

// GetContainerLogs retrieves the logs of a container by name from the provider.
func (p *MockProvider) GetContainerLogs(namespace, podName, containerName string, tail int) (string, error) {
	return "", nil
}

// GetPodStatus returns the status of a pod by name that is "running".
// returns nil if a pod by that name is not found.
func (p *MockProvider) GetPodStatus(namespace, name string) (*v1.PodStatus, error) {
	now := metav1.NewTime(time.Now())

	status := &v1.PodStatus{
		Phase:     v1.PodRunning,
		HostIP:    "1.2.3.4",
		PodIP:     "5.6.7.8",
		StartTime: &now,
		Conditions: []v1.PodCondition{
			{
				Type:   v1.PodInitialized,
				Status: v1.ConditionTrue,
			},
			{
				Type:   v1.PodReady,
				Status: v1.ConditionTrue,
			},
			{
				Type:   v1.PodScheduled,
				Status: v1.ConditionTrue,
			},
		},
	}

	pod, err := p.GetPod(namespace, name)
	if err != nil {
		return status, err
	}

	for _, container := range pod.Spec.Containers {
		status.ContainerStatuses = append(status.ContainerStatuses, v1.ContainerStatus{
			Name:         container.Name,
			Image:        container.Image,
			Ready:        true,
			RestartCount: 0,
			State: v1.ContainerState{
				Running: &v1.ContainerStateRunning{
					StartedAt: now,
				},
			},
		})
	}

	return status, nil
}

// GetPods returns a list of all pods known to be "running".
func (p *MockProvider) GetPods() ([]*v1.Pod, error) {
	log.Printf("receive GetPods\n")
	return p.pods, nil
}

// Capacity returns a resource list containing the capacity limits.
func (p *MockProvider) Capacity() v1.ResourceList {
	// TODO: These should be configurable
	return v1.ResourceList{
		"cpu":    resource.MustParse("20"),
		"memory": resource.MustParse("100Gi"),
		"pods":   resource.MustParse("20"),
	}
}

// NodeConditions returns a list of conditions (Ready, OutOfDisk, etc), for updates to the node status
// within Kubernetes.
func (p *MockProvider) NodeConditions() []v1.NodeCondition {
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
func (p *MockProvider) NodeAddresses() []v1.NodeAddress {
	return nil
}

// NodeDaemonEndpoints returns NodeDaemonEndpoints for the node status
// within Kubernetes.
func (p *MockProvider) NodeDaemonEndpoints() *v1.NodeDaemonEndpoints {
	return &v1.NodeDaemonEndpoints{
		KubeletEndpoint: v1.DaemonEndpoint{
			Port: p.daemonEndpointPort,
		},
	}
}

// OperatingSystem returns the operating system for this provider.
// This is a noop to default to Linux for now.
func (p *MockProvider) OperatingSystem() string {
	return providers.OperatingSystemLinux
}
