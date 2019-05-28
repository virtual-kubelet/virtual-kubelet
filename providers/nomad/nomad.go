package nomad

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strings"

	nomad "github.com/hashicorp/nomad/api"
	"github.com/virtual-kubelet/virtual-kubelet/manager"
	"github.com/virtual-kubelet/virtual-kubelet/providers"
	"github.com/virtual-kubelet/virtual-kubelet/vkubelet/api"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Nomad provider constants
const (
	jobNamePrefix              = "nomad-virtual-kubelet"
	nomadDatacentersAnnotation = "nomad.hashicorp.com/datacenters"
	defaultNomadAddress        = "127.0.0.1:4646"
	defaultNomadDatacenter     = "dc1"
	defaultNomadRegion         = "global"
)

// Provider implements the virtual-kubelet provider interface and communicates with the Nomad API.
type Provider struct {
	nomadClient     *nomad.Client
	resourceManager *manager.ResourceManager
	nodeName        string
	operatingSystem string
	nomadAddress    string
	nomadRegion     string
	cpu             string
	memory          string
	pods            string
}

// NewProvider creates a new Provider
func NewProvider(rm *manager.ResourceManager, nodeName, operatingSystem string) (*Provider, error) {
	p := Provider{}
	p.resourceManager = rm
	p.nodeName = nodeName
	p.operatingSystem = operatingSystem
	p.nomadAddress = os.Getenv("NOMAD_ADDR")
	p.nomadRegion = os.Getenv("NOMAD_REGION")

	if p.nomadAddress == "" {
		p.nomadAddress = defaultNomadAddress
	}

	if p.nomadRegion == "" {
		p.nomadRegion = defaultNomadRegion
	}

	c := nomad.DefaultConfig()
	log.Printf("nomad client address: %s", p.nomadAddress)
	nomadClient, err := nomad.NewClient(c.ClientConfig(p.nomadRegion, p.nomadAddress, false))
	if err != nil {
		log.Printf("Unable to create nomad client: %s", err)
		return nil, err
	}

	p.nomadClient = nomadClient

	return &p, nil
}

// CreatePod accepts a Pod definition and creates
// a Nomad job
func (p *Provider) CreatePod(ctx context.Context, pod *v1.Pod) error {
	log.Printf("CreatePod %q\n", pod.Name)

	// Ignore daemonSet Pod
	if pod != nil && pod.OwnerReferences != nil && len(pod.OwnerReferences) != 0 && pod.OwnerReferences[0].Kind == "DaemonSet" {
		log.Printf("Skip to create DaemonSet pod %q\n", pod.Name)
		return nil
	}

	// Default datacenter name
	datacenters := []string{defaultNomadDatacenter}
	nomadDatacenters := pod.Annotations[nomadDatacentersAnnotation]
	if nomadDatacenters != "" {
		datacenters = strings.Split(nomadDatacenters, ",")
	}

	// Create a list of nomad tasks
	nomadTasks := p.createNomadTasks(pod)
	taskGroups := p.createTaskGroups(pod.Name, nomadTasks)
	job := p.createJob(pod.Name, datacenters, taskGroups)

	// Register nomad job
	_, _, err := p.nomadClient.Jobs().Register(job, nil)
	if err != nil {
		return fmt.Errorf("couldn't start nomad job: %q", err)
	}

	return nil
}

// UpdatePod is a noop, nomad does not support live updates of a pod.
func (p *Provider) UpdatePod(ctx context.Context, pod *v1.Pod) error {
	log.Println("Pod Update called: No-op as not implemented")
	return nil
}

// DeletePod accepts a Pod definition and deletes a Nomad job.
func (p *Provider) DeletePod(ctx context.Context, pod *v1.Pod) (err error) {
	// Deregister job
	response, _, err := p.nomadClient.Jobs().Deregister(pod.Name, true, nil)
	if err != nil {
		return fmt.Errorf("couldn't stop or deregister nomad job: %s: %s", response, err)
	}

	log.Printf("deregistered nomad job %q response %q\n", pod.Name, response)

	return nil
}

// GetPod returns the pod running in the Nomad cluster. returns nil
// if pod is not found.
func (p *Provider) GetPod(ctx context.Context, namespace, name string) (pod *v1.Pod, err error) {
	jobID := fmt.Sprintf("%s-%s", jobNamePrefix, name)

	// Get nomad job
	job, _, err := p.nomadClient.Jobs().Info(jobID, nil)
	if err != nil {
		return nil, fmt.Errorf("couldn't retrieve nomad job: %s", err)
	}

	// Get nomad job allocations to get individual task statuses
	jobAllocs, _, err := p.nomadClient.Jobs().Allocations(jobID, false, nil)
	if err != nil {
		return nil, fmt.Errorf("couldn't retrieve nomad job allocations: %s", err)
	}

	// Change a nomad job into a kubernetes pod
	pod, err = p.jobToPod(job, jobAllocs)
	if err != nil {
		return nil, fmt.Errorf("couldn't convert a nomad job into a pod: %s", err)
	}

	return pod, nil
}

// GetContainerLogs retrieves the logs of a container by name from the provider.
func (p *Provider) GetContainerLogs(ctx context.Context, namespace, podName, containerName string, opts api.ContainerLogOpts) (io.ReadCloser, error) {
	return ioutil.NopCloser(strings.NewReader("")), nil
}

// GetPodFullName as defined in the provider context
func (p *Provider) GetPodFullName(ctx context.Context, namespace string, pod string) string {
	return fmt.Sprintf("%s-%s", jobNamePrefix, pod)
}

// RunInContainer executes a command in a container in the pod, copying data
// between in/out/err and the container's stdin/stdout/stderr.
// TODO: Implementation
func (p *Provider) RunInContainer(ctx context.Context, namespace, name, container string, cmd []string, attach api.AttachIO) error {
	log.Printf("ExecInContainer %q\n", container)
	return nil
}

// GetPodStatus returns the status of a pod by name that is running as a job
// in the Nomad cluster returns nil if a pod by that name is not found.
func (p *Provider) GetPodStatus(ctx context.Context, namespace, name string) (*v1.PodStatus, error) {
	pod, err := p.GetPod(ctx, namespace, name)
	if err != nil {
		return nil, err
	}
	return &pod.Status, nil
}

// GetPods returns a list of all pods known to be running in Nomad nodes.
func (p *Provider) GetPods(ctx context.Context) ([]*v1.Pod, error) {
	log.Printf("GetPods\n")
	jobsList, _, err := p.nomadClient.Jobs().PrefixList(jobNamePrefix)
	if err != nil {
		return nil, fmt.Errorf("couldn't get job list from nomad: %s", err)
	}

	var pods = []*v1.Pod{}
	for _, job := range jobsList {
		// Get nomad job
		j, _, err := p.nomadClient.Jobs().Info(job.ID, nil)
		if err != nil {
			return nil, fmt.Errorf("couldn't retrieve nomad job: %s", err)
		}

		// Get nomad job allocations to get individual task statuses
		jobAllocs, _, err := p.nomadClient.Jobs().Allocations(job.ID, false, nil)
		if err != nil {
			return nil, fmt.Errorf("couldn't retrieve nomad job allocations: %s", err)
		}

		// Change a nomad job into a kubernetes pod
		pod, err := p.jobToPod(j, jobAllocs)
		if err != nil {
			return nil, fmt.Errorf("couldn't convert a nomad job into a pod: %s", err)
		}

		pods = append(pods, pod)
	}

	return pods, nil
}

// Capacity returns a resource list containing the capacity limits set for Nomad.
func (p *Provider) Capacity(ctx context.Context) v1.ResourceList {
	// TODO: Use nomad /nodes api to get a list of nodes in the cluster
	// and then use the read node /node/:node_id endpoint to calculate
	// the total resources of the cluster to report back to kubernetes.
	return v1.ResourceList{
		"cpu":    resource.MustParse("20"),
		"memory": resource.MustParse("100Gi"),
		"pods":   resource.MustParse("20"),
	}
}

// NodeConditions returns a list of conditions (Ready, OutOfDisk, etc), for updates to the node status
// within Kubernetes.
func (p *Provider) NodeConditions(ctx context.Context) []v1.NodeCondition {
	// TODO: Make these dynamic.
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
func (p *Provider) NodeAddresses(ctx context.Context) []v1.NodeAddress {
	// TODO: Use nomad api to get a list of node addresses.
	return nil
}

// NodeDaemonEndpoints returns NodeDaemonEndpoints for the node status
// within Kubernetes.
func (p *Provider) NodeDaemonEndpoints(ctx context.Context) *v1.NodeDaemonEndpoints {
	return &v1.NodeDaemonEndpoints{}
}

// OperatingSystem returns the operating system for this provider.
// This is a noop to default to Linux for now.
func (p *Provider) OperatingSystem() string {
	return providers.OperatingSystemLinux
}
