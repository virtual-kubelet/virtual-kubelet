package openstack

import (
	"context"
	"fmt"
	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/container/v1/containers"
	"github.com/virtual-kubelet/virtual-kubelet/manager"
	"github.com/virtual-kubelet/virtual-kubelet/providers"
	"io"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/remotecommand"
	"log"
	"os"
	"time"
)


// ZunProvider implements the virtual-kubelet provider interface and communicates with OpenStack's Zun APIs.
type ZunProvider struct {
	ZunClient *gophercloud.ServiceClient
	resourceManager *manager.ResourceManager
	region string
	nodeName string
	operatingSystem string
	cpu string
	memory string
	pods string
	daemonEndpointPort int32
}

// NewZunProvider creates a new ZunProvider.
func NewZunProvider(config string,rm *manager.ResourceManager,nodeName string,operatingSystem string,daemonEndpointPort int32)(*ZunProvider,error){
	var p ZunProvider
	var err error

	p.resourceManager = rm

	AuthOptions,err := openstack.AuthOptionsFromEnv()

	if err != nil{
		return nil, fmt.Errorf("Unable to get the Auth options from environment variables: %s", err)
	}

	Provider, err := openstack.AuthenticatedClient(AuthOptions)

	if err != nil {
		return nil, fmt.Errorf("Unable to get provider: %s", err)
	}

	p.ZunClient, err = openstack.NewContainerV1(Provider, gophercloud.EndpointOpts{
		Region: os.Getenv("OS_REGION_NAME"),
	})

	if err != nil {
		return nil, fmt.Errorf("Unable to get zun client")
	}
	// Set sane defaults for Capacity in case config is not supplied
	p.cpu = "24"
	p.memory = "64Gi"
	p.pods = "20"
	p.operatingSystem = operatingSystem
	p.nodeName = nodeName
	p.daemonEndpointPort = daemonEndpointPort
	p.ZunClient.Microversion = "1.9"
	return &p, err
}

// CreatePod takes a Kubernetes Pod and deploys it within the provider.
func (p *ZunProvider)CreatePod(ctx context.Context, pod *v1.Pod) error{
	container,err := p.getContainer(pod)
	if err != nil{
		return err
	}

	createOpts := containers.CreateOpts{
		TemplateOpts:container,
	}
	_, err = containers.Create(p.ZunClient, createOpts).Extract()


	if err != nil {
		return err
	}

	return err
}

func (p *ZunProvider) getContainer(pod *v1.Pod) (Container, error) {
	containers := make([]Container, 0, len(pod.Spec.Containers))
	for _, container := range pod.Spec.Containers {
		c := Container{
			//Nets:             []network{
			//	network{
			//		"network":"container-netword",
			//	},
			//},
			CPU:              2,
			Memory:           2048,
			Image:            container.Image,
			Labels:           pod.Labels,
			Name:             container.Name,
			AutoRemove:       false,
			WorkDir:          container.WorkingDir,
			ImagePullPolicy:  "always",
			HostName:         "master",
			Environment:      nil,
			ImageDriver:      "docker",
			Command:          container.Command,
			Runtime:          "runc",
			Interactive:      false,
			RestartPolicy:    nil,
			SecurityGroups:   []string{"default"},
			AvailabilityZone: "nova",
		}
		// Container ENV need to sync with K8s in Zun and gophercloud. Will change them.
		// From map[string]string to []map[string]string
		c.Environment = map[string]string{}
		for _, e := range container.Env {
			c.Environment[e.Name] = e.Value
		}
		//if container.Resources.Limits != nil {
		//	cpuLimit := float64(1)
		//	if _, ok := container.Resources.Limits[v1.ResourceCPU]; ok {
		//		cpuLimit = float64(container.Resources.Limits.Cpu().MilliValue()) / 1000.00
		//	}
		//	memoryLimit := 0.5
		//	if _, ok := container.Resources.Limits[v1.ResourceMemory]; ok {
		//		memoryLimit = float64(container.Resources.Limits.Memory().Value()) / 1000000000.00
		//	}
		//	c.Resources.Limits["cpu"] = cpuLimit
		//	c.Resources.Limits["memory"] = memoryLimit*1024
		//}
		//TODO: Add Sync with Resource requirement
		//TODO: Add port Sync
		//TODO: Add volume support
		containers = append(containers, c)
	}
	return containers[0], nil
}

// UpdatePod takes a Kubernetes Pod and updates it within the provider.
func (p *ZunProvider) UpdatePod(ctx context.Context, pod *v1.Pod) error{
	var err error
	return err
}

// DeletePod takes a Kubernetes Pod and deletes it from the provider.
func (p *ZunProvider) DeletePod(ctx context.Context, pod *v1.Pod) error{
	var err error
	return err
}

// GetPod retrieves a pod by name from the provider (can be cached).
func (p *ZunProvider) GetPod(ctx context.Context, namespace, name string) (*v1.Pod, error){
	var err error
	return nil,err
}

// GetContainerLogs retrieves the logs of a container by name from the provider.
func (p *ZunProvider) GetContainerLogs(ctx context.Context, namespace, podName, containerName string, tail int) (string, error){
	var err error
	return "",err
}

// ExecInContainer executes a command in a container in the pod, copying data
// between in/out/err and the container's stdin/stdout/stderr.
func (p *ZunProvider) ExecInContainer(name string, uid types.UID, container string, cmd []string, in io.Reader, out, err io.WriteCloser, tty bool, resize <-chan remotecommand.TerminalSize, timeout time.Duration) error{
	var e error
	return e
}

// GetPodStatus retrieves the status of a pod by name from the provider.
func (p *ZunProvider) GetPodStatus(ctx context.Context, namespace, name string) (*v1.PodStatus, error){
	var err error
	return nil,err
}

// GetPods retrieves a list of all pods running on the provider (can be cached).
func (p *ZunProvider) GetPods(context.Context) ([]*v1.Pod, error){
	var err error
	return nil,err
}

// Capacity returns a resource list with the capacity constraints of the provider.
func (p *ZunProvider) Capacity(context.Context) v1.ResourceList{
	return v1.ResourceList{
		"cpu" : resource.MustParse(p.cpu),
		"memory": resource.MustParse(p.memory),
		"pods":   resource.MustParse(p.pods),
	}
}

// NodeConditions returns a list of conditions (Ready, OutOfDisk, etc), which is
// polled periodically to update the node status within Kubernetes.
func (p *ZunProvider) NodeConditions(context.Context) []v1.NodeCondition{
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
func (p *ZunProvider) NodeAddresses(context.Context) []v1.NodeAddress{
	log.Println("Received NodeAddresses request.")
	return []v1.NodeAddress{
		{
			Type:    v1.NodeInternalIP,
			Address: "10.10.87.13",
		},
	}
}

// NodeDaemonEndpoints returns NodeDaemonEndpoints for the node status
// within Kubernetes.
func (p *ZunProvider) NodeDaemonEndpoints(context.Context) *v1.NodeDaemonEndpoints{
	return &v1.NodeDaemonEndpoints{
		KubeletEndpoint: v1.DaemonEndpoint{
			Port: p.daemonEndpointPort,
		},
	}
}

// OperatingSystem returns the operating system the provider is for.
func (p *ZunProvider) OperatingSystem() string{
	if p.operatingSystem != ""{
		return p.operatingSystem
	}
	return providers.OperatingSystemLinux
}

func zunContainerStausToContainerStatus(cs *containers.Container) v1.ContainerState {
	// Zun already container start time but not add support at gophercloud
	//startTime := metav1.NewTime(time.Time(cs.StartTime))

	// Zun container status:
	//'Error', 'Running', 'Stopped', 'Paused', 'Unknown', 'Creating', 'Created',
	//'Deleted', 'Deleting', 'Rebuilding', 'Dead', 'Restarting'

	// Handle the case where the container is running.
	if cs.Status == "Running" || cs.Status == "Stopped"{
		return v1.ContainerState{
			Running: &v1.ContainerStateRunning{
				StartedAt: metav1.NewTime(time.Time(time.Now())),
			},
		}
	}

	// Handle the case where the container failed.
	if cs.Status == "Error" || cs.Status == "Dead" {
		return v1.ContainerState{
			Terminated: &v1.ContainerStateTerminated{
				ExitCode:   int32(0),
				Reason:     cs.Status,
				Message:    cs.StatusDetail,
				StartedAt:  metav1.NewTime(time.Time(time.Now())),
				FinishedAt: metav1.NewTime(time.Time(time.Now())),
			},
		}
	}

	// Handle the case where the container is pending.
	// Which should be all other Zun states.
	return v1.ContainerState{
		Waiting: &v1.ContainerStateWaiting{
			Reason:  cs.Status,
			Message: cs.StatusDetail,
		},
	}
}


func zunStatusToPodPhase(status string) v1.PodPhase {
	switch status {
	case "Running":
		return v1.PodRunning
	case "Stopped":
		return v1.PodSucceeded
	case "Error":
		return v1.PodFailed
	case "Dead":
		return v1.PodFailed
	case "Creating":
		return v1.PodPending
	case "Created":
		return v1.PodPending
	case "Restarting":
		return v1.PodPending
	case "Rebuilding":
		return v1.PodPending
	case "Paused":
		return v1.PodPending
	case "Deleting":
		return v1.PodPending
	case "Deleted":
		return v1.PodPending
	}

	return v1.PodUnknown
}

func zunContainerStatusToPodPhase(status string) v1.PodPhase {
	switch status {
	case "Running":
		return v1.PodRunning
	case "Succeeded":
		return v1.PodSucceeded
	case "Failed":
		return v1.PodFailed
	case "Pending":
		return v1.PodPending
	}

	return v1.PodUnknown
}