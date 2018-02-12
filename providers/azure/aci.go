package azure

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/virtual-kubelet/virtual-kubelet/manager"
	client "github.com/virtual-kubelet/virtual-kubelet/providers/azure/client"
	"github.com/virtual-kubelet/virtual-kubelet/providers/azure/client/aci"
	"k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// The service account secret mount path.
const serviceAccountSecretMountPath = "/var/run/secrets/kubernetes.io/serviceaccount"

// ACIProvider implements the virtual-kubelet provider interface and communicates with Azure's ACI APIs.
type ACIProvider struct {
	aciClient          *aci.Client
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

// NewACIProvider creates a new ACIProvider.
func NewACIProvider(config string, rm *manager.ResourceManager, nodeName, operatingSystem string, internalIP string, daemonEndpointPort int32) (*ACIProvider, error) {
	var p ACIProvider
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

	var azAuth *client.Authentication

	if authFilepath := os.Getenv("AZURE_AUTH_LOCATION"); authFilepath != "" {
		auth, err := client.NewAuthenticationFromFile(authFilepath)
		if err != nil {
			return nil, err
		}

		azAuth = auth
	}

	if acsFilepath := os.Getenv("ACS_CREDENTIAL_LOCATION"); acsFilepath != "" {
		acsCredential, err := NewAcsCredential(acsFilepath)
		if err != nil {
			return nil, err
		}

		if acsCredential != nil {
			if acsCredential.ClientSecret != client.PublicCloud.Name {
				return nil, fmt.Errorf("ACI only supports Public Azure. '%v' is not supported", acsCredential.Cloud)
			}
	
			azAuth = client.NewAuthentication(
				acsCredential.Cloud, 
				acsCredential.ClientID, 
				acsCredential.ClientSecret, 
				acsCredential.SubscriptionID, 
				acsCredential.TenantID)
	
			p.resourceGroup = acsCredential.ResourceGroup
			p.region = acsCredential.Region
		}
	}
	
	if clientID := os.Getenv("AZURE_CLIENT_ID"); clientID != "" {
		azAuth.ClientID = clientID
	}

	if clientSecret := os.Getenv("AZURE_CLIENT_SECRET"); clientSecret != "" {
		azAuth.ClientSecret = clientSecret
	}

	if tenantID := os.Getenv("AZURE_TENANT_ID"); tenantID != "" {
		azAuth.TenantID = tenantID
	}

	if subscriptionID := os.Getenv("AZURE_SUBSCRIPTION_ID"); subscriptionID != "" {
		azAuth.SubscriptionID = subscriptionID
	}

	p.aciClient, err = aci.NewClient(azAuth)
	if err != nil {
		return nil, err
	}

	if rg := os.Getenv("ACI_RESOURCE_GROUP"); rg != "" {
		p.resourceGroup = rg
	}
	if p.resourceGroup == "" {
		return nil, errors.New("Resource group can not be empty please set ACI_RESOURCE_GROUP")
	}

	if r := os.Getenv("ACI_REGION"); r != "" {
		p.region = r
	}
	if p.region == "" {
		return nil, errors.New("Region can not be empty please set ACI_REGION")
	}

	// Set sane defaults for Capacity in case config is not supplied
	p.cpu = "20"
	p.memory = "100Gi"
	p.pods = "20"

	p.operatingSystem = operatingSystem
	p.nodeName = nodeName
	p.internalIP = internalIP
	p.daemonEndpointPort = daemonEndpointPort

	return &p, err
}

// CreatePod accepts a Pod definition and creates
// an ACI deployment
func (p *ACIProvider) CreatePod(pod *v1.Pod) error {
	var containerGroup aci.ContainerGroup
	containerGroup.Location = p.region
	containerGroup.RestartPolicy = aci.ContainerGroupRestartPolicy(pod.Spec.RestartPolicy)
	containerGroup.ContainerGroupProperties.OsType = aci.OperatingSystemTypes(p.OperatingSystem())

	// get containers
	containers, err := p.getContainers(pod)
	if err != nil {
		return err
	}
	// get registry creds
	creds, err := p.getImagePullSecrets(pod)
	if err != nil {
		return err
	}
	// get volumes
	volumes, err := p.getVolumes(pod)
	if err != nil {
		return err
	}
	// assign all the things
	containerGroup.ContainerGroupProperties.Containers = containers
	containerGroup.ContainerGroupProperties.Volumes = volumes
	containerGroup.ContainerGroupProperties.ImageRegistryCredentials = creds

	filterServiceAccountSecretVolume(p.operatingSystem, &containerGroup)

	// create ipaddress if containerPort is used
	count := 0
	for _, container := range containers {
		count = count + len(container.Ports)
	}
	ports := make([]aci.Port, 0, count)
	for _, container := range containers {
		for _, containerPort := range container.Ports {

			ports = append(ports, aci.Port{
				Port:     containerPort.Port,
				Protocol: aci.ContainerGroupNetworkProtocol("TCP"),
			})
		}
	}
	if len(ports) > 0 {
		containerGroup.ContainerGroupProperties.IPAddress = &aci.IPAddress{
			Ports: ports,
			Type:  "Public",
		}
	}

	podUID := string(pod.UID)
	podCreationTimestamp := pod.CreationTimestamp.String()
	containerGroup.Tags = map[string]string{
		"PodName":           pod.Name,
		"ClusterName":       pod.ClusterName,
		"NodeName":          pod.Spec.NodeName,
		"Namespace":         pod.Namespace,
		"UID":               podUID,
		"CreationTimestamp": podCreationTimestamp,
	}

	// TODO(BJK) containergrouprestartpolicy??
	_, err = p.aciClient.CreateContainerGroup(
		p.resourceGroup,
		fmt.Sprintf("%s-%s", pod.Namespace, pod.Name),
		containerGroup,
	)

	return err
}

// UpdatePod is a noop, ACI currently does not support live updates of a pod.
func (p *ACIProvider) UpdatePod(pod *v1.Pod) error {
	return nil
}

// DeletePod deletes the specified pod out of ACI.
func (p *ACIProvider) DeletePod(pod *v1.Pod) error {
	return p.aciClient.DeleteContainerGroup(p.resourceGroup, fmt.Sprintf("%s-%s", pod.Namespace, pod.Name))
}

// GetPod returns a pod by name that is running inside ACI
// returns nil if a pod by that name is not found.
func (p *ACIProvider) GetPod(namespace, name string) (*v1.Pod, error) {
	cg, err, status := p.aciClient.GetContainerGroup(p.resourceGroup, fmt.Sprintf("%s-%s", namespace, name))
	if err != nil {
		if *status == http.StatusNotFound {
			return nil, nil
		}
		return nil, err
	}

	if cg.Tags["NodeName"] != p.nodeName {
		return nil, nil
	}

	return containerGroupToPod(cg)
}

// GetContainerLogs returns the logs of a pod by name that is running inside ACI.
func (p *ACIProvider) GetContainerLogs(namespace, podName, containerName string, tail int) (string, error) {
	logContent := ""
	cg, err, _ := p.aciClient.GetContainerGroup(p.resourceGroup, fmt.Sprintf("%s-%s", namespace, podName))
	if err != nil {
		return logContent, err
	}

	if cg.Tags["NodeName"] != p.nodeName {
		return logContent, nil
	}
	// get logs from cg
	retry := 10
	for i := 0; i < retry; i++ {
		cLogs, err := p.aciClient.GetContainerLogs(p.resourceGroup, cg.Name, containerName, tail)
		if err != nil {
			log.Println(err)
			time.Sleep(5000 * time.Millisecond)
		} else {
			logContent = cLogs.Content
			break
		}
	}

	return logContent, err
}

// GetPodStatus returns the status of a pod by name that is running inside ACI
// returns nil if a pod by that name is not found.
func (p *ACIProvider) GetPodStatus(namespace, name string) (*v1.PodStatus, error) {
	pod, err := p.GetPod(namespace, name)
	if err != nil {
		return nil, err
	}

	if pod == nil {
		return nil, nil
	}

	return &pod.Status, nil
}

// GetPods returns a list of all pods known to be running within ACI.
func (p *ACIProvider) GetPods() ([]*v1.Pod, error) {
	cgs, err := p.aciClient.ListContainerGroups(p.resourceGroup)
	if err != nil {
		return nil, err
	}
	pods := make([]*v1.Pod, 0, len(cgs.Value))

	for _, cg := range cgs.Value {
		c := cg
		if cg.Tags["NodeName"] != p.nodeName {
			continue
		}

		p, err := containerGroupToPod(&c)
		if err != nil {
			log.Println(err)
			continue
		}
		pods = append(pods, p)
	}

	return pods, nil
}

// Capacity returns a resource list containing the capacity limits set for ACI.
func (p *ACIProvider) Capacity() v1.ResourceList {
	return v1.ResourceList{
		"cpu":    resource.MustParse(p.cpu),
		"memory": resource.MustParse(p.memory),
		"pods":   resource.MustParse(p.pods),
	}
}

// NodeConditions returns a list of conditions (Ready, OutOfDisk, etc), for updates to the node status
// within Kubernetes.
func (p *ACIProvider) NodeConditions() []v1.NodeCondition {
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
func (p *ACIProvider) NodeAddresses() []v1.NodeAddress {
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
func (p *ACIProvider) NodeDaemonEndpoints() *v1.NodeDaemonEndpoints {
	return &v1.NodeDaemonEndpoints{
		KubeletEndpoint: v1.DaemonEndpoint{
			Port: p.daemonEndpointPort,
		},
	}
}

// OperatingSystem returns the operating system that was provided by the config.
func (p *ACIProvider) OperatingSystem() string {
	return p.operatingSystem
}

func (p *ACIProvider) getImagePullSecrets(pod *v1.Pod) ([]aci.ImageRegistryCredential, error) {
	ips := make([]aci.ImageRegistryCredential, 0, len(pod.Spec.ImagePullSecrets))
	for _, ref := range pod.Spec.ImagePullSecrets {
		secret, err := p.resourceManager.GetSecret(ref.Name, pod.Namespace)
		if err != nil {
			return ips, err
		}
		if secret == nil {
			return nil, fmt.Errorf("error getting image pull secret")
		}
		// TODO: Check if secret type is v1.SecretTypeDockercfg and use DockerConfigKey instead of hardcoded value
		// TODO: Check if secret type is v1.SecretTypeDockerConfigJson and use DockerConfigJsonKey to determine if it's in json format
		// TODO: Return error if it's not one of these two types
		repoData, ok := secret.Data[".dockercfg"]
		if !ok {
			return ips, fmt.Errorf("no dockercfg present in secret")
		}

		var authConfigs map[string]AuthConfig
		err = json.Unmarshal(repoData, &authConfigs)
		if err != nil {
			return ips, err
		}

		for server, authConfig := range authConfigs {
			ips = append(ips, aci.ImageRegistryCredential{
				Password: authConfig.Password,
				Server:   server,
				Username: authConfig.Username,
			})
		}
	}
	return ips, nil
}

func (p *ACIProvider) getContainers(pod *v1.Pod) ([]aci.Container, error) {
	containers := make([]aci.Container, 0, len(pod.Spec.Containers))
	for _, container := range pod.Spec.Containers {
		c := aci.Container{
			Name: container.Name,
			ContainerProperties: aci.ContainerProperties{
				Image:   container.Image,
				Command: container.Command,
				Ports:   make([]aci.ContainerPort, 0, len(container.Ports)),
			},
		}

		for _, p := range container.Ports {
			c.Ports = append(c.Ports, aci.ContainerPort{
				Port:     p.ContainerPort,
				Protocol: getProtocol(p.Protocol),
			})
		}

		c.VolumeMounts = make([]aci.VolumeMount, 0, len(container.VolumeMounts))
		for _, v := range container.VolumeMounts {
			c.VolumeMounts = append(c.VolumeMounts, aci.VolumeMount{
				Name:      v.Name,
				MountPath: v.MountPath,
				ReadOnly:  v.ReadOnly,
			})
		}

		c.EnvironmentVariables = make([]aci.EnvironmentVariable, 0, len(container.Env))
		for _, e := range container.Env {
			c.EnvironmentVariables = append(c.EnvironmentVariables, aci.EnvironmentVariable{
				Name:  e.Name,
				Value: e.Value,
			})
		}

		cpuLimit := float64(container.Resources.Limits.Cpu().Value())
		memoryLimit := float64(container.Resources.Limits.Memory().Value()) / 1000000000.00
		cpuRequest := float64(container.Resources.Requests.Cpu().Value())
		memoryRequest := float64(container.Resources.Requests.Memory().Value()) / 1000000000.00

		c.Resources = aci.ResourceRequirements{
			Limits: aci.ResourceLimits{
				CPU:        cpuLimit,
				MemoryInGB: memoryLimit,
			},
			Requests: aci.ResourceRequests{
				CPU:        cpuRequest,
				MemoryInGB: memoryRequest,
			},
		}

		containers = append(containers, c)
	}
	return containers, nil
}

func (p *ACIProvider) getVolumes(pod *v1.Pod) ([]aci.Volume, error) {
	volumes := make([]aci.Volume, 0, len(pod.Spec.Volumes))
	for _, v := range pod.Spec.Volumes {
		// Handle the case for the AzureFile volume.
		if v.AzureFile != nil {
			secret, err := p.resourceManager.GetSecret(v.AzureFile.SecretName, pod.Namespace)
			if err != nil {
				return volumes, err
			}

			if secret == nil {
				return nil, fmt.Errorf("Getting secret for AzureFile volume returned an empty secret.")
			}

			volumes = append(volumes, aci.Volume{
				Name: v.Name,
				AzureFile: &aci.AzureFileVolume{
					ShareName:          v.AzureFile.ShareName,
					ReadOnly:           v.AzureFile.ReadOnly,
					StorageAccountName: string(secret.Data["StorageAccountName"]),
					StorageAccountKey:  string(secret.Data["StorageAccountKey"]),
				},
			})
			continue
		}

		// Handle the case for the EmptyDir.
		if v.EmptyDir != nil {
			volumes = append(volumes, aci.Volume{
				Name:     v.Name,
				EmptyDir: map[string]interface{}{},
			})
			continue
		}

		// Handle the case for GitRepo volume.
		if v.GitRepo != nil {
			volumes = append(volumes, aci.Volume{
				Name: v.Name,
				GitRepo: &aci.GitRepoVolume{
					Directory:  v.GitRepo.Directory,
					Repository: v.GitRepo.Repository,
					Revision:   v.GitRepo.Revision,
				},
			})
			continue
		}

		// Handle the case for Secret volume.
		if v.Secret != nil {
			paths := make(map[string]string)
			secret, err := p.resourceManager.GetSecret(v.Secret.SecretName, pod.Namespace)
			if v.Secret.Optional != nil && !*v.Secret.Optional && k8serr.IsNotFound(err) {
				return nil, fmt.Errorf("Secret %s is required by Pod %s and does not exist", v.Secret.SecretName, pod.Name)
			}
			if secret == nil {
				continue
			}

			for k, v := range secret.Data {
				var b bytes.Buffer
				enc := base64.NewEncoder(base64.StdEncoding, &b)
				enc.Write(v)

				paths[k] = b.String()
			}

			if len(paths) != 0 {
				volumes = append(volumes, aci.Volume{
					Name:   v.Name,
					Secret: paths,
				})
			}
			continue
		}

		// Handle the case for ConfigMap volume.
		if v.ConfigMap != nil {
			paths := make(map[string]string)
			configMap, err := p.resourceManager.GetConfigMap(v.ConfigMap.Name, pod.Namespace)
			if v.ConfigMap.Optional != nil && !*v.ConfigMap.Optional && k8serr.IsNotFound(err) {
				return nil, fmt.Errorf("ConfigMap %s is required by Pod %s and does not exist", v.ConfigMap.Name, pod.Name)
			}
			if configMap == nil {
				continue
			}

			for k, v := range configMap.Data {
				var b bytes.Buffer
				enc := base64.NewEncoder(base64.StdEncoding, &b)
				enc.Write([]byte(v))

				paths[k] = b.String()
			}

			if len(paths) != 0 {
				volumes = append(volumes, aci.Volume{
					Name:   v.Name,
					Secret: paths,
				})
			}
			continue
		}

		// If we've made it this far we have found a volume type that isn't supported
		return nil, fmt.Errorf("Pod %s requires volume %s which is of an unsupported type", pod.Name, v.Name)
	}

	return volumes, nil
}

func getProtocol(pro v1.Protocol) aci.ContainerNetworkProtocol {
	switch pro {
	case v1.ProtocolUDP:
		return aci.ContainerNetworkProtocolUDP
	default:
		return aci.ContainerNetworkProtocolTCP
	}
}

func containerGroupToPod(cg *aci.ContainerGroup) (*v1.Pod, error) {
	var podCreationTimestamp metav1.Time

	if cg.Tags["CreationTimestamp"] != "" {
		t, err := time.Parse("2006-01-02 15:04:05.999999999 -0700 MST", cg.Tags["CreationTimestamp"])
		if err != nil {
			return nil, err
		}
		podCreationTimestamp = metav1.NewTime(t)
	}
	containerStartTime := metav1.NewTime(time.Time(cg.Containers[0].ContainerProperties.InstanceView.CurrentState.StartTime))

	// Use the Provisioning State if it's not Succeeded,
	// otherwise use the state of the instance.
	aciState := cg.ContainerGroupProperties.ProvisioningState
	if aciState == "Succeeded" {
		aciState = cg.ContainerGroupProperties.InstanceView.State
	}

	containers := make([]v1.Container, 0, len(cg.Containers))
	containerStatuses := make([]v1.ContainerStatus, 0, len(cg.Containers))
	for _, c := range cg.Containers {
		container := v1.Container{
			Name:    c.Name,
			Image:   c.Image,
			Command: c.Command,
			Resources: v1.ResourceRequirements{
				Limits: v1.ResourceList{
					v1.ResourceCPU:    resource.MustParse(fmt.Sprintf("%d", int64(c.Resources.Limits.CPU))),
					v1.ResourceMemory: resource.MustParse(fmt.Sprintf("%gG", c.Resources.Limits.MemoryInGB)),
				},
				Requests: v1.ResourceList{
					v1.ResourceCPU:    resource.MustParse(fmt.Sprintf("%d", int64(c.Resources.Requests.CPU))),
					v1.ResourceMemory: resource.MustParse(fmt.Sprintf("%gG", c.Resources.Requests.MemoryInGB)),
				},
			},
		}
		containers = append(containers, container)
		containerStatus := v1.ContainerStatus{
			Name:                 c.Name,
			State:                aciContainerStateToContainerState(c.InstanceView.CurrentState),
			LastTerminationState: aciContainerStateToContainerState(c.InstanceView.PreviousState),
			Ready:                aciStateToPodPhase(c.InstanceView.CurrentState.State) == v1.PodRunning,
			RestartCount:         c.InstanceView.RestartCount,
			Image:                c.Image,
			ImageID:              "",
			ContainerID:          "",
		}

		// Add to containerStatuses
		containerStatuses = append(containerStatuses, containerStatus)
	}

	ip := ""
	if cg.IPAddress != nil {
		ip = cg.IPAddress.IP
	}

	p := v1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:              cg.Tags["PodName"],
			Namespace:         cg.Tags["Namespace"],
			ClusterName:       cg.Tags["ClusterName"],
			UID:               types.UID(cg.Tags["UID"]),
			CreationTimestamp: podCreationTimestamp,
		},
		Spec: v1.PodSpec{
			NodeName:   cg.Tags["NodeName"],
			Volumes:    []v1.Volume{},
			Containers: containers,
		},
		// TODO: Make this dynamic, likely can translate the provisioningState or instanceView.ContainerState into a Phase,
		// and some of the Events into Conditions
		Status: v1.PodStatus{
			Phase:             aciStateToPodPhase(aciState),
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

func aciStateToPodPhase(state string) v1.PodPhase {
	switch state {
	case "Running":
		return v1.PodRunning
	case "Succeeded":
		return v1.PodSucceeded
	case "Failed":
		return v1.PodFailed
	case "Canceled":
		return v1.PodFailed
	case "Creating":
		return v1.PodPending
	case "Repairing":
		return v1.PodPending
	case "Pending":
		return v1.PodPending
	case "Accepted":
		return v1.PodPending
	}

	return v1.PodUnknown
}

func aciContainerStateToContainerState(cs aci.ContainerState) v1.ContainerState {
	startTime := metav1.NewTime(time.Time(cs.StartTime))

	// Handle the case where the container is running.
	if cs.State == "Running" || cs.State == "Succeeded" {
		return v1.ContainerState{
			Running: &v1.ContainerStateRunning{
				StartedAt: startTime,
			},
		}
	}

	// Handle the case where the container failed.
	if cs.State == "Failed" || cs.State == "Canceled" {
		return v1.ContainerState{
			Terminated: &v1.ContainerStateTerminated{
				ExitCode:   cs.ExitCode,
				Reason:     cs.State,
				Message:    cs.DetailStatus,
				StartedAt:  startTime,
				FinishedAt: metav1.NewTime(time.Time(cs.FinishTime)),
			},
		}
	}

	// Handle the case where the container is pending.
	// Which should be all other aci states.
	return v1.ContainerState{
		Waiting: &v1.ContainerStateWaiting{
			Reason:  cs.State,
			Message: cs.DetailStatus,
		},
	}
}

// Filters service account secret volume for Windows.
// Service account secret volume gets automatically turned on if not specified otherwise.
// ACI doesn't support secret volume for Windows, so we need to filter it.
func filterServiceAccountSecretVolume(osType string, containerGroup *aci.ContainerGroup) {
	if strings.EqualFold(osType, "Windows") {
		serviceAccountSecretVolumeName := make(map[string]bool)

		for index, container := range containerGroup.ContainerGroupProperties.Containers {
			volumeMounts := make([]aci.VolumeMount, 0, len(container.VolumeMounts))
			for _, volumeMount := range container.VolumeMounts {
				if !strings.EqualFold(serviceAccountSecretMountPath, volumeMount.MountPath) {
					volumeMounts = append(volumeMounts, volumeMount)
				} else {
					serviceAccountSecretVolumeName[volumeMount.Name] = true
				}
			}
			containerGroup.ContainerGroupProperties.Containers[index].VolumeMounts = volumeMounts
		}

		if len(serviceAccountSecretVolumeName) == 0 {
			return
		}

		log.Printf("Ignoring service account secret volumes '%v' for Windows", reflect.ValueOf(serviceAccountSecretVolumeName).MapKeys())

		volumes := make([]aci.Volume, 0, len(containerGroup.ContainerGroupProperties.Volumes))
		for _, volume := range containerGroup.ContainerGroupProperties.Volumes {
			if _, ok := serviceAccountSecretVolumeName[volume.Name]; !ok {
				volumes = append(volumes, volume)
			}
		}

		containerGroup.ContainerGroupProperties.Volumes = volumes
	}
}
