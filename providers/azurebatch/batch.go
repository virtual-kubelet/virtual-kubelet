package azurebatch

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/Azure/go-autorest/autorest/to"

	"github.com/Azure/azure-sdk-for-go/services/batch/2017-09-01.6.0/batch"
	"github.com/Azure/go-autorest/autorest"
	"github.com/golang/glog"
	"github.com/virtual-kubelet/virtual-kubelet/manager"
	"github.com/virtual-kubelet/virtual-kubelet/providers/azure/client/aci"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	stdoutFile string = "stdout.txt"
	stderrFile string = "stderr.txt"
)

// BatchProvider the base struct for the Azure Batch provider
type BatchProvider struct {
	batchConfig        *BatchConfig
	ctx                context.Context
	cancelOps          context.CancelFunc
	poolClient         *batch.PoolClient
	jobClient          *batch.JobClient
	taskClient         *batch.TaskClient
	fileClient         *batch.FileClient
	resourceManager    *manager.ResourceManager
	resourceGroup      string
	region             string
	nodeName           string
	operatingSystem    string
	cpu                string
	memory             string
	pods               string
	internalIP         string
	daemonEndpointPort int32
}

// BatchConfig - Basic azure config used to interact with ARM resources.
type BatchConfig struct {
	ClientID        string
	ClientSecret    string
	SubscriptionID  string
	TenantID        string
	ResourceGroup   string
	PoolID          string
	JobID           string
	AccountName     string
	AccountLocation string
}

// BatchPodComponents provides details for the batch task wrapper
// to run a pod
type BatchPodComponents struct {
	PullCredentials []aci.ImageRegistryCredential
	Containers      []v1.Container
	Volumes         []v1.Volume
	PodName         string
	TaskID          string
}

const batchManagementEndpoint = "https://batch.core.windows.net/"

func fixContentTypeInspector() autorest.PrepareDecorator {
	return func(p autorest.Preparer) autorest.Preparer {
		return autorest.PreparerFunc(func(r *http.Request) (*http.Request, error) {
			r.Header.Set("Content-Type", "application/json; odata=minimalmetadata")
			// dump, _ := httputil.DumpRequestOut(r, true)
			// log.Println(string(dump))
			return r, nil
		})
	}
}

// NewBatchProvider Creates a batch provider
func NewBatchProvider(config string, rm *manager.ResourceManager, nodeName, operatingSystem string, internalIP string, daemonEndpointPort int32) (*BatchProvider, error) {
	fmt.Println("Starting create provider")

	batchConfig, err := getAzureConfigFromEnv()
	if err != nil {
		log.Println("Failed to get auth information")
	}

	p := BatchProvider{}
	p.batchConfig = &batchConfig
	// Set sane defaults for Capacity in case config is not supplied
	p.cpu = "20"
	p.memory = "100Gi"
	p.pods = "20"
	p.resourceManager = rm
	p.operatingSystem = operatingSystem
	p.nodeName = nodeName
	p.internalIP = internalIP
	p.daemonEndpointPort = daemonEndpointPort
	p.ctx = context.Background()

	spt, err := newServicePrincipalTokenFromCredentials(batchConfig, batchManagementEndpoint)

	if err != nil {
		glog.Fatalf("Failed creating service principal: %v", err)
	}

	auth := autorest.NewBearerAuthorizer(spt)

	createOrGetPool(&p, auth)
	createOrGetJob(&p, auth)

	taskclient := batch.NewTaskClientWithBaseURI(getBatchBaseURL(p.batchConfig))
	taskclient.Authorizer = auth
	taskclient.RequestInspector = fixContentTypeInspector()
	p.taskClient = &taskclient

	fileClient := batch.NewFileClientWithBaseURI(getBatchBaseURL(p.batchConfig))
	fileClient.Authorizer = auth
	fileClient.RequestInspector = fixContentTypeInspector()
	p.fileClient = &fileClient

	return &p, nil
}

// CreatePod accepts a Pod definition
func (p *BatchProvider) CreatePod(pod *v1.Pod) error {
	log.Println("Creating pod...")
	podCommand, err := getPodCommand(BatchPodComponents{
		Containers: pod.Spec.Containers,
		PodName:    pod.Name,
		TaskID:     string(pod.UID),
		Volumes:    pod.Spec.Volumes,
	})
	if err != nil {
		return err
	}
	task := batch.TaskAddParameter{
		DisplayName: to.StringPtr(string(pod.UID)),
		ID:          to.StringPtr(getTaskIDForPod(pod)),
		CommandLine: to.StringPtr(fmt.Sprintf(`/bin/bash -c "%s"`, podCommand)),
		UserIdentity: &batch.UserIdentity{
			AutoUser: &batch.AutoUserSpecification{
				ElevationLevel: batch.Admin,
				Scope:          batch.Task,
			},
		},
	}
	p.taskClient.Add(p.ctx, p.batchConfig.JobID, task, nil, nil, nil, nil)

	return nil
}

// UpdatePod accepts a Pod definition
func (p *BatchProvider) UpdatePod(pod *v1.Pod) error {
	log.Println("NOOP: Pod update not supported")
	return fmt.Errorf("Failed to update pod %s as update not supported", pod.Name)
}

// DeletePod accepts a Pod definition
func (p *BatchProvider) DeletePod(pod *v1.Pod) error {
	task, err := p.taskClient.Delete(p.ctx, p.batchConfig.JobID, getTaskIDForPod(pod), nil, nil, nil, nil, "", "", nil, nil)
	if err != nil {
		log.Println(err)
		return err
	}

	log.Println(task)
	return nil
}

// GetPod returns a pod by name
func (p *BatchProvider) GetPod(namespace, name string) (*v1.Pod, error) {
	log.Println("Getting Pod ...")
	pod := p.resourceManager.GetPod(name)
	task, err := p.taskClient.Get(p.ctx, p.batchConfig.JobID, getTaskIDForPod(pod), "", "", nil, nil, nil, nil, "", "", nil, nil)
	if err != nil {
		if task.Response.StatusCode == http.StatusNotFound {
			return nil, nil
		}
		log.Println(err)
		return nil, err
	}

	jsonBytpes, _ := json.Marshal(task)
	if pod.Labels == nil {
		pod.Labels = make(map[string]string)
	}
	pod.Labels["batchStatus"] = string(jsonBytpes)
	status, _ := p.GetPodStatus(namespace, name)
	pod.Status = *status

	return pod, nil
}

// GetContainerLogs returns the logs of a container running in a pod by name.
func (p *BatchProvider) GetContainerLogs(namespace, podName, containerName string, tail int) (string, error) {
	log.Println("Getting pod logs ....")

	pod := p.resourceManager.GetPod(podName)

	logFileLocation := fmt.Sprintf("wd/%s", containerName)
	// todo: Log file is the json log from docker - deserialise and form at it before returning it.
	reader, err := p.fileClient.GetFromTask(p.ctx, p.batchConfig.JobID, getTaskIDForPod(pod), logFileLocation, nil, nil, nil, nil, "", nil, nil)

	if err != nil {
		return "", err
	}

	bytes, err := ioutil.ReadAll(*reader.Value)

	if err != nil {
		return "", err
	}

	return string(bytes), nil
}

// GetPodStatus retrieves the status of a given pod by name.
func (p *BatchProvider) GetPodStatus(namespace, name string) (*v1.PodStatus, error) {
	log.Println("Getting pod status ....")
	pod := p.resourceManager.GetPod(name)
	task, err := p.taskClient.Get(p.ctx, p.batchConfig.JobID, getTaskIDForPod(pod), "", "", nil, nil, nil, nil, "", "", nil, nil)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	startTime := metav1.Time{}
	retryCount := int32(0)
	if task.ExecutionInfo != nil {
		if task.ExecutionInfo.StartTime != nil {
			startTime.Time = task.ExecutionInfo.StartTime.Time
		}
		if task.ExecutionInfo.RetryCount != nil {
			retryCount = *task.ExecutionInfo.RetryCount
		}
	}

	// Todo: Review indivudal container status response
	return &v1.PodStatus{
		Phase:     convertTaskStatusToPodPhase(task.State),
		StartTime: &startTime,
		ContainerStatuses: []v1.ContainerStatus{
			v1.ContainerStatus{
				Name:         pod.Spec.Containers[0].Name,
				RestartCount: retryCount,
				State: v1.ContainerState{
					Running: &v1.ContainerStateRunning{
						StartedAt: startTime,
					},
				},
			},
		},
	}, nil
}

// GetPods retrieves a list of all pods scheduled to run.
func (p *BatchProvider) GetPods() ([]*v1.Pod, error) {
	log.Println("Getting pods...")
	pods := p.resourceManager.GetPods()
	for _, pod := range pods {
		status, _ := p.GetPodStatus(pod.Namespace, pod.Name)
		if status != nil {
			pod.Status = *status
		}
	}
	return pods, nil
}

// Capacity returns a resource list containing the capacity limits
func (p *BatchProvider) Capacity() v1.ResourceList {
	return v1.ResourceList{
		"cpu":    resource.MustParse(p.cpu),
		"memory": resource.MustParse(p.memory),
		"pods":   resource.MustParse(p.pods),
	}
}

// NodeConditions returns a list of conditions (Ready, OutOfDisk, etc), for updates to the node status
// within Kubernetes.
func (p *BatchProvider) NodeConditions() []v1.NodeCondition {
	// TODO: Make these dynamic and augment with custom ACI specific conditions of interest
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
func (p *BatchProvider) NodeAddresses() []v1.NodeAddress {
	// TODO: Make these dynamic and augment with custom ACI specific conditions of interest
	return []v1.NodeAddress{
		{
			Type:    "InternalIP",
			Address: p.internalIP,
		},
	}
}

// NodeDaemonEndpoints returns NodeDaemonEndpoints for the node status
// within Kubernetes.
func (p *BatchProvider) NodeDaemonEndpoints() *v1.NodeDaemonEndpoints {
	return &v1.NodeDaemonEndpoints{
		KubeletEndpoint: v1.DaemonEndpoint{
			Port: p.daemonEndpointPort,
		},
	}
}

// OperatingSystem returns the operating system for this provider.
func (p *BatchProvider) OperatingSystem() string {
	return p.operatingSystem
}
