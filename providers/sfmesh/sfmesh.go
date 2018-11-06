package sfmesh

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/Azure/azure-sdk-for-go/profiles/preview/preview/servicefabricmesh/mgmt/servicefabricmesh"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/virtual-kubelet/virtual-kubelet/manager"
	"github.com/virtual-kubelet/virtual-kubelet/providers"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/remotecommand"
)

const (
	defaultCPUCapacity    = "60"
	defaultMemoryCapacity = "48Gi"
	defaultPodCapacity    = "5"
	defaultCPURequests    = 1.0
	defaultMemoryRequests = 1.0
	defaultCPULimit       = 4.0
	defaultMemoryLimit    = 16.0
)

// SFMeshProvider implements the Virtual Kubelet provider interface
type SFMeshProvider struct {
	nodeName           string
	operatingSystem    string
	internalIP         string
	daemonEndpointPort int32
	appClient          *servicefabricmesh.ApplicationClient
	networkClient      *servicefabricmesh.NetworkClient
	serviceClient      *servicefabricmesh.ServiceClient
	region             string
	resourceGroup      string
	subscriptionID     string
	resourceManager    *manager.ResourceManager
}

// AuthConfig is the secret returned from an ImageRegistryCredential
type AuthConfig struct {
	Username      string `json:"username,omitempty"`
	Password      string `json:"password,omitempty"`
	Auth          string `json:"auth,omitempty"`
	Email         string `json:"email,omitempty"`
	ServerAddress string `json:"serveraddress,omitempty"`
	IdentityToken string `json:"identitytoken,omitempty"`
	RegistryToken string `json:"registrytoken,omitempty"`
}

// NewSFMeshProvider creates a new SFMeshProvider
func NewSFMeshProvider(rm *manager.ResourceManager, nodeName, operatingSystem string, internalIP string, daemonEndpointPort int32) (*SFMeshProvider, error) {
	azureSubscriptionID := os.Getenv("AZURE_SUBSCRIPTION_ID")
	azureTenantID := os.Getenv("AZURE_TENANT_ID")
	azureClientID := os.Getenv("AZURE_CLIENT_ID")
	azureClientSecret := os.Getenv("AZURE_CLIENT_SECRET")
	region := os.Getenv("REGION")
	resourceGroup := os.Getenv("RESOURCE_GROUP")

	if azureSubscriptionID == "" {
		return nil, errors.New("Subscription ID cannot be empty, please set AZURE_SUBSCRIPTION_ID")
	}
	if azureTenantID == "" {
		return nil, errors.New("Tenant ID cannot be empty, please set AZURE_TENANT_ID")
	}
	if azureClientID == "" {
		return nil, errors.New("Client ID cannot be empty, please set AZURE_CLIENT_ID ")
	}
	if azureClientSecret == "" {
		return nil, errors.New("Client Secret cannot be empty, please set AZURE_CLIENT_SECRET ")
	}
	if region == "" {
		return nil, errors.New("Region cannot be empty, please set REGION ")
	}
	if resourceGroup == "" {
		return nil, errors.New("Resource Group cannot be empty, please set RESOURCE_GROUP ")
	}

	client := servicefabricmesh.NewApplicationClient(azureSubscriptionID)

	auth, err := auth.NewAuthorizerFromEnvironment()
	if err != nil {
		return nil, err
	}

	client.Authorizer = auth

	networkClient := servicefabricmesh.NewNetworkClient(azureSubscriptionID)
	networkClient.Authorizer = auth

	serviceClient := servicefabricmesh.NewServiceClient(azureSubscriptionID)
	serviceClient.Authorizer = auth

	provider := SFMeshProvider{
		nodeName:           nodeName,
		operatingSystem:    operatingSystem,
		internalIP:         internalIP,
		daemonEndpointPort: daemonEndpointPort,
		appClient:          &client,
		networkClient:      &networkClient,
		serviceClient:      &serviceClient,
		region:             region,
		resourceGroup:      resourceGroup,
		resourceManager:    rm,
		subscriptionID:     azureSubscriptionID,
	}
	return &provider, nil
}

func readDockerCfgSecret(secret *v1.Secret, ips []servicefabricmesh.ImageRegistryCredential) ([]servicefabricmesh.ImageRegistryCredential, error) {
	var err error
	var authConfigs map[string]AuthConfig
	repoData, ok := secret.Data[string(v1.DockerConfigKey)]

	if !ok {
		return ips, fmt.Errorf("no dockercfg present in secret")
	}

	err = json.Unmarshal(repoData, &authConfigs)
	if err != nil {
		return ips, err
	}

	for server, authConfig := range authConfigs {
		ips = append(ips, servicefabricmesh.ImageRegistryCredential{
			Password: &authConfig.Password,
			Server:   &server,
			Username: &authConfig.Username,
		})
	}

	return ips, err
}

func readDockerConfigJSONSecret(secret *v1.Secret, ips []servicefabricmesh.ImageRegistryCredential) ([]servicefabricmesh.ImageRegistryCredential, error) {
	var err error
	repoData, ok := secret.Data[string(v1.DockerConfigJsonKey)]

	if !ok {
		return ips, fmt.Errorf("no dockerconfigjson present in secret")
	}

	var authConfigs map[string]map[string]AuthConfig

	err = json.Unmarshal(repoData, &authConfigs)
	if err != nil {
		return ips, err
	}

	auths, ok := authConfigs["auths"]

	if !ok {
		return ips, fmt.Errorf("malformed dockerconfigjson in secret")
	}

	for server, authConfig := range auths {
		ips = append(ips, servicefabricmesh.ImageRegistryCredential{
			Password: &authConfig.Password,
			Server:   &server,
			Username: &authConfig.Username,
		})
	}

	return ips, err
}

func (p *SFMeshProvider) getImagePullSecrets(pod *v1.Pod) ([]servicefabricmesh.ImageRegistryCredential, error) {
	ips := make([]servicefabricmesh.ImageRegistryCredential, 0, len(pod.Spec.ImagePullSecrets))
	for _, ref := range pod.Spec.ImagePullSecrets {
		secret, err := p.resourceManager.GetSecret(ref.Name, pod.Namespace)
		if err != nil {
			return ips, err
		}
		if secret == nil {
			return nil, fmt.Errorf("error getting image pull secret")
		}

		switch secret.Type {
		case v1.SecretTypeDockercfg:
			ips, err = readDockerCfgSecret(secret, ips)
		case v1.SecretTypeDockerConfigJson:
			ips, err = readDockerConfigJSONSecret(secret, ips)
		default:
			return nil, fmt.Errorf("image pull secret type is not one of kubernetes.io/dockercfg or kubernetes.io/dockerconfigjson")
		}

		if err != nil {
			return ips, err
		}

	}
	return ips, nil
}

func (p *SFMeshProvider) getMeshApplication(pod *v1.Pod) (servicefabricmesh.ApplicationResourceDescription, error) {
	meshApp := servicefabricmesh.ApplicationResourceDescription{}
	meshApp.Name = &pod.Name
	meshApp.Location = &p.region

	podUID := string(pod.UID)
	podCreationTimestamp := pod.CreationTimestamp.String()

	tags := map[string]*string{
		"PodName":           &pod.Name,
		"ClusterName":       &pod.ClusterName,
		"NodeName":          &pod.Spec.NodeName,
		"Namespace":         &pod.Namespace,
		"UID":               &podUID,
		"CreationTimestamp": &podCreationTimestamp,
	}

	meshApp.Tags = tags

	properties := servicefabricmesh.ApplicationResourceProperties{}
	meshApp.ApplicationResourceProperties = &properties

	services := []servicefabricmesh.ServiceResourceDescription{}
	service := servicefabricmesh.ServiceResourceDescription{}
	serviceName := *meshApp.Name + "-service"
	service.Name = &serviceName
	serviceType := "Microsoft.ServiceFabricMesh/services"
	service.Type = &serviceType

	creds, err := p.getImagePullSecrets(pod)
	if err != nil {
		return meshApp, err
	}

	codePackages := []servicefabricmesh.ContainerCodePackageProperties{}

	for _, container := range pod.Spec.Containers {
		codePackage := servicefabricmesh.ContainerCodePackageProperties{}
		codePackage.Image = &container.Image
		codePackage.Name = &container.Name

		if creds != nil {
			if len(creds) > 0 {
				// Mesh ImageRegistryCredential supports only a single credential
				codePackage.ImageRegistryCredential = &creds[0]
			}
		}

		requirements := servicefabricmesh.ResourceRequirements{}
		requests := servicefabricmesh.ResourceRequests{}

		cpuRequest := defaultCPURequests
		memoryRequest := defaultMemoryRequests

		if container.Resources.Requests != nil {
			if _, ok := container.Resources.Requests[v1.ResourceCPU]; ok {
				containerCPURequest := float64(container.Resources.Requests.Cpu().MilliValue()/10.00) / 100.00
				if containerCPURequest > 1 && containerCPURequest <= 4 {
					cpuRequest = containerCPURequest
				}
			}

			if _, ok := container.Resources.Requests[v1.ResourceMemory]; ok {
				containerMemoryRequest := float64(container.Resources.Requests.Memory().Value()/100000000.00) / 10.00
				if containerMemoryRequest < 0.10 {
					containerMemoryRequest = 0.10
				}

				memoryRequest = containerMemoryRequest
			}
		}

		requests.CPU = &cpuRequest
		requests.MemoryInGB = &memoryRequest

		requirements.Requests = &requests

		if container.Resources.Limits != nil {
			cpuLimit := defaultCPULimit
			memoryLimit := defaultMemoryLimit

			limits := servicefabricmesh.ResourceLimits{}
			limits.CPU = &cpuLimit
			limits.MemoryInGB = &memoryLimit

			if _, ok := container.Resources.Limits[v1.ResourceCPU]; ok {
				containerCPULimit := float64(container.Resources.Limits.Cpu().MilliValue()) / 1000.00
				if containerCPULimit > 1 {
					limits.CPU = &containerCPULimit
				}
			}

			if _, ok := container.Resources.Limits[v1.ResourceMemory]; ok {
				containerMemoryLimit := float64(container.Resources.Limits.Memory().Value()) / 1000000000.00
				if containerMemoryLimit < 0.10 {
					containerMemoryLimit = 0.10
				}

				limits.MemoryInGB = &containerMemoryLimit
			}

			requirements.Limits = &limits
		}

		codePackage.Resources = &requirements

		if len(container.Command) > 0 {
			codePackage.Commands = &container.Command
		}

		if len(container.Env) > 0 {
			envVars := []servicefabricmesh.EnvironmentVariable{}

			for _, envVar := range container.Env {
				env := servicefabricmesh.EnvironmentVariable{}
				env.Name = &envVar.Name
				env.Value = &envVar.Value

				envVars = append(envVars, env)
			}

			codePackage.EnvironmentVariables = &envVars
		}

		endpoints := []servicefabricmesh.EndpointProperties{}

		for _, port := range container.Ports {
			endpoint := p.getEndpointFromContainerPort(port)
			endpoints = append(endpoints, endpoint)
		}

		if len(endpoints) > 0 {
			codePackage.Endpoints = &endpoints
		}

		codePackages = append(codePackages, codePackage)
	}

	serviceProperties := servicefabricmesh.ServiceResourceProperties{}
	serviceProperties.OsType = servicefabricmesh.Linux
	replicaCount := int32(1)
	serviceProperties.ReplicaCount = &replicaCount
	serviceProperties.CodePackages = &codePackages
	service.ServiceResourceProperties = &serviceProperties
	services = append(services, service)
	properties.Services = &services

	return meshApp, nil
}

func (p *SFMeshProvider) getMeshNetwork(pod *v1.Pod, meshApp servicefabricmesh.ApplicationResourceDescription, location string) servicefabricmesh.NetworkResourceDescription {
	network := servicefabricmesh.NetworkResourceDescription{}
	network.Name = meshApp.Name
	network.Location = &location

	networkProperties := servicefabricmesh.NetworkResourceProperties{}
	addressPrefix := "10.0.0.4/22"
	networkProperties.AddressPrefix = &addressPrefix

	layers := []servicefabricmesh.Layer4IngressConfig{}

	service := (*meshApp.Services)[0]

	for _, codePackage := range *service.CodePackages {
		for _, endpoint := range *codePackage.Endpoints {
			layer := p.getLayer(&endpoint, *meshApp.Name, *service.Name)
			layers = append(layers, layer)
		}
	}

	ingressConfig := servicefabricmesh.IngressConfig{}
	ingressConfig.Layer4 = &layers

	networkProperties.IngressConfig = &ingressConfig
	network.NetworkResourceProperties = &networkProperties

	return network
}

func (p *SFMeshProvider) getLayer(endpoint *servicefabricmesh.EndpointProperties, appName string, serviceName string) servicefabricmesh.Layer4IngressConfig {
	layer := servicefabricmesh.Layer4IngressConfig{}
	name := *endpoint.Name + "Ingress"
	layerName := &name
	layer.Name = layerName
	layer.PublicPort = endpoint.Port
	layer.EndpointName = endpoint.Name
	layer.ApplicationName = &appName
	layer.ServiceName = &serviceName

	return layer
}

func (p *SFMeshProvider) getEndpointFromContainerPort(port v1.ContainerPort) servicefabricmesh.EndpointProperties {
	endpoint := servicefabricmesh.EndpointProperties{}
	endpointName := strconv.Itoa(int(port.ContainerPort)) + "Listener"
	endpoint.Name = &endpointName
	endpoint.Port = &port.ContainerPort

	return endpoint
}

// CreatePod accepts a Pod definition and creates a SF Mesh App.
func (p *SFMeshProvider) CreatePod(ctx context.Context, pod *v1.Pod) error {
	log.Printf("receive CreatePod %q\n", pod.Name)

	meshApp, err := p.getMeshApplication(pod)
	if err != nil {
		return err
	}

	meshNetwork := p.getMeshNetwork(pod, meshApp, p.region)
	_, err = p.networkClient.Create(context.Background(), p.resourceGroup, *meshNetwork.Name, meshNetwork)
	if err != nil {
		return err
	}

	networkName := *meshNetwork.Name
	resourceID := "/subscriptions/" + p.subscriptionID + "/resourceGroups/" + p.resourceGroup + "/providers/Microsoft.ServiceFabricMesh/networks/" + networkName

	service := (*meshApp.Services)[0]

	networkRef := servicefabricmesh.NetworkRef{}
	networkRef.Name = &resourceID

	networkRefs := []servicefabricmesh.NetworkRef{}
	networkRefs = append(networkRefs, networkRef)

	service.NetworkRefs = &networkRefs

	_, err = p.appClient.Create(context.Background(), p.resourceGroup, pod.Name, meshApp)
	if err != nil {
		return err
	}

	return nil
}

// UpdatePod updates the pod running inside SF Mesh.
func (p *SFMeshProvider) UpdatePod(ctx context.Context, pod *v1.Pod) error {
	log.Printf("receive UpdatePod %q\n", pod.Name)

	app, err := p.getMeshApplication(pod)
	if err != nil {
		return err
	}

	_, err = p.appClient.Create(context.Background(), p.resourceGroup, pod.Name, app)
	if err != nil {
		return err
	}

	return nil
}

// DeletePod deletes the specified pod out of SF Mesh.
func (p *SFMeshProvider) DeletePod(ctx context.Context, pod *v1.Pod) (err error) {
	log.Printf("receive DeletePod %q\n", pod.Name)

	_, err = p.appClient.Delete(ctx, p.resourceGroup, pod.Name)
	if err != nil {
		return wrapError(err)
	}

	return nil
}

// GetPod returns a pod by name that is running inside SF Mesh.
// returns nil if a pod by that name is not found.
func (p *SFMeshProvider) GetPod(ctx context.Context, namespace, name string) (pod *v1.Pod, err error) {
	log.Printf("receive GetPod %q\n", name)

	resp, err := p.appClient.Get(ctx, p.resourceGroup, name)
	httpResponse := resp.Response.Response

	if err != nil {
		if httpResponse.StatusCode == 404 {
			return nil, nil
		}
		return nil, err
	}

	if resp.Tags == nil {
		return nil, nil
	}

	val, present := resp.Tags["NodeName"]

	if !present {
		return nil, nil
	}

	if *val != p.nodeName {
		return nil, nil
	}

	pod, err = p.applicationDescriptionToPod(resp)
	if err != nil {
		return nil, err
	}

	return pod, nil
}

func (p *SFMeshProvider) appStateToPodPhase(state string) v1.PodPhase {
	switch state {
	case "Succeeded":
		return v1.PodRunning
	case "Failed":
		return v1.PodFailed
	case "Canceled":
		return v1.PodFailed
	case "Creating":
		return v1.PodPending
	case "Updating":
		return v1.PodPending
	}

	return v1.PodUnknown
}

func (p *SFMeshProvider) appStateToPodConditions(state string, transitiontime metav1.Time) []v1.PodCondition {
	switch state {
	case "Succeeded":
		return []v1.PodCondition{
			v1.PodCondition{
				Type:               v1.PodReady,
				Status:             v1.ConditionTrue,
				LastTransitionTime: transitiontime,
			}, v1.PodCondition{
				Type:               v1.PodInitialized,
				Status:             v1.ConditionTrue,
				LastTransitionTime: transitiontime,
			}, v1.PodCondition{
				Type:               v1.PodScheduled,
				Status:             v1.ConditionTrue,
				LastTransitionTime: transitiontime,
			},
		}
	}
	return []v1.PodCondition{}
}

func (p *SFMeshProvider) getMeshService(appName string, serviceName string) (servicefabricmesh.ServiceResourceDescription, error) {
	svc, err := p.serviceClient.Get(context.Background(), p.resourceGroup, appName, serviceName)
	if err != nil {
		return servicefabricmesh.ServiceResourceDescription{}, err
	}

	return svc, err
}

func appStateToContainerState(state string, appStartTime metav1.Time) v1.ContainerState {
	if state == "Succeeded" {
		return v1.ContainerState{
			Running: &v1.ContainerStateRunning{
				StartedAt: appStartTime,
			},
		}
	}

	if state == "Failed" || state == "Canceled" {
		return v1.ContainerState{
			Terminated: &v1.ContainerStateTerminated{
				ExitCode:   1,
				Reason:     "",
				Message:    "",
				StartedAt:  appStartTime,
				FinishedAt: metav1.NewTime(time.Now()),
			},
		}
	}

	return v1.ContainerState{
		Waiting: &v1.ContainerStateWaiting{
			Reason:  "",
			Message: "",
		},
	}
}

func (p *SFMeshProvider) getMeshNetworkPublicIP(networkName string) (*string, error) {
	network, err := p.networkClient.Get(context.Background(), p.resourceGroup, networkName)
	if err != nil {
		return nil, err
	}

	ipAddress := network.IngressConfig.PublicIPAddress
	return ipAddress, nil
}

func (p *SFMeshProvider) applicationDescriptionToPod(app servicefabricmesh.ApplicationResourceDescription) (*v1.Pod, error) {
	var podCreationTimestamp metav1.Time

	if *app.Tags["CreationTimestamp"] != "" {
		t, err := time.Parse("2006-01-02 15:04:05.999999999 -0700 MST", *app.Tags["CreationTimestamp"])
		if err != nil {
			return nil, err
		}
		podCreationTimestamp = metav1.NewTime(t)
	}

	containerStartTime := podCreationTimestamp

	appState := app.ProvisioningState
	podPhase := p.appStateToPodPhase(*appState)
	podConditions := p.appStateToPodConditions(*appState, podCreationTimestamp)

	service, err := p.getMeshService(*app.Name, (*app.ServiceNames)[0])

	containers := []v1.Container{}
	containerStatuses := []v1.ContainerStatus{}

	for _, codePkg := range *service.CodePackages {
		container := v1.Container{}
		container.Name = *codePkg.Name
		container.Image = *codePkg.Image

		if codePkg.Commands != nil {
			container.Command = *codePkg.Commands
		}

		container.Resources = v1.ResourceRequirements{
			Requests: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse(fmt.Sprintf("%g", *codePkg.Resources.Requests.CPU)),
				v1.ResourceMemory: resource.MustParse(fmt.Sprintf("%gG", *codePkg.Resources.Requests.MemoryInGB)),
			},
		}

		if codePkg.Resources.Limits != nil {
			container.Resources.Limits = v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse(fmt.Sprintf("%g", *codePkg.Resources.Limits.CPU)),
				v1.ResourceMemory: resource.MustParse(fmt.Sprintf("%gG", *codePkg.Resources.Limits.MemoryInGB)),
			}
		}

		containerStatus := v1.ContainerStatus{
			Name:        *codePkg.Name,
			State:       appStateToContainerState(*appState, podCreationTimestamp),
			Ready:       podPhase == v1.PodRunning,
			Image:       container.Image,
			ImageID:     "",
			ContainerID: "",
		}

		containerStatuses = append(containerStatuses, containerStatus)
		containers = append(containers, container)
	}

	appName := app.Name
	ipAddress := ""
	meshIP, err := p.getMeshNetworkPublicIP(*appName)
	if err != nil {
		return nil, err
	}

	if meshIP != nil {
		ipAddress = *meshIP
	}

	pod := v1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:              *app.Tags["PodName"],
			Namespace:         *app.Tags["Namespace"],
			ClusterName:       *app.Tags["ClusterName"],
			UID:               types.UID(*app.Tags["UID"]),
			CreationTimestamp: podCreationTimestamp,
		},
		Spec: v1.PodSpec{
			NodeName:   *app.Tags["NodeName"],
			Volumes:    []v1.Volume{},
			Containers: containers,
		},
		Status: v1.PodStatus{
			Phase:             podPhase,
			Conditions:        podConditions,
			Message:           "",
			Reason:            "",
			HostIP:            "",
			PodIP:             ipAddress,
			StartTime:         &containerStartTime,
			ContainerStatuses: containerStatuses,
		},
	}

	return &pod, nil
}

// GetContainerLogs retrieves the logs of a container by name.
func (p *SFMeshProvider) GetContainerLogs(ctx context.Context, namespace, podName, containerName string, tail int) (string, error) {
	log.Printf("receive GetContainerLogs %q\n", podName)
	return "", nil
}

// GetPodFullName gets the full pod name as defined in the provider context
func (p *SFMeshProvider) GetPodFullName(namespace string, pod string) string {
	return ""
}

// ExecInContainer executes a command in a container in the pod, copying data
// between in/out/err and the container's stdin/stdout/stderr.
func (p *SFMeshProvider) ExecInContainer(name string, uid types.UID, container string, cmd []string, in io.Reader, out, err io.WriteCloser, tty bool, resize <-chan remotecommand.TerminalSize, timeout time.Duration) error {
	log.Printf("receive ExecInContainer %q\n", container)
	return nil
}

// GetPodStatus returns the status of a pod by name that is "running".
// returns nil if a pod by that name is not found.
func (p *SFMeshProvider) GetPodStatus(ctx context.Context, namespace, name string) (*v1.PodStatus, error) {
	pod, err := p.GetPod(ctx, namespace, name)
	if err != nil {
		return nil, err
	}

	if pod == nil {
		return nil, nil
	}

	return &pod.Status, nil
}

// GetPods returns a list of all pods known to be running within SF Mesh.
func (p *SFMeshProvider) GetPods(ctx context.Context) ([]*v1.Pod, error) {
	log.Printf("receive GetPods\n")

	var pods []*v1.Pod

	list, err := p.appClient.ListByResourceGroup(ctx, p.resourceGroup)
	if err != nil {
		return pods, err
	}

	apps := list.Values()

	for _, app := range apps {
		if app.Tags == nil {
			continue
		}

		val, present := app.Tags["NodeName"]

		if !present {
			continue
		}

		if *val != p.nodeName {
			continue
		}

		pod, err := p.applicationDescriptionToPod(app)
		if err != nil {
			return pods, err
		}

		pods = append(pods, pod)
	}

	return pods, nil
}

// Capacity returns a resource list containing the capacity limits set for SF Mesh.
func (p *SFMeshProvider) Capacity(ctx context.Context) v1.ResourceList {
	return v1.ResourceList{
		"cpu":    resource.MustParse(defaultCPUCapacity),
		"memory": resource.MustParse(defaultMemoryCapacity),
		"pods":   resource.MustParse(defaultPodCapacity),
	}
}

// NodeConditions returns a list of conditions (Ready, OutOfDisk, etc), for updates to the node status
// within Kubernetes.
func (p *SFMeshProvider) NodeConditions(ctx context.Context) []v1.NodeCondition {
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
func (p *SFMeshProvider) NodeAddresses(ctx context.Context) []v1.NodeAddress {
	return []v1.NodeAddress{
		{
			Type:    "InternalIP",
			Address: p.internalIP,
		},
	}
}

// NodeDaemonEndpoints returns NodeDaemonEndpoints for the node status
// within Kubernetes.
func (p *SFMeshProvider) NodeDaemonEndpoints(ctx context.Context) *v1.NodeDaemonEndpoints {
	return &v1.NodeDaemonEndpoints{
		KubeletEndpoint: v1.DaemonEndpoint{
			Port: p.daemonEndpointPort,
		},
	}
}

// OperatingSystem returns the operating system for this provider.
// This is a noop to default to Linux for now.
func (p *SFMeshProvider) OperatingSystem() string {
	return providers.OperatingSystemLinux
}
