package openstack

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/container/v1/capsules"
	"github.com/gophercloud/gophercloud/pagination"
	"github.com/virtual-kubelet/virtual-kubelet/manager"
	"github.com/virtual-kubelet/virtual-kubelet/node/api"
	"github.com/virtual-kubelet/virtual-kubelet/providers"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// ZunProvider implements the virtual-kubelet provider interface and communicates with OpenStack's Zun APIs.
type ZunProvider struct {
	ZunClient          *gophercloud.ServiceClient
	resourceManager    *manager.ResourceManager
	region             string
	nodeName           string
	operatingSystem    string
	cpu                string
	memory             string
	pods               string
	daemonEndpointPort int32
}

// NewZunProvider creates a new ZunProvider.
func NewZunProvider(config string, rm *manager.ResourceManager, nodeName string, operatingSystem string, daemonEndpointPort int32) (*ZunProvider, error) {
	var p ZunProvider
	var err error

	p.resourceManager = rm

	AuthOptions, err := openstack.AuthOptionsFromEnv()
	if err != nil {
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
	p.ZunClient.Microversion = "1.32"

	// Set sane defaults for Capacity in case config is not supplied
	p.cpu = "20"
	p.memory = "100Gi"
	p.pods = "20"

	p.operatingSystem = operatingSystem
	p.nodeName = nodeName
	p.daemonEndpointPort = daemonEndpointPort

	return &p, err
}

// GetPod returns a pod by name that is running inside Zun
// returns nil if a pod by that name is not found.
func (p *ZunProvider) GetPod(ctx context.Context, namespace, name string) (*v1.Pod, error) {
	capsule, err := capsules.Get(p.ZunClient, fmt.Sprintf("%s-%s", namespace, name)).ExtractV132()
	if err != nil {
		return nil, err
	}

	if capsule.MetaLabels["NodeName"] != p.nodeName {
		return nil, nil
	}

	return capsuleToPod(capsule)
}

// GetPods returns a list of all pods known to be running within Zun.
func (p *ZunProvider) GetPods(ctx context.Context) ([]*v1.Pod, error) {
	pager := capsules.List(p.ZunClient, nil)

	pages := 0
	err := pager.EachPage(func(page pagination.Page) (bool, error) {
		pages++
		return true, nil
	})
	if err != nil {
		return nil, err
	}

	pods := make([]*v1.Pod, 0, pages)
	err = pager.EachPage(func(page pagination.Page) (bool, error) {
		CapsuleList, err := capsules.ExtractCapsulesV132(page)
		if err != nil {
			return false, err
		}

		for _, m := range CapsuleList {
			c := m
			if m.MetaLabels["NodeName"] != p.nodeName {
				continue
			}
			p, err := capsuleToPod(&c)
			if err != nil {
				log.Println(err)
				continue
			}
			pods = append(pods, p)
		}
		return true, nil
	})
	if err != nil {
		return nil, err
	}
	return pods, nil
}

// CreatePod accepts a Pod definition and creates
// an Zun deployment
func (p *ZunProvider) CreatePod(ctx context.Context, pod *v1.Pod) error {
	var capsuleTemplate CapsuleTemplate
	capsuleTemplate.Kind = "capsule"

	podUID := string(pod.UID)
	podCreationTimestamp := pod.CreationTimestamp.String()
	var metadata Metadata
	metadata.Labels = map[string]string{
		"PodName":           pod.Name,
		"ClusterName":       pod.ClusterName,
		"NodeName":          pod.Spec.NodeName,
		"Namespace":         pod.Namespace,
		"UID":               podUID,
		"CreationTimestamp": podCreationTimestamp,
	}
	metadata.Name = pod.Namespace + "-" + pod.Name
	capsuleTemplate.Metadata = metadata
	// get containers
	containers, err := p.getContainers(ctx, pod)
	if err != nil {
		return err
	}
	capsuleTemplate.Spec.Containers = containers
	data, err := json.MarshalIndent(capsuleTemplate, "", "  ")
	if err != nil {
		return err
	}
	template := new(capsules.Template)
	template.Bin = []byte(data)
	createOpts := capsules.CreateOpts{
		TemplateOpts: template,
	}
	_, err = capsules.Create(p.ZunClient, createOpts).ExtractV132()
	if err != nil {
		return err
	}
	return err
}

func (p *ZunProvider) getContainers(ctx context.Context, pod *v1.Pod) ([]Container, error) {
	containers := make([]Container, 0, len(pod.Spec.Containers))
	for _, container := range pod.Spec.Containers {
		c := Container{
			//	Name: container.Name,
			Image:           container.Image,
			Command:         append(container.Command, container.Args...),
			WorkingDir:      container.WorkingDir,
			ImagePullPolicy: string(container.ImagePullPolicy),
		}

		// Container ENV need to sync with K8s in Zun and gophercloud. Will change them.
		// From map[string]string to []map[string]string
		c.Env = map[string]string{}
		for _, e := range container.Env {
			c.Env[e.Name] = e.Value
		}

		if container.Resources.Limits != nil {
			cpuLimit := float64(1)
			if _, ok := container.Resources.Limits[v1.ResourceCPU]; ok {
				cpuLimit = float64(container.Resources.Limits.Cpu().MilliValue()) / 1000.00
			}

			memoryLimit := 0.5
			if _, ok := container.Resources.Limits[v1.ResourceMemory]; ok {
				memoryLimit = float64(container.Resources.Limits.Memory().Value()) / 1000000000.00
			}

			c.Resources.Limits["cpu"] = cpuLimit
			c.Resources.Limits["memory"] = memoryLimit * 1024
		}

		//TODO: Add Sync with Resource requirement

		//TODO: Add port Sync

		//TODO: Add volume support
		containers = append(containers, c)
	}
	return containers, nil
}

// RunInContainer executes a command in a container in the pod, copying data
// between in/out/err and the container's stdin/stdout/stderr.
func (p *ZunProvider) RunInContainer(ctx context.Context, namespace, name, container string, cmd []string, attach api.AttachIO) error {
	log.Printf("receive ExecInContainer %q\n", container)
	return nil
}

// GetPodStatus returns the status of a pod by name that is running inside Zun
// returns nil if a pod by that name is not found.
func (p *ZunProvider) GetPodStatus(ctx context.Context, namespace, name string) (*v1.PodStatus, error) {
	pod, err := p.GetPod(ctx, namespace, name)
	if err != nil {
		return nil, err
	}

	if pod == nil {
		return nil, nil
	}

	return &pod.Status, nil
}

func (p *ZunProvider) GetContainerLogs(ctx context.Context, namespace, podName, containerName string, opts api.ContainerLogOpts) (io.ReadCloser, error) {
	return ioutil.NopCloser(strings.NewReader("not support in Zun Provider")), nil
}

// NodeConditions returns a list of conditions (Ready, OutOfDisk, etc), for updates to the node status
// within Kubernetes.
func (p *ZunProvider) NodeConditions(ctx context.Context) []v1.NodeCondition {
	// TODO: Make these dynamic and augment with custom Zun specific conditions of interest
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
func (p *ZunProvider) NodeAddresses(ctx context.Context) []v1.NodeAddress {
	return nil
}

// NodeDaemonEndpoints returns NodeDaemonEndpoints for the node status
// within Kubernetes.
func (p *ZunProvider) NodeDaemonEndpoints(ctx context.Context) *v1.NodeDaemonEndpoints {
	return &v1.NodeDaemonEndpoints{
		KubeletEndpoint: v1.DaemonEndpoint{
			Port: p.daemonEndpointPort,
		},
	}
}

// OperatingSystem returns the operating system for this provider.
// This is a noop to default to Linux for now.
func (p *ZunProvider) OperatingSystem() string {
	return providers.OperatingSystemLinux
}

func capsuleToPod(capsule *capsules.CapsuleV132) (*v1.Pod, error) {
	var podCreationTimestamp metav1.Time
	var containerStartTime metav1.Time

	podCreationTimestamp = metav1.NewTime(capsule.CreatedAt)
	if len(capsule.Containers) > 0 {
		containerStartTime = metav1.NewTime(capsule.Containers[0].StartedAt)
	}
	containerStartTime = metav1.NewTime(time.Time{})
	// Deal with container inside capsule
	containers := make([]v1.Container, 0, len(capsule.Containers))
	containerStatuses := make([]v1.ContainerStatus, 0, len(capsule.Containers))
	for _, c := range capsule.Containers {
		containerMemoryMB := 0
		if c.Memory != "" {
			containerMemory, err := strconv.Atoi(c.Memory)
			if err != nil {
				log.Println(err)
			}
			containerMemoryMB = containerMemory
		}
		container := v1.Container{
			Name:    c.Name,
			Image:   c.Image,
			Command: c.Command,
			Resources: v1.ResourceRequirements{
				Limits: v1.ResourceList{
					v1.ResourceCPU:    resource.MustParse(fmt.Sprintf("%g", float64(c.CPU))),
					v1.ResourceMemory: resource.MustParse(fmt.Sprintf("%dM", containerMemoryMB)),
				},

				Requests: v1.ResourceList{
					v1.ResourceCPU:    resource.MustParse(fmt.Sprintf("%g", float64(c.CPU*1024/100))),
					v1.ResourceMemory: resource.MustParse(fmt.Sprintf("%dM", containerMemoryMB)),
				},
			},
		}
		containers = append(containers, container)
		containerStatus := v1.ContainerStatus{
			Name:                 c.Name,
			State:                zunContainerStausToContainerStatus(&c),
			LastTerminationState: zunContainerStausToContainerStatus(&c),
			Ready:                zunStatusToPodPhase(c.Status) == v1.PodRunning,
			RestartCount:         int32(0),
			Image:                c.Image,
			ImageID:              "",
			ContainerID:          c.UUID,
		}

		// Add to containerStatuses
		containerStatuses = append(containerStatuses, containerStatus)
	}

	ip := ""
	if capsule.Addresses != nil {
		for _, v := range capsule.Addresses {
			for _, addr := range v {
				if addr.Version == float64(4) {
					ip = addr.Addr
				}
			}
		}
	}
	p := v1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:              capsule.MetaLabels["PodName"],
			Namespace:         capsule.MetaLabels["Namespace"],
			ClusterName:       capsule.MetaLabels["ClusterName"],
			UID:               types.UID(capsule.UUID),
			CreationTimestamp: podCreationTimestamp,
		},
		Spec: v1.PodSpec{
			NodeName:   capsule.MetaLabels["NodeName"],
			Volumes:    []v1.Volume{},
			Containers: containers,
		},

		Status: v1.PodStatus{
			Phase:             zunStatusToPodPhase(capsule.Status),
			Conditions:        []v1.PodCondition{},
			Message:           "",
			Reason:            "",
			HostIP:            "",
			PodIP:             ip,
			StartTime:         &containerStartTime,
			ContainerStatuses: containerStatuses,
		},
	}

	return &p, nil
}

// UpdatePod is a noop, Zun currently does not support live updates of a pod.
func (p *ZunProvider) UpdatePod(ctx context.Context, pod *v1.Pod) error {
	return nil
}

// DeletePod deletes the specified pod out of Zun.
func (p *ZunProvider) DeletePod(ctx context.Context, pod *v1.Pod) error {
	err := capsules.Delete(p.ZunClient, fmt.Sprintf("%s-%s", pod.Namespace, pod.Name)).ExtractErr()
	if err != nil {
		return err
	}

	// wait for the capsule deletion
	for i := 0; i < 300; i++ {
		time.Sleep(1 * time.Second)

		capsule, err := capsules.Get(p.ZunClient, fmt.Sprintf("%s-%s", pod.Namespace, pod.Name)).ExtractV132()
		if _, ok := err.(gophercloud.ErrDefault404); ok {
			// deletion complete
			return nil
		}

		if err != nil {
			return err
		}

		if capsule.Status == "Error" {
			return fmt.Errorf("Capsule in ERROR state")
		}
	}

	return fmt.Errorf("Timed out on waiting capsule deletion")
}

func zunContainerStausToContainerStatus(cs *capsules.Container) v1.ContainerState {
	// Zun already container start time but not add support at gophercloud
	//startTime := metav1.NewTime(time.Time(cs.StartTime))

	// Zun container status:
	//'Error', 'Running', 'Stopped', 'Paused', 'Unknown', 'Creating', 'Created',
	//'Deleted', 'Deleting', 'Rebuilding', 'Dead', 'Restarting'

	// Handle the case where the container is running.
	if cs.Status == "Running" || cs.Status == "Stopped" {
		return v1.ContainerState{
			Running: &v1.ContainerStateRunning{
				StartedAt: metav1.NewTime(time.Time(cs.StartedAt)),
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
				StartedAt:  metav1.NewTime(time.Time(cs.StartedAt)),
				FinishedAt: metav1.NewTime(time.Time(cs.UpdatedAt)),
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

// Capacity returns a resource list containing the capacity limits set for Zun.
func (p *ZunProvider) Capacity(ctx context.Context) v1.ResourceList {
	return v1.ResourceList{
		"cpu":    resource.MustParse(p.cpu),
		"memory": resource.MustParse(p.memory),
		"pods":   resource.MustParse(p.pods),
	}
}
