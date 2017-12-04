package hypersh

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/docker/go-connections/nat"
	hyper "github.com/hyperhq/hyper-api/client"
	"github.com/hyperhq/hyper-api/types"
	"github.com/hyperhq/hyper-api/types/container"
	"github.com/hyperhq/hyper-api/types/filters"
	"github.com/hyperhq/hyper-api/types/network"
	"github.com/virtual-kubelet/virtual-kubelet/manager"
	"github.com/virtual-kubelet/virtual-kubelet/providers"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	host           = "https://us-west-1.hyper.sh"
	verStr         = "v1.23"
	containerLabel = "hyper-virtual-kubelet"
	nodeLabel      = containerLabel + "-node"
)

// HyperProvider implements the virtual-kubelet provider interface and communicates with hyper.sh APIs.
type HyperProvider struct {
	hyperClient     *hyper.Client
	resourceManager *manager.ResourceManager
	nodeName        string
	operatingSystem string
	region          string
	accessKey       string
	secretKey       string
	cpu             string
	memory          string
	pods            string
}

// NewHyperProvider creates a new HyperProvider
func NewHyperProvider(config string, rm *manager.ResourceManager, nodeName, operatingSystem string) (*HyperProvider, error) {
	var p HyperProvider
	var err error

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

	if ak := os.Getenv("HYPERSH_ACCESS_KEY"); ak != "" {
		p.accessKey = ak
	}

	if sk := os.Getenv("HYPERSH_SECRET_KEY"); sk != "" {
		p.secretKey = sk
	}

	if r := os.Getenv("HYPERSH_REGION"); r != "" {
		p.region = r
	}

	p.operatingSystem = operatingSystem
	p.nodeName = nodeName

	p.hyperClient, err = hyper.NewClient(host, verStr, http.DefaultClient, nil, p.accessKey, p.secretKey, p.region)
	if err != nil {
		return nil, err
	}

	return &p, nil
}

// CreatePod accepts a Pod definition and creates
// a hyper.sh deployment
func (p *HyperProvider) CreatePod(pod *v1.Pod) error {

	// get containers
	containers, hostConfigs, err := getContainers(pod)
	if err != nil {
		return err
	}
	// TODO: get registry creds
	// TODO: get volumes

	// Iterate over the containers to create and start them.
	for k, ctr := range containers {
		containerName := fmt.Sprintf("pod-%s-%s", pod.Name, pod.Spec.Containers[k].Name)
		// Add labels to the pod containers.
		ctr.Labels = map[string]string{
			containerLabel: pod.Name,
			nodeLabel:      p.nodeName,
		}

		// Create the container.
		resp, err := p.hyperClient.ContainerCreate(context.Background(), &ctr, &hostConfigs[k], &network.NetworkingConfig{}, containerName)
		if err != nil {
			return fmt.Errorf("Creating container %q failed in pod %q: %v", containerName, pod.Name, err)
		}
		// Iterate throught the warnings.
		for _, warning := range resp.Warnings {
			log.Printf("Warning while creating container %q for pod %q: %s", containerName, pod.Name, warning)
		}

		// Start the container.
		if err := p.hyperClient.ContainerStart(context.Background(), resp.ID, ""); err != nil {
			return fmt.Errorf("Starting container %q failed in pod %q: %v", containerName, pod.Name, err)
		}
	}

	return nil
}

// UpdatePod is a noop, hyper.sh currently does not support live updates of a pod.
func (p *HyperProvider) UpdatePod(pod *v1.Pod) error {
	return nil
}

// DeletePod deletes the specified pod out of hyper.sh.
func (p *HyperProvider) DeletePod(pod *v1.Pod) error {
	return nil
}

// GetPod returns a pod by name that is running inside hyper.sh
// returns nil if a pod by that name is not found.
func (p *HyperProvider) GetPod(name string) (*v1.Pod, error) {
	return nil, nil
}

// GetPodStatus returns the status of a pod by name that is running inside hyper.sh
// returns nil if a pod by that name is not found.
func (p *HyperProvider) GetPodStatus(name string) (*v1.PodStatus, error) {
	return nil, nil
}

// GetPods returns a list of all pods known to be running within hyper.sh.
func (p *HyperProvider) GetPods() ([]*v1.Pod, error) {
	filter, err := filters.FromParam(nodeLabel + "=" + p.nodeName)
	if err != nil {
		return nil, fmt.Errorf("Creating filter to get containers by node name failed: %v", err)
	}
	// Filter by label.
	_, err = p.hyperClient.ContainerList(context.Background(), types.ContainerListOptions{
		Filter: filter,
		All:    true,
	})
	if err != nil {
		return nil, fmt.Errorf("Listing containers failed: %v", err)
	}

	// TODO: convert containers into pods

	return nil, nil
}

// Capacity returns a resource list containing the capacity limits set for hyper.sh.
func (p *HyperProvider) Capacity() v1.ResourceList {
	// TODO: These should be configurable
	return v1.ResourceList{
		"cpu":    resource.MustParse("20"),
		"memory": resource.MustParse("100Gi"),
		"pods":   resource.MustParse("20"),
	}
}

// NodeConditions returns a list of conditions (Ready, OutOfDisk, etc), for updates to the node status
// within Kuberentes.
func (p *HyperProvider) NodeConditions() []v1.NodeCondition {
	// TODO: Make these dynamic and augment with custom hyper.sh specific conditions of interest
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

// OperatingSystem returns the operating system for this provider.
// This is a noop to default to Linux for now.
func (p *HyperProvider) OperatingSystem() string {
	return providers.OperatingSystemLinux
}

func getContainers(pod *v1.Pod) ([]container.Config, []container.HostConfig, error) {
	containers := make([]container.Config, len(pod.Spec.Containers))
	hostConfigs := make([]container.HostConfig, len(pod.Spec.Containers))
	for x, ctr := range pod.Spec.Containers {
		// Do container.Config
		var c container.Config
		c.Image = ctr.Image
		c.Cmd = ctr.Command
		ports := map[nat.Port]struct{}{}
		portBindings := nat.PortMap{}
		for _, p := range ctr.Ports {
			port, err := nat.NewPort(string(p.Protocol), fmt.Sprintf("%d", p.HostPort))
			if err != nil {
				return nil, nil, fmt.Errorf("creating new port in container conversion failed: %v", err)
			}
			ports[port] = struct{}{}

			portBindings[port] = []nat.PortBinding{
				{
					HostPort: fmt.Sprintf("%d", p.HostPort),
				},
			}
		}
		c.ExposedPorts = ports

		// TODO: do volumes

		envs := make([]string, len(ctr.Env))
		for z, e := range ctr.Env {
			envs[z] = fmt.Sprintf("%s=%s", e.Name, e.Value)
		}
		c.Env = envs

		// Do container.HostConfig
		var hc container.HostConfig
		cpuLimit := ctr.Resources.Limits.Cpu().Value()
		memoryLimit := ctr.Resources.Limits.Memory().Value()

		hc.Resources = container.Resources{
			CPUShares: cpuLimit,
			Memory:    memoryLimit,
		}

		hc.PortBindings = portBindings

		containers[x] = c
		hostConfigs[x] = hc
	}
	return containers, hostConfigs, nil
}
