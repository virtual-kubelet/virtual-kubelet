package azurebatch

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Azure/go-autorest/autorest"
	"os"
	"strings"

	"io/ioutil"
	"log"
	"net/http"

	"github.com/Azure/go-autorest/autorest/azure"

	"github.com/Azure/azure-sdk-for-go/services/batch/2017-09-01.6.0/batch"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/lawrencegripper/pod2docker"
	"github.com/virtual-kubelet/virtual-kubelet/manager"
	azureCreds "github.com/virtual-kubelet/virtual-kubelet/providers/azure"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	podJSONKey string = "virtualkubelet_pod"
)

// Provider the base struct for the Azure Batch provider
type Provider struct {
	batchConfig        *Config
	ctx                context.Context
	cancelCtx          context.CancelFunc
	fileClient         *batch.FileClient
	resourceManager    *manager.ResourceManager
	listTasks          func() (*[]batch.CloudTask, error)
	addTask            func(batch.TaskAddParameter) (autorest.Response, error)
	getTask            func(taskID string) (batch.CloudTask, error)
	deleteTask         func(taskID string) (autorest.Response, error)
	getFileFromTask    func(taskID, path string) (result batch.ReadCloser, err error)
	nodeName           string
	operatingSystem    string
	cpu                string
	memory             string
	pods               string
	internalIP         string
	daemonEndpointPort int32
}

// Config - Basic azure config used to interact with ARM resources.
type Config struct {
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

// NewBatchProvider Creates a batch provider
func NewBatchProvider(configString string, rm *manager.ResourceManager, nodeName, operatingSystem string, internalIP string, daemonEndpointPort int32) (*Provider, error) {
	fmt.Println("Starting create provider")

	config := &Config{}
	if azureCredsFilepath := os.Getenv("AZURE_CREDENTIALS_LOCATION"); azureCredsFilepath != "" {
		creds, err := azureCreds.NewAcsCredential(azureCredsFilepath)
		if err != nil {
			return nil, err
		}
		config.ClientID = creds.ClientID
		config.ClientSecret = creds.ClientSecret
		config.SubscriptionID = creds.SubscriptionID
		config.TenantID = creds.TenantID
	}

	err := getAzureConfigFromEnv(config)
	if err != nil {
		log.Println("Failed to get auth information")
	}

	return NewBatchProviderFromConfig(config, rm, nodeName, operatingSystem, internalIP, daemonEndpointPort)
}

// NewBatchProviderFromConfig Creates a batch provider
func NewBatchProviderFromConfig(config *Config, rm *manager.ResourceManager, nodeName, operatingSystem string, internalIP string, daemonEndpointPort int32) (*Provider, error) {
	p := Provider{}
	p.batchConfig = config
	// Set sane defaults for Capacity in case config is not supplied
	p.cpu = "20"
	p.memory = "100Gi"
	p.pods = "20"
	p.resourceManager = rm
	p.operatingSystem = operatingSystem
	p.nodeName = nodeName
	p.internalIP = internalIP
	p.daemonEndpointPort = daemonEndpointPort
	p.ctx, p.cancelCtx = context.WithCancel(context.Background())

	auth := getAzureADAuthorizer(config, azure.PublicCloud.BatchManagementEndpoint)

	batchBaseURL := getBatchBaseURL(config.AccountName, config.AccountLocation)
	_, err := getPool(p.ctx, batchBaseURL, config.PoolID, auth)
	if err != nil {
		log.Panicf("Error retreiving Azure Batch pool: %v", err)
	}
	_, err = createOrGetJob(p.ctx, batchBaseURL, config.JobID, config.PoolID, auth)
	if err != nil {
		log.Panicf("Error retreiving/creating Azure Batch job: %v", err)
	}
	taskClient := batch.NewTaskClientWithBaseURI(batchBaseURL)
	taskClient.Authorizer = auth
	p.listTasks = func() (*[]batch.CloudTask, error) {
		res, err := taskClient.List(p.ctx, config.JobID, "", "", "", nil, nil, nil, nil, nil)
		if err != nil {
			return &[]batch.CloudTask{}, err
		}
		currentTasks := res.Values()
		for res.NotDone() {
			err = res.Next()
			if err != nil {
				return &[]batch.CloudTask{}, err
			}
			pageTasks := res.Values()
			if pageTasks != nil || len(pageTasks) != 0 {
				currentTasks = append(currentTasks, pageTasks...)
			}
		}

		return &currentTasks, nil
	}
	p.addTask = func(task batch.TaskAddParameter) (autorest.Response, error) {
		return taskClient.Add(p.ctx, config.JobID, task, nil, nil, nil, nil)
	}
	p.getTask = func(taskID string) (batch.CloudTask, error) {
		return taskClient.Get(p.ctx, config.JobID, taskID, "", "", nil, nil, nil, nil, "", "", nil, nil)
	}
	p.deleteTask = func(taskID string) (autorest.Response, error) {
		return taskClient.Delete(p.ctx, config.JobID, taskID, nil, nil, nil, nil, "", "", nil, nil)
	}
	p.getFileFromTask = func(taskID, path string) (batch.ReadCloser, error) {
		return p.fileClient.GetFromTask(p.ctx, config.JobID, taskID, path, nil, nil, nil, nil, "", nil, nil)
	}

	fileClient := batch.NewFileClientWithBaseURI(batchBaseURL)
	fileClient.Authorizer = auth
	p.fileClient = &fileClient

	return &p, nil
}

// CreatePod accepts a Pod definition
func (p *Provider) CreatePod(pod *v1.Pod) error {
	log.Println("Creating pod...")
	podCommand, err := pod2docker.GetBashCommand(pod2docker.PodComponents{
		InitContainers: pod.Spec.InitContainers,
		Containers:     pod.Spec.Containers,
		PodName:        pod.Name,
		Volumes:        pod.Spec.Volumes,
	})
	if err != nil {
		return err
	}

	bytes, err := json.Marshal(pod)
	if err != nil {
		panic(err)
	}

	task := batch.TaskAddParameter{
		DisplayName: to.StringPtr(string(pod.UID)),
		ID:          to.StringPtr(getTaskIDForPod(pod.Namespace, pod.Name)),
		CommandLine: to.StringPtr(fmt.Sprintf(`/bin/bash -c "%s"`, podCommand)),
		UserIdentity: &batch.UserIdentity{
			AutoUser: &batch.AutoUserSpecification{
				ElevationLevel: batch.Admin,
				Scope:          batch.Pool,
			},
		},
		EnvironmentSettings: &[]batch.EnvironmentSetting{
			{
				Name:  to.StringPtr(podJSONKey),
				Value: to.StringPtr(string(bytes)),
			},
		},
	}
	_, err = p.addTask(task)
	if err != nil {
		return err
	}

	return nil
}

// GetPodStatus retrieves the status of a given pod by name.
func (p *Provider) GetPodStatus(namespace, name string) (*v1.PodStatus, error) {
	log.Println("Getting pod status ....")
	pod, err := p.GetPod(namespace, name)

	if err != nil {
		return nil, err
	}
	if pod == nil {
		return nil, nil
	}
	return &pod.Status, nil
}

// UpdatePod accepts a Pod definition
func (p *Provider) UpdatePod(pod *v1.Pod) error {
	log.Println("Pod Update called: No-op as not implemented")
	return nil
}

// DeletePod accepts a Pod definition
func (p *Provider) DeletePod(pod *v1.Pod) error {
	taskID := getTaskIDForPod(pod.Namespace, pod.Name)
	task, err := p.deleteTask(taskID)
	if err != nil {
		log.Println(task)
		log.Println(err)
		return err
	}

	log.Printf(fmt.Sprintf("Deleting task: %v", taskID))
	return nil
}

// GetPod returns a pod by name
func (p *Provider) GetPod(namespace, name string) (*v1.Pod, error) {
	log.Println("Getting Pod ...")
	task, err := p.getTask(getTaskIDForPod(namespace, name))
	if err != nil {
		if task.Response.StatusCode == http.StatusNotFound {
			return nil, nil
		}
		log.Println(err)
		return nil, err
	}

	pod, err := getPodFromTask(&task)
	if err != nil {
		panic(err)
	}

	status, _ := convertTaskToPodStatus(&task)
	pod.Status = *status

	return pod, nil
}

// GetContainerLogs returns the logs of a container running in a pod by name.
func (p *Provider) GetContainerLogs(namespace, podName, containerName string, tail int) (string, error) {
	log.Println("Getting pod logs ....")

	taskID := getTaskIDForPod(namespace, podName)
	logFileLocation := fmt.Sprintf("wd/%s.log", containerName)
	containerLogReader, err := p.getFileFromTask(taskID, logFileLocation)

	if containerLogReader.Response.Response != nil && containerLogReader.StatusCode == http.StatusNotFound {
		stdoutReader, err := p.getFileFromTask(taskID, "stdout.txt")
		if err != nil {
			return "", err
		}
		stderrReader, err := p.getFileFromTask(taskID, "stderr.txt")
		if err != nil {
			return "", err
		}

		var builder strings.Builder
		builderPtr := &builder
		mustWriteString(builderPtr, "Container still starting....\n")
		mustWriteString(builderPtr, "Showing startup logs from Azure Batch node instead:\n")
		mustWriteString(builderPtr, "----- STDOUT -----\n")
		stdoutBytes, _ := ioutil.ReadAll(*stdoutReader.Value)
		mustWrite(builderPtr, stdoutBytes)
		mustWriteString(builderPtr, "\n")

		mustWriteString(builderPtr, "----- STDERR -----\n")
		stderrBytes, _ := ioutil.ReadAll(*stderrReader.Value)
		mustWrite(builderPtr, stderrBytes)
		mustWriteString(builderPtr, "\n")

		return builder.String(), nil
	}

	if err != nil {
		return "", err
	}

	result, err := formatLogJSON(containerLogReader)
	if err != nil {
		return "", fmt.Errorf("Container log formating failed err: %v", err)
	}

	return result, nil
}

// GetPods retrieves a list of all pods scheduled to run.
func (p *Provider) GetPods() ([]*v1.Pod, error) {
	log.Println("Getting pods...")
	tasksPtr, err := p.listTasks()
	if err != nil {
		panic(err)
	}
	if tasksPtr == nil {
		return []*v1.Pod{}, nil
	}

	tasks := *tasksPtr

	pods := make([]*v1.Pod, len(tasks), len(tasks))
	for i, t := range tasks {
		pod, err := getPodFromTask(&t)
		if err != nil {
			panic(err)
		}
		pods[i] = pod
	}
	return pods, nil
}

// Capacity returns a resource list containing the capacity limits
func (p *Provider) Capacity() v1.ResourceList {
	return v1.ResourceList{
		"cpu":            resource.MustParse(p.cpu),
		"memory":         resource.MustParse(p.memory),
		"pods":           resource.MustParse(p.pods),
		"nvidia.com/gpu": resource.MustParse("1"),
	}
}

// NodeConditions returns a list of conditions (Ready, OutOfDisk, etc), for updates to the node status
// within Kubernetes.
func (p *Provider) NodeConditions() []v1.NodeCondition {
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
func (p *Provider) NodeAddresses() []v1.NodeAddress {
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
func (p *Provider) NodeDaemonEndpoints() *v1.NodeDaemonEndpoints {
	return &v1.NodeDaemonEndpoints{
		KubeletEndpoint: v1.DaemonEndpoint{
			Port: p.daemonEndpointPort,
		},
	}
}

// OperatingSystem returns the operating system for this provider.
func (p *Provider) OperatingSystem() string {
	return p.operatingSystem
}
