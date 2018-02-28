package azure

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/batch/2017-09-01.6.0/batch"
	"github.com/Azure/go-autorest/autorest"
	"github.com/golang/glog"
	"github.com/google/uuid"
	"github.com/virtual-kubelet/virtual-kubelet/manager"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type BatchProvider struct {
	ctx                context.Context
	cancelOps          context.CancelFunc
	poolClient         *batch.PoolClient
	jobClient          *batch.JobClient
	taskClient         *batch.TaskClient
	resourceManager    *manager.ResourceManager
	batchPoolId        string
	batchJobId         string
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

// ARMConfig - Basic azure config used to interact with ARM resources.
type BatchConfig struct {
	ClientID        string
	ClientSecret    string
	SubscriptionID  string
	TenantID        string
	ResourceGroup   string
	PoolId          string
	AccountName     string
	AccountLocation string
}

const batchManagementEndpoint = "https://batch.core.windows.net/"

func FixContentTypeInspector() autorest.PrepareDecorator {
	return func(p autorest.Preparer) autorest.Preparer {
		return autorest.PreparerFunc(func(r *http.Request) (*http.Request, error) {
			r.Header.Set("Content-Type", "application/json; odata=minimalmetadata")
			dump, _ := httputil.DumpRequestOut(r, true)
			log.Println(string(dump))
			return r, nil
		})
	}
}

func LogResponse() autorest.RespondDecorator {
	return func(p autorest.Responder) autorest.Responder {
		return autorest.ResponderFunc(func(r *http.Response) error {
			dump, _ := httputil.DumpResponse(r, true)
			log.Println(string(dump))
			return nil
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
	p.batchPoolId = batchConfig.PoolId
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

	accountClient := batch.NewAccountClientWithBaseURI(getBatchBaseUrl(batchConfig))
	accountClient.Authorizer = auth
	res, err := accountClient.ListNodeAgentSkus(p.ctx, "", nil, nil, nil, nil, nil)
	log.Println(res.Values())
	for res.NotDone() {
		res.Next()
		log.Println(res.Values())
	}

	poolClient := batch.NewPoolClientWithBaseURI(getBatchBaseUrl(batchConfig))
	poolClient.Authorizer = auth
	poolClient.RetryAttempts = 0
	poolClient.RequestInspector = FixContentTypeInspector()
	poolClient.ResponseInspector = LogResponse()

	pool, err := poolClient.Get(p.ctx, batchConfig.PoolId, "*", "", nil, nil, nil, nil, "", "", nil, nil)

	// If we observe an error which isn't related to the pool not existing panic.
	// 404 is expected if this is first run.
	if err != nil && pool.StatusCode != 404 {
		panic(err)
	}

	if err != nil && pool.State == batch.PoolStateActive {
		log.Println("Pool active and running...")
	}

	if pool.Response.StatusCode == 404 {
		// Todo: Fixup pool create currently return error stating SKU not supported
		toCreate := batch.PoolAddParameter{
			ID: &p.batchPoolId,
			VirtualMachineConfiguration: &batch.VirtualMachineConfiguration{
				ImageReference: &batch.ImageReference{
					Publisher: StringPointer("Canonical"),
					Sku:       StringPointer("16.04-LTS"),
					Offer:     StringPointer("UbuntuServer"),
					Version:   StringPointer("latest"),
				},
				NodeAgentSKUID: StringPointer("batch.node.ubuntu 16.04"),
			},
			MaxTasksPerNode:      IntPointer(1),
			TargetDedicatedNodes: IntPointer(1),
			StartTask: &batch.StartTask{
				ResourceFiles: &[]batch.ResourceFile{
					batch.ResourceFile{
						BlobSource: StringPointer("https://raw.githubusercontent.com/Azure/batch-shipyard/b40a812d3df7df1d283cc30344ca2a69a1d97f95/contrib/packer/ubuntu-16.04-GPU%2BIB/bootstrap.sh"),
						FilePath:   StringPointer("bootstrap.sh"),
						FileMode:   StringPointer("777"),
					},
				},
				CommandLine:    StringPointer("bash -f /mnt/batch/tasks/startup/wd/bootstrap.sh 17.12.0~ce-0~ubuntu NVIDIA-Linux-x86_64-384.111.run"),
				WaitForSuccess: boolPointer(true),
				UserIdentity: &batch.UserIdentity{
					AutoUser: &batch.AutoUserSpecification{
						ElevationLevel: batch.Admin,
						Scope:          batch.Task,
					},
				},
			},
			// TaskSchedulingPolicy: &batch.TaskSchedulingPolicy{
			// 	NodeFillType: "spread",
			// },
			VMSize: StringPointer("standard_a1"),
		}
		poolCreate, err := poolClient.Add(p.ctx, toCreate, nil, nil, nil, nil)

		if err != nil {
			panic(err)
		}

		if poolCreate.StatusCode != 201 {
			panic(poolCreate)
		}

		log.Println("Pool Created")

	}

	for {
		pool, _ := poolClient.Get(p.ctx, batchConfig.PoolId, "*", "", nil, nil, nil, nil, "", "", nil, nil)

		if pool.State != "" && pool.State == batch.PoolStateActive {
			log.Println("Created pool... State is Active!")
			break
		} else {
			log.Println("Pool not created yet... sleeping")
			log.Println(pool)
			time.Sleep(time.Second * 20)
		}
	}

	jobClient := batch.NewJobClientWithBaseURI(getBatchBaseUrl(batchConfig))
	jobClient.RequestInspector = FixContentTypeInspector()
	jobClient.ResponseInspector = LogResponse()

	jobClient.Authorizer = auth
	jobID := "virtualkubeletjob3"
	p.batchJobId = jobID

	// check if job exists already
	currentJob, err := jobClient.Get(p.ctx, jobID, "", "", nil, nil, nil, nil, "", "", nil, nil)
	if err == nil && currentJob.State == batch.JobStateActive {
		log.Println("Wrapper job already exists...")
	} else {
		log.Println("Wrapper job missing... creating...")
		wrapperJob := batch.JobAddParameter{
			ID: &jobID,
			PoolInfo: &batch.PoolInformation{
				PoolID: pool.ID,
			},
		}
		// reqID := uuid.NewV4()
		res, err := jobClient.Add(p.ctx, wrapperJob, nil, nil, nil, nil)

		if err != nil {
			panic(err)
		}
		log.Println(res)
		p.jobClient = &jobClient
		p.batchJobId = jobID
	}

	taskclient := batch.NewTaskClientWithBaseURI(getBatchBaseUrl(batchConfig))
	taskclient.Authorizer = auth
	taskclient.RequestInspector = FixContentTypeInspector()
	p.taskClient = &taskclient
	taskToAdd := batch.TaskAddParameter{
		ID: StringPointer(uuid.New().String()),
		// ContainerSettings: &batch.TaskContainerSettings{
		// 	ContainerRunOptions: StringPointer("--rm"),
		// 	ImageName:           StringPointer("busybox"),
		// },
		CommandLine: StringPointer("sudo docker run busybox sleep 5"),
		UserIdentity: &batch.UserIdentity{
			AutoUser: &batch.AutoUserSpecification{
				ElevationLevel: batch.Admin,
				Scope:          batch.Task,
			},
		},
	}
	taskRes, err := taskclient.Add(p.ctx, p.batchJobId, taskToAdd, nil, nil, nil, nil)

	if err != nil {
		panic(err)
	}

	log.Println(taskRes)
	//Todo: Parse provider config string

	return &p, nil
}

// CreatePod accepts a Pod definition
func (p *BatchProvider) CreatePod(pod *v1.Pod) error {
	ips := make([]batch.ContainerRegistry, 0, len(pod.Spec.ImagePullSecrets))
	for _, ref := range pod.Spec.ImagePullSecrets {
		secret, err := p.resourceManager.GetSecret(ref.Name, pod.Namespace)
		if err != nil {
			log.Println("Failed to get secret")
		}
		if secret == nil {
			log.Println("Failed to get secret: nil")
		}
		// TODO: Check if secret type is v1.SecretTypeDockercfg and use DockerConfigKey instead of hardcoded value
		// TODO: Check if secret type is v1.SecretTypeDockerConfigJson and use DockerConfigJsonKey to determine if it's in json format
		// TODO: Return error if it's not one of these two types
		repoData, ok := secret.Data[".dockercfg"]
		if !ok {
			log.Println("no dockercfg present in secret")
		}

		var authConfigs map[string]AuthConfig
		err = json.Unmarshal(repoData, &authConfigs)
		if err != nil {
			log.Println("failed to unmarshal dockercfg")
		}

		for server, authConfig := range authConfigs {
			ips = append(ips, batch.ContainerRegistry{
				Password:       &authConfig.Password,
				RegistryServer: &server,
				UserName:       &authConfig.Username,
			})
		}
	}

	if len(ips) > 1 {
		log.Println("Pod has more than one image pull secret. Skipping all but the 1st")
	}

	if len(pod.Spec.Containers) > 1 {
		log.Println("Pod contains more than 1 container currently not supported.")
	}

	for _, container := range pod.Spec.Containers {

		task := batch.TaskAddParameter{
			DisplayName: StringPointer(string(pod.UID)),
			ID:          StringPointer(getTaskIdForPod(pod)),
			ContainerSettings: &batch.TaskContainerSettings{
				ImageName: StringPointer(container.Image),
				Registry:  &ips[0],
			},
		}
		p.taskClient.Add(p.ctx, p.batchJobId, task, nil, nil, nil, nil)

		// Todo: Look at using a task wrapper to coschedule containers in a single task.
		break
	}

	return nil
}

// UpdatePod accepts a Pod definition
func (p *BatchProvider) UpdatePod(pod *v1.Pod) error {
	log.Println("NOOP: Pod update not supported")
	return nil
}

// DeletePod accepts a Pod definition
func (p *BatchProvider) DeletePod(pod *v1.Pod) error {
	task, err := p.taskClient.Delete(p.ctx, p.batchJobId, getTaskIdForPod(pod), nil, nil, nil, nil, "", "", nil, nil)
	if err != nil {
		log.Println(err)
		panic(err)
	}

	log.Println(task)
	return nil
}

// GetPod returns a pod by name
func (p *BatchProvider) GetPod(namespace, name string) (*v1.Pod, error) {
	pod := p.resourceManager.GetPod(name)
	// task, err := p.taskClient.Get(p.ctx, p.batchJobId, getTaskIdForPod(pod), "", "", nil, nil, nil, nil, "", "", nil, nil)
	// if err != nil {
	// 	log.Println(err)
	// 	panic(err)
	// }

	return pod, nil
}

// GetContainerLogs returns the logs of a container running in a pod by name.
func (p *BatchProvider) GetContainerLogs(namespace, podName, containerName string, tail int) (string, error) {
	log.Println("not implimented")
	return "no logs yet", nil
}

// GetPodStatus retrieves the status of a given pod by name.
func (p *BatchProvider) GetPodStatus(namespace, name string) (*v1.PodStatus, error) {
	pod := p.resourceManager.GetPod(name)
	task, err := p.taskClient.Get(p.ctx, p.batchJobId, getTaskIdForPod(pod), "", "", nil, nil, nil, nil, "", "", nil, nil)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	responseBody, _ := ioutil.ReadAll(task.Response.Body)

	log.Println(responseBody)

	startTime := metav1.Time{
		Time: task.ExecutionInfo.StartTime.Time,
	}

	return &v1.PodStatus{
		Phase:     convertTaskStatusToPodPhase(task.State),
		StartTime: &startTime,
		ContainerStatuses: []v1.ContainerStatus{
			v1.ContainerStatus{
				Name:         pod.Spec.Containers[0].Name,
				Ready:        task.State == batch.TaskStateRunning,
				RestartCount: *task.ExecutionInfo.RetryCount,
			},
		},
	}, nil
}

// GetPods retrieves a list of all pods scheduled to run.
func (p *BatchProvider) GetPods() ([]*v1.Pod, error) {
	log.Println("not implimented")
	return make([]*v1.Pod, 0), nil
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
