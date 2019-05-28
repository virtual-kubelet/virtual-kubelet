package alibabacloud

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/requests"
	"github.com/cpuguy83/strongerrors"
	"github.com/virtual-kubelet/virtual-kubelet/log"
	"github.com/virtual-kubelet/virtual-kubelet/manager"
	"github.com/virtual-kubelet/virtual-kubelet/providers/alibabacloud/eci"
	"github.com/virtual-kubelet/virtual-kubelet/vkubelet/api"
	v1 "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// The service account secret mount path.
const serviceAccountSecretMountPath = "/var/run/secrets/kubernetes.io/serviceaccount"

const podTagTimeFormat = "2006-01-02T15-04-05Z"
const timeFormat = "2006-01-02T15:04:05Z"

// ECIProvider implements the virtual-kubelet provider interface and communicates with Alibaba Cloud's ECI APIs.
type ECIProvider struct {
	eciClient          *eci.Client
	resourceManager    *manager.ResourceManager
	resourceGroup      string
	region             string
	nodeName           string
	operatingSystem    string
	clusterName        string
	cpu                string
	memory             string
	pods               string
	internalIP         string
	daemonEndpointPort int32
	secureGroup        string
	vSwitch            string
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

var validEciRegions = []string{
	"cn-hangzhou",
	"cn-shanghai",
	"cn-beijing",
	"us-west-1",
}

// isValidECIRegion checks to make sure we're using a valid ECI region
func isValidECIRegion(region string) bool {
	regionLower := strings.ToLower(region)
	regionTrimmed := strings.Replace(regionLower, " ", "", -1)

	for _, validRegion := range validEciRegions {
		if regionTrimmed == validRegion {
			return true
		}
	}

	return false
}

// NewECIProvider creates a new ECIProvider.
func NewECIProvider(config string, rm *manager.ResourceManager, nodeName, operatingSystem string, internalIP string, daemonEndpointPort int32) (*ECIProvider, error) {
	var p ECIProvider
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
	if r := os.Getenv("ECI_CLUSTER_NAME"); r != "" {
		p.clusterName = r
	}
	if p.clusterName == "" {
		p.clusterName = "default"
	}
	if r := os.Getenv("ECI_REGION"); r != "" {
		p.region = r
	}
	if p.region == "" {
		return nil, errors.New("Region can't be empty please set ECI_REGION\n")
	}
	if r := p.region; !isValidECIRegion(r) {
		unsupportedRegionMessage := fmt.Sprintf("Region %s is invalid. Current supported regions are: %s",
			r, strings.Join(validEciRegions, ", "))

		return nil, errors.New(unsupportedRegionMessage)
	}

	var accessKey, secretKey string

	if ak := os.Getenv("ECI_ACCESS_KEY"); ak != "" {
		accessKey = ak
	}
	if sk := os.Getenv("ECI_SECRET_KEY"); sk != "" {
		secretKey = sk
	}
	if sg := os.Getenv("ECI_SECURITY_GROUP"); sg != "" {
		p.secureGroup = sg
	}
	if vsw := os.Getenv("ECI_VSWITCH"); vsw != "" {
		p.vSwitch = vsw
	}
	if p.secureGroup == "" {
		return nil, errors.New("secureGroup can't be empty\n")
	}

	if p.vSwitch == "" {
		return nil, errors.New("vSwitch can't be empty\n")
	}

	p.eciClient, err = eci.NewClientWithAccessKey(p.region, accessKey, secretKey)
	if err != nil {
		return nil, err
	}

	p.cpu = "1000"
	p.memory = "4Ti"
	p.pods = "1000"

	if cpuQuota := os.Getenv("ECI_QUOTA_CPU"); cpuQuota != "" {
		p.cpu = cpuQuota
	}

	if memoryQuota := os.Getenv("ECI_QUOTA_MEMORY"); memoryQuota != "" {
		p.memory = memoryQuota
	}

	if podsQuota := os.Getenv("ECI_QUOTA_POD"); podsQuota != "" {
		p.pods = podsQuota
	}

	p.operatingSystem = operatingSystem
	p.nodeName = nodeName
	p.internalIP = internalIP
	p.daemonEndpointPort = daemonEndpointPort
	return &p, err
}

// CreatePod accepts a Pod definition and creates
// an ECI deployment
func (p *ECIProvider) CreatePod(ctx context.Context, pod *v1.Pod) error {
	//Ignore daemonSet Pod
	if pod != nil && pod.OwnerReferences != nil && len(pod.OwnerReferences) != 0 && pod.OwnerReferences[0].Kind == "DaemonSet" {
		msg := fmt.Sprintf("Skip to create DaemonSet pod %q", pod.Name)
		log.G(ctx).WithField("Method", "CreatePod").Info(msg)
		return nil
	}

	request := eci.CreateCreateContainerGroupRequest()
	request.RestartPolicy = string(pod.Spec.RestartPolicy)

	// get containers
	containers, err := p.getContainers(pod, false)
	initContainers, err := p.getContainers(pod, true)
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
	request.Containers = containers
	request.InitContainers = initContainers
	request.Volumes = volumes
	request.ImageRegistryCredentials = creds
	CreationTimestamp := pod.CreationTimestamp.UTC().Format(podTagTimeFormat)
	tags := []eci.Tag{
		eci.Tag{Key: "ClusterName", Value: p.clusterName},
		eci.Tag{Key: "NodeName", Value: p.nodeName},
		eci.Tag{Key: "NameSpace", Value: pod.Namespace},
		eci.Tag{Key: "PodName", Value: pod.Name},
		eci.Tag{Key: "UID", Value: string(pod.UID)},
		eci.Tag{Key: "CreationTimestamp", Value: CreationTimestamp},
	}

	ContainerGroupName := containerGroupName(pod)
	request.Tags = tags
	request.SecurityGroupId = p.secureGroup
	request.VSwitchId = p.vSwitch
	request.ContainerGroupName = ContainerGroupName
	msg := fmt.Sprintf("CreateContainerGroup request %+v", request)
	log.G(ctx).WithField("Method", "CreatePod").Info(msg)
	response, err := p.eciClient.CreateContainerGroup(request)
	if err != nil {
		return err
	}
	msg = fmt.Sprintf("CreateContainerGroup successed. %s, %s, %s", response.RequestId, response.ContainerGroupId, ContainerGroupName)
	log.G(ctx).WithField("Method", "CreatePod").Info(msg)
	return nil
}

func containerGroupName(pod *v1.Pod) string {
	return fmt.Sprintf("%s-%s", pod.Namespace, pod.Name)
}

// UpdatePod is a noop, ECI currently does not support live updates of a pod.
func (p *ECIProvider) UpdatePod(ctx context.Context, pod *v1.Pod) error {
	return nil
}

// DeletePod deletes the specified pod out of ECI.
func (p *ECIProvider) DeletePod(ctx context.Context, pod *v1.Pod) error {
	eciId := ""
	for _, cg := range p.GetCgs() {
		if getECITagValue(&cg, "PodName") == pod.Name && getECITagValue(&cg, "NameSpace") == pod.Namespace {
			eciId = cg.ContainerGroupId
			break
		}
	}
	if eciId == "" {
		return strongerrors.NotFound(fmt.Errorf("DeletePod can't find Pod %s-%s", pod.Namespace, pod.Name))
	}

	request := eci.CreateDeleteContainerGroupRequest()
	request.ContainerGroupId = eciId
	_, err := p.eciClient.DeleteContainerGroup(request)
	return wrapError(err)
}

// GetPod returns a pod by name that is running inside ECI
// returns nil if a pod by that name is not found.
func (p *ECIProvider) GetPod(ctx context.Context, namespace, name string) (*v1.Pod, error) {
	pods, err := p.GetPods(ctx)
	if err != nil {
		return nil, err
	}
	for _, pod := range pods {
		if pod.Name == name && pod.Namespace == namespace {
			return pod, nil
		}
	}
	return nil, nil
}

// GetContainerLogs returns the logs of a pod by name that is running inside ECI.
func (p *ECIProvider) GetContainerLogs(ctx context.Context, namespace, podName, containerName string, opts api.ContainerLogOpts) (io.ReadCloser, error) {
	eciId := ""
	for _, cg := range p.GetCgs() {
		if getECITagValue(&cg, "PodName") == podName && getECITagValue(&cg, "NameSpace") == namespace {
			eciId = cg.ContainerGroupId
			break
		}
	}
	if eciId == "" {
		return nil, errors.New(fmt.Sprintf("GetContainerLogs can't find Pod %s-%s", namespace, podName))
	}

	request := eci.CreateDescribeContainerLogRequest()
	request.ContainerGroupId = eciId
	request.ContainerName = containerName
	request.Tail = requests.Integer(opts.Tail)

	// get logs from cg
	logContent := ""
	retry := 10
	for i := 0; i < retry; i++ {
		response, err := p.eciClient.DescribeContainerLog(request)
		if err != nil {
			msg := fmt.Sprint("Error getting container logs, retrying")
			log.G(ctx).WithField("Method", "GetContainerLogs").Info(msg)
			time.Sleep(5000 * time.Millisecond)
		} else {
			logContent = response.Content
			break
		}
	}

	return ioutil.NopCloser(strings.NewReader(logContent)), nil
}

// Get full pod name as defined in the provider context
func (p *ECIProvider) GetPodFullName(namespace string, pod string) string {
	return fmt.Sprintf("%s-%s", namespace, pod)
}

// RunInContainer executes a command in a container in the pod, copying data
// between in/out/err and the container's stdin/stdout/stderr.
func (p *ECIProvider) RunInContainer(ctx context.Context, namespace, podName, containerName string, cmd []string, attach api.AttachIO) error {
	return nil
}

// GetPodStatus returns the status of a pod by name that is running inside ECI
// returns nil if a pod by that name is not found.
func (p *ECIProvider) GetPodStatus(ctx context.Context, namespace, name string) (*v1.PodStatus, error) {
	pod, err := p.GetPod(ctx, namespace, name)
	if err != nil {
		return nil, err
	}

	if pod == nil {
		return nil, nil
	}

	return &pod.Status, nil
}

func (p *ECIProvider) GetCgs() []eci.ContainerGroup {
	cgs := make([]eci.ContainerGroup, 0)
	request := eci.CreateDescribeContainerGroupsRequest()
	for {
		cgsResponse, err := p.eciClient.DescribeContainerGroups(request)
		if err != nil || len(cgsResponse.ContainerGroups) == 0 {
			break
		}
		request.NextToken = cgsResponse.NextToken

		for _, cg := range cgsResponse.ContainerGroups {
			if getECITagValue(&cg, "NodeName") != p.nodeName {
				continue
			}
			cn := getECITagValue(&cg, "ClusterName")
			if cn == "" {
				cn = "default"
			}
			if cn != p.clusterName {
				continue
			}
			cgs = append(cgs, cg)
		}
		if request.NextToken == "" {
			break
		}
	}
	return cgs
}

// GetPods returns a list of all pods known to be running within ECI.
func (p *ECIProvider) GetPods(ctx context.Context) ([]*v1.Pod, error) {
	pods := make([]*v1.Pod, 0)
	for _, cg := range p.GetCgs() {
		c := cg
		pod, err := containerGroupToPod(&c)
		if err != nil {
			msg := fmt.Sprint("error converting container group to pod", cg.ContainerGroupId, err)
			log.G(context.TODO()).WithField("Method", "GetPods").Info(msg)
			continue
		}
		pods = append(pods, pod)
	}
	return pods, nil
}

// Capacity returns a resource list containing the capacity limits set for ECI.
func (p *ECIProvider) Capacity(ctx context.Context) v1.ResourceList {
	return v1.ResourceList{
		"cpu":    resource.MustParse(p.cpu),
		"memory": resource.MustParse(p.memory),
		"pods":   resource.MustParse(p.pods),
	}
}

// NodeConditions returns a list of conditions (Ready, OutOfDisk, etc), for updates to the node status
// within Kubernetes.
func (p *ECIProvider) NodeConditions(ctx context.Context) []v1.NodeCondition {
	// TODO: Make these dynamic and augment with custom ECI specific conditions of interest
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
func (p *ECIProvider) NodeAddresses(ctx context.Context) []v1.NodeAddress {
	// TODO: Make these dynamic and augment with custom ECI specific conditions of interest
	return []v1.NodeAddress{
		{
			Type:    "InternalIP",
			Address: p.internalIP,
		},
	}
}

// NodeDaemonEndpoints returns NodeDaemonEndpoints for the node status
// within Kubernetes.
func (p *ECIProvider) NodeDaemonEndpoints(ctx context.Context) *v1.NodeDaemonEndpoints {
	return &v1.NodeDaemonEndpoints{
		KubeletEndpoint: v1.DaemonEndpoint{
			Port: p.daemonEndpointPort,
		},
	}
}

// OperatingSystem returns the operating system that was provided by the config.
func (p *ECIProvider) OperatingSystem() string {
	return p.operatingSystem
}

func (p *ECIProvider) getImagePullSecrets(pod *v1.Pod) ([]eci.ImageRegistryCredential, error) {
	ips := make([]eci.ImageRegistryCredential, 0, len(pod.Spec.ImagePullSecrets))
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

func readDockerCfgSecret(secret *v1.Secret, ips []eci.ImageRegistryCredential) ([]eci.ImageRegistryCredential, error) {
	var err error
	var authConfigs map[string]AuthConfig
	repoData, ok := secret.Data[string(v1.DockerConfigKey)]

	if !ok {
		return ips, fmt.Errorf("no dockercfg present in secret")
	}

	err = json.Unmarshal(repoData, &authConfigs)
	if err != nil {
		return ips, fmt.Errorf("failed to unmarshal auth config %+v", err)
	}

	for server, authConfig := range authConfigs {
		ips = append(ips, eci.ImageRegistryCredential{
			Password: authConfig.Password,
			Server:   server,
			UserName: authConfig.Username,
		})
	}

	return ips, err
}

func readDockerConfigJSONSecret(secret *v1.Secret, ips []eci.ImageRegistryCredential) ([]eci.ImageRegistryCredential, error) {
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
		ips = append(ips, eci.ImageRegistryCredential{
			Password: authConfig.Password,
			Server:   server,
			UserName: authConfig.Username,
		})
	}

	return ips, err
}

func (p *ECIProvider) getContainers(pod *v1.Pod, init bool) ([]eci.CreateContainer, error) {
	podContainers := pod.Spec.Containers
	if init {
		podContainers = pod.Spec.InitContainers
	}
	containers := make([]eci.CreateContainer, 0, len(podContainers))
	for _, container := range podContainers {
		c := eci.CreateContainer{
			Name:     container.Name,
			Image:    container.Image,
			Commands: append(container.Command, container.Args...),
			Ports:    make([]eci.ContainerPort, 0, len(container.Ports)),
		}

		for _, p := range container.Ports {
			c.Ports = append(c.Ports, eci.ContainerPort{
				Port:     requests.Integer(strconv.FormatInt(int64(p.ContainerPort), 10)),
				Protocol: string(p.Protocol),
			})
		}

		c.VolumeMounts = make([]eci.VolumeMount, 0, len(container.VolumeMounts))
		for _, v := range container.VolumeMounts {
			c.VolumeMounts = append(c.VolumeMounts, eci.VolumeMount{
				Name:      v.Name,
				MountPath: v.MountPath,
				ReadOnly:  requests.Boolean(strconv.FormatBool(v.ReadOnly)),
			})
		}

		c.EnvironmentVars = make([]eci.EnvironmentVar, 0, len(container.Env))
		for _, e := range container.Env {
			c.EnvironmentVars = append(c.EnvironmentVars, eci.EnvironmentVar{Key: e.Name, Value: e.Value})
		}

		cpuRequest := 1.00
		if _, ok := container.Resources.Requests[v1.ResourceCPU]; ok {
			cpuRequest = float64(container.Resources.Requests.Cpu().MilliValue()) / 1000.00
		}

		c.Cpu = requests.Float(fmt.Sprintf("%.3f", cpuRequest))

		memoryRequest := 2.0
		if _, ok := container.Resources.Requests[v1.ResourceMemory]; ok {
			memoryRequest = float64(container.Resources.Requests.Memory().Value()) / 1024.0 / 1024.0 / 1024.0
		}

		c.Memory = requests.Float(fmt.Sprintf("%.3f", memoryRequest))

		c.ImagePullPolicy = string(container.ImagePullPolicy)
		c.WorkingDir = container.WorkingDir

		containers = append(containers, c)
	}
	return containers, nil
}

func (p *ECIProvider) getVolumes(pod *v1.Pod) ([]eci.Volume, error) {
	volumes := make([]eci.Volume, 0, len(pod.Spec.Volumes))
	for _, v := range pod.Spec.Volumes {
		// Handle the case for the EmptyDir.
		if v.EmptyDir != nil {
			volumes = append(volumes, eci.Volume{
				Type:                 eci.VOL_TYPE_EMPTYDIR,
				Name:                 v.Name,
				EmptyDirVolumeEnable: requests.Boolean(strconv.FormatBool(true)),
			})
			continue
		}

		// Handle the case for the NFS.
		if v.NFS != nil {
			volumes = append(volumes, eci.Volume{
				Type:              eci.VOL_TYPE_NFS,
				Name:              v.Name,
				NfsVolumeServer:   v.NFS.Server,
				NfsVolumePath:     v.NFS.Path,
				NfsVolumeReadOnly: requests.Boolean(strconv.FormatBool(v.NFS.ReadOnly)),
			})
			continue
		}

		// Handle the case for ConfigMap volume.
		if v.ConfigMap != nil {
			ConfigFileToPaths := make([]eci.ConfigFileToPath, 0)
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

				ConfigFileToPaths = append(ConfigFileToPaths, eci.ConfigFileToPath{Path: k, Content: b.String()})
			}

			if len(ConfigFileToPaths) != 0 {
				volumes = append(volumes, eci.Volume{
					Type:              eci.VOL_TYPE_CONFIGFILEVOLUME,
					Name:              v.Name,
					ConfigFileToPaths: ConfigFileToPaths,
				})
			}
			continue
		}

		if v.Secret != nil {
			ConfigFileToPaths := make([]eci.ConfigFileToPath, 0)
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
				ConfigFileToPaths = append(ConfigFileToPaths, eci.ConfigFileToPath{Path: k, Content: b.String()})
			}

			if len(ConfigFileToPaths) != 0 {
				volumes = append(volumes, eci.Volume{
					Type:              eci.VOL_TYPE_CONFIGFILEVOLUME,
					Name:              v.Name,
					ConfigFileToPaths: ConfigFileToPaths,
				})
			}
			continue
		}

		// If we've made it this far we have found a volume type that isn't supported
		return nil, fmt.Errorf("Pod %s requires volume %s which is of an unsupported type\n", pod.Name, v.Name)
	}

	return volumes, nil
}

func containerGroupToPod(cg *eci.ContainerGroup) (*v1.Pod, error) {
	var podCreationTimestamp, containerStartTime metav1.Time

	CreationTimestamp := getECITagValue(cg, "CreationTimestamp")
	if CreationTimestamp != "" {
		if t, err := time.Parse(podTagTimeFormat, CreationTimestamp); err == nil {
			podCreationTimestamp = metav1.NewTime(t)
		}
	}

	if t, err := time.Parse(timeFormat, cg.Containers[0].CurrentState.StartTime); err == nil {
		containerStartTime = metav1.NewTime(t)
	}

	// Use the Provisioning State if it's not Succeeded,
	// otherwise use the state of the instance.
	eciState := cg.Status

	containers := make([]v1.Container, 0, len(cg.Containers))
	containerStatuses := make([]v1.ContainerStatus, 0, len(cg.Containers))
	for _, c := range cg.Containers {
		container := v1.Container{
			Name:    c.Name,
			Image:   c.Image,
			Command: c.Commands,
			Resources: v1.ResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceCPU:    resource.MustParse(fmt.Sprintf("%.2f", c.Cpu)),
					v1.ResourceMemory: resource.MustParse(fmt.Sprintf("%.1fG", c.Memory)),
				},
			},
		}

		container.Resources.Limits = v1.ResourceList{
			v1.ResourceCPU:    resource.MustParse(fmt.Sprintf("%.2f", c.Cpu)),
			v1.ResourceMemory: resource.MustParse(fmt.Sprintf("%.1fG", c.Memory)),
		}

		containers = append(containers, container)
		containerStatus := v1.ContainerStatus{
			Name:                 c.Name,
			State:                eciContainerStateToContainerState(c.CurrentState),
			LastTerminationState: eciContainerStateToContainerState(c.PreviousState),
			Ready:                eciStateToPodPhase(c.CurrentState.State) == v1.PodRunning,
			RestartCount:         int32(c.RestartCount),
			Image:                c.Image,
			ImageID:              "",
			ContainerID:          getContainerID(cg.ContainerGroupId, c.Name),
		}

		// Add to containerStatuses
		containerStatuses = append(containerStatuses, containerStatus)
	}

	pod := v1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:              getECITagValue(cg, "PodName"),
			Namespace:         getECITagValue(cg, "NameSpace"),
			ClusterName:       getECITagValue(cg, "ClusterName"),
			UID:               types.UID(getECITagValue(cg, "UID")),
			CreationTimestamp: podCreationTimestamp,
		},
		Spec: v1.PodSpec{
			NodeName:   getECITagValue(cg, "NodeName"),
			Volumes:    []v1.Volume{},
			Containers: containers,
		},
		Status: v1.PodStatus{
			Phase:             eciStateToPodPhase(eciState),
			Conditions:        eciStateToPodConditions(eciState, podCreationTimestamp),
			Message:           "",
			Reason:            "",
			HostIP:            "",
			PodIP:             cg.IntranetIp,
			StartTime:         &containerStartTime,
			ContainerStatuses: containerStatuses,
		},
	}

	return &pod, nil
}

func getContainerID(cgID, containerName string) string {
	if cgID == "" {
		return ""
	}

	containerResourceID := fmt.Sprintf("%s/containers/%s", cgID, containerName)

	h := sha256.New()
	h.Write([]byte(strings.ToUpper(containerResourceID)))
	hashBytes := h.Sum(nil)
	return fmt.Sprintf("eci://%s", hex.EncodeToString(hashBytes))
}

func eciStateToPodPhase(state string) v1.PodPhase {
	switch state {
	case "Scheduling":
		return v1.PodPending
	case "ScheduleFailed":
		return v1.PodFailed
	case "Pending":
		return v1.PodPending
	case "Running":
		return v1.PodRunning
	case "Failed":
		return v1.PodFailed
	case "Succeeded":
		return v1.PodSucceeded
	}
	return v1.PodUnknown
}

func eciStateToPodConditions(state string, transitionTime metav1.Time) []v1.PodCondition {
	switch state {
	case "Running", "Succeeded":
		return []v1.PodCondition{
			v1.PodCondition{
				Type:               v1.PodReady,
				Status:             v1.ConditionTrue,
				LastTransitionTime: transitionTime,
			}, v1.PodCondition{
				Type:               v1.PodInitialized,
				Status:             v1.ConditionTrue,
				LastTransitionTime: transitionTime,
			}, v1.PodCondition{
				Type:               v1.PodScheduled,
				Status:             v1.ConditionTrue,
				LastTransitionTime: transitionTime,
			},
		}
	}
	return []v1.PodCondition{}
}

func eciContainerStateToContainerState(cs eci.ContainerState) v1.ContainerState {
	t1, err := time.Parse(timeFormat, cs.StartTime)
	if err != nil {
		return v1.ContainerState{}
	}

	startTime := metav1.NewTime(t1)

	// Handle the case where the container is running.
	if cs.State == "Running" || cs.State == "Succeeded" {
		return v1.ContainerState{
			Running: &v1.ContainerStateRunning{
				StartedAt: startTime,
			},
		}
	}

	t2, err := time.Parse(timeFormat, cs.FinishTime)
	if err != nil {
		return v1.ContainerState{}
	}

	finishTime := metav1.NewTime(t2)

	// Handle the case where the container failed.
	if cs.State == "Failed" || cs.State == "Canceled" {
		return v1.ContainerState{
			Terminated: &v1.ContainerStateTerminated{
				ExitCode:   int32(cs.ExitCode),
				Reason:     cs.State,
				Message:    cs.DetailStatus,
				StartedAt:  startTime,
				FinishedAt: finishTime,
			},
		}
	}

	// Handle the case where the container is pending.
	// Which should be all other eci states.
	return v1.ContainerState{
		Waiting: &v1.ContainerStateWaiting{
			Reason:  cs.State,
			Message: cs.DetailStatus,
		},
	}
}

func getECITagValue(cg *eci.ContainerGroup, key string) string {
	for _, tag := range cg.Tags {
		if tag.Key == key {
			return tag.Value
		}
	}
	return ""
}
