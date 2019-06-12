package huawei

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/virtual-kubelet/virtual-kubelet/errdefs"
	"github.com/virtual-kubelet/virtual-kubelet/manager"
	"github.com/virtual-kubelet/virtual-kubelet/node/api"
	"github.com/virtual-kubelet/virtual-kubelet/providers/huawei/auth"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	podAnnotationNamespaceKey      = "virtual-kubelet-namespace"
	podAnnotationPodNameKey        = "virtual-kubelet-podname"
	podAnnotationClusterNameKey    = "virtual-kubelet-clustername"
	podAnnotationUIDkey            = "virtual-kubelet-uid"
	podAnnotationNodeName          = "virtual-kubelet-nodename"
	podAnnotationCreationTimestamp = "virtual-kubelet-creationtimestamp"
)

var defaultApiEndpoint string = "https://cciback.cn-north-1.huaweicloud.com"

// CCIProvider implements the virtual-kubelet provider interface and communicates with Huawei's CCI APIs.
type CCIProvider struct {
	appKey             string
	appSecret          string
	apiEndpoint        string
	region             string
	service            string
	project            string
	internalIP         string
	daemonEndpointPort int32
	nodeName           string
	operatingSystem    string
	client             *Client
	resourceManager    *manager.ResourceManager
	cpu                string
	memory             string
	pods               string
}

// Client represents the client config for Huawei.
type Client struct {
	Signer     auth.Signer
	HTTPClient http.Client
}

// NewCCIProvider creates a new CCI provider.
func NewCCIProvider(config string, rm *manager.ResourceManager, nodeName, operatingSystem string, internalIP string, daemonEndpointPort int32) (*CCIProvider, error) {
	p := CCIProvider{}

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
	if appKey := os.Getenv("CCI_APP_KEP"); appKey != "" {
		p.appKey = appKey
	}
	if p.appKey == "" {
		return nil, errors.New("AppKey can not be empty please set CCI_APP_KEP")
	}
	if appSecret := os.Getenv("CCI_APP_SECRET"); appSecret != "" {
		p.appSecret = appSecret
	}
	if p.appSecret == "" {
		return nil, errors.New("AppSecret can not be empty please set CCI_APP_SECRET")
	}
	p.client = new(Client)
	p.client.Signer = &auth.SignerHws{
		AppKey:    p.appKey,
		AppSecret: p.appSecret,
		Region:    p.region,
		Service:   p.service,
	}
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	p.client.HTTPClient = http.Client{
		Transport: tr,
	}
	p.resourceManager = rm
	p.apiEndpoint = defaultApiEndpoint
	p.nodeName = nodeName
	p.operatingSystem = operatingSystem
	p.internalIP = internalIP
	p.daemonEndpointPort = daemonEndpointPort

	if err := p.createProject(); err != nil {
		return nil, err
	}
	return &p, nil
}

func (p *CCIProvider) createProject() error {
	// Create the createProject request url
	uri := p.apiEndpoint + "/api/v1/namespaces"
	// build the request
	project := &v1.Namespace{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Namespace",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: p.project,
		},
	}
	var bodyReader io.Reader
	body, err := json.Marshal(project)
	if err != nil {
		return err
	}
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	r, err := http.NewRequest("POST", uri, bodyReader)
	if err != nil {
		return err
	}
	if err = p.signRequest(r); err != nil {
		return fmt.Errorf("Sign the request failed: %v", err)
	}
	_, err = p.client.HTTPClient.Do(r)
	return err
}

func (p *CCIProvider) signRequest(r *http.Request) error {
	r.Header.Add("content-type", "application/json; charset=utf-8")
	if err := p.client.Signer.Sign(r); err != nil {
		return fmt.Errorf("Sign the request failed: %v", err)
	}
	return nil
}

func (p *CCIProvider) setPodAnnotations(pod *v1.Pod) {
	metav1.SetMetaDataAnnotation(&pod.ObjectMeta, podAnnotationNamespaceKey, pod.Namespace)
	metav1.SetMetaDataAnnotation(&pod.ObjectMeta, podAnnotationClusterNameKey, pod.ClusterName)
	metav1.SetMetaDataAnnotation(&pod.ObjectMeta, podAnnotationPodNameKey, pod.Name)
	metav1.SetMetaDataAnnotation(&pod.ObjectMeta, podAnnotationUIDkey, string(pod.UID))
	metav1.SetMetaDataAnnotation(&pod.ObjectMeta, podAnnotationNodeName, pod.Spec.NodeName)
	metav1.SetMetaDataAnnotation(&pod.ObjectMeta, podAnnotationCreationTimestamp, pod.CreationTimestamp.String())
	pod.Namespace = p.project
	pod.Name = pod.Namespace + "-" + pod.Name
	pod.UID = ""
	pod.Spec.NodeName = ""
	pod.CreationTimestamp = metav1.Time{}
}

func (p *CCIProvider) deletePodAnnotations(pod *v1.Pod) error {
	pod.Name = pod.Annotations[podAnnotationPodNameKey]
	pod.Namespace = pod.Annotations[podAnnotationNamespaceKey]
	pod.UID = types.UID(pod.Annotations[podAnnotationUIDkey])
	pod.ClusterName = pod.Annotations[podAnnotationClusterNameKey]
	pod.Spec.NodeName = pod.Annotations[podAnnotationNodeName]
	if pod.Annotations[podAnnotationCreationTimestamp] != "" {
		t, err := time.Parse("2006-01-02 15:04:05.999999999 -0700 MST", pod.Annotations[podAnnotationCreationTimestamp])
		if err != nil {
			return err
		}
		podCreationTimestamp := metav1.NewTime(t)
		pod.CreationTimestamp = podCreationTimestamp
	}
	delete(pod.Annotations, podAnnotationPodNameKey)
	delete(pod.Annotations, podAnnotationNamespaceKey)
	delete(pod.Annotations, podAnnotationUIDkey)
	delete(pod.Annotations, podAnnotationClusterNameKey)
	delete(pod.Annotations, podAnnotationNodeName)
	delete(pod.Annotations, podAnnotationCreationTimestamp)
	pod.Annotations = nil
	return nil
}

// CreatePod takes a Kubernetes Pod and deploys it within the huawei CCI provider.
func (p *CCIProvider) CreatePod(ctx context.Context, pod *v1.Pod) error {
	// Create the createPod request url
	p.setPodAnnotations(pod)
	uri := p.apiEndpoint + "/api/v1/namespaces/" + p.project + "/pods"
	// build the request
	var bodyReader io.Reader
	body, err := json.Marshal(pod)
	if err != nil {
		return err
	}
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	r, err := http.NewRequest("POST", uri, bodyReader)
	if err != nil {
		return err
	}

	if err = p.signRequest(r); err != nil {
		return fmt.Errorf("Sign the request failed: %v", err)
	}
	_, err = p.client.HTTPClient.Do(r)
	return err
}

// UpdatePod takes a Kubernetes Pod and updates it within the huawei CCI provider.
func (p *CCIProvider) UpdatePod(ctx context.Context, pod *v1.Pod) error {
	return nil
}

// DeletePod takes a Kubernetes Pod and deletes it from the huawei CCI provider.
func (p *CCIProvider) DeletePod(ctx context.Context, pod *v1.Pod) error {
	// Create the deletePod request url
	podName := pod.Namespace + "-" + pod.Name
	uri := p.apiEndpoint + "/api/v1/namespaces/" + p.project + "/pods/" + podName
	// build the request
	r, err := http.NewRequest("DELETE", uri, nil)
	if err != nil {
		return err
	}

	if err = p.signRequest(r); err != nil {
		return fmt.Errorf("Sign the request failed: %v", err)
	}
	resp, err := p.client.HTTPClient.Do(r)
	if err != nil {
		return err
	}

	return errorFromResponse(resp)
}

func errorFromResponse(resp *http.Response) error {
	if resp.StatusCode < 400 {
		return nil
	}

	body, _ := ioutil.ReadAll(io.LimitReader(resp.Body, 16*1024))
	err := fmt.Errorf("error during http request, status=%d: %q", resp.StatusCode, string(body))

	switch resp.StatusCode {
	case http.StatusNotFound:
		return errdefs.AsNotFound(err)
	default:
		return err
	}
}

// GetPod retrieves a pod by name from the huawei CCI provider.
func (p *CCIProvider) GetPod(ctx context.Context, namespace, name string) (*v1.Pod, error) {
	// Create the getPod request url
	podName := namespace + "-" + name
	uri := p.apiEndpoint + "/api/v1/namespaces/" + p.project + "/pods/" + podName
	r, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		return nil, fmt.Errorf("Create get POD request failed: %v", err)
	}

	if err = p.signRequest(r); err != nil {
		return nil, fmt.Errorf("Sign the request failed: %v", err)
	}

	resp, err := p.client.HTTPClient.Do(r)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var pod v1.Pod
	if err = json.Unmarshal(body, &pod); err != nil {
		return nil, err
	}
	if err := p.deletePodAnnotations(&pod); err != nil {
		return nil, err
	}
	return &pod, nil
}

// GetContainerLogs retrieves the logs of a container by name from the huawei CCI provider.
func (p *CCIProvider) GetContainerLogs(ctx context.Context, namespace, podName, containerName string, opts api.ContainerLogOpts) (io.ReadCloser, error) {
	return ioutil.NopCloser(strings.NewReader("")), nil
}

// Get full pod name as defined in the provider context
// TODO: Implementation
func (p *CCIProvider) GetPodFullName(namespace string, pod string) string {
	return ""
}

// RunInContainer executes a command in a container in the pod, copying data
// between in/out/err and the container's stdin/stdout/stderr.
// TODO: Implementation
func (p *CCIProvider) RunInContainer(ctx context.Context, namespace, name, container string, cmd []string, attach api.AttachIO) error {
	log.Printf("receive ExecInContainer %q\n", container)
	return nil
}

// GetPodStatus retrieves the status of a pod by name from the huawei CCI provider.
func (p *CCIProvider) GetPodStatus(ctx context.Context, namespace, name string) (*v1.PodStatus, error) {
	pod, err := p.GetPod(ctx, namespace, name)
	if err != nil {
		return nil, err
	}

	if pod == nil {
		return nil, nil
	}

	return &pod.Status, nil
}

// GetPods retrieves a list of all pods running on the huawei CCI provider.
func (p *CCIProvider) GetPods(ctx context.Context) ([]*v1.Pod, error) {
	// Create the getPod request url
	uri := p.apiEndpoint + "/api/v1/namespaces/" + p.project + "/pods"
	r, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		return nil, fmt.Errorf("Create get POD request failed: %v", err)
	}

	if err = p.signRequest(r); err != nil {
		return nil, fmt.Errorf("Sign the request failed: %v", err)
	}
	resp, err := p.client.HTTPClient.Do(r)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var pods []*v1.Pod
	if err = json.Unmarshal(body, &pods); err != nil {
		return nil, err
	}
	for _, pod := range pods {
		if err := p.deletePodAnnotations(pod); err != nil {
			return nil, err
		}
	}
	return pods, nil
}

// Capacity returns a resource list with the capacity constraints of the huawei CCI provider.
func (p *CCIProvider) Capacity(ctx context.Context) v1.ResourceList {
	return v1.ResourceList{
		"cpu":    resource.MustParse(p.cpu),
		"memory": resource.MustParse(p.memory),
		"pods":   resource.MustParse(p.pods),
	}
}

// NodeConditions returns a list of conditions (Ready, OutOfDisk, etc), which is
// polled periodically to update the node status within Kubernetes.
func (p *CCIProvider) NodeConditions(ctx context.Context) []v1.NodeCondition {
	// TODO: Make these dynamic and augment with custom CCI specific conditions of interest
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
func (p *CCIProvider) NodeAddresses(ctx context.Context) []v1.NodeAddress {
	// TODO: Make these dynamic and augment with custom CCI specific conditions of interest
	return []v1.NodeAddress{
		{
			Type:    "InternalIP",
			Address: p.internalIP,
		},
	}
}

// NodeDaemonEndpoints returns NodeDaemonEndpoints for the node status
// within Kubernetes.
func (p *CCIProvider) NodeDaemonEndpoints(ctx context.Context) *v1.NodeDaemonEndpoints {
	return &v1.NodeDaemonEndpoints{
		KubeletEndpoint: v1.DaemonEndpoint{
			Port: p.daemonEndpointPort,
		},
	}
}

// OperatingSystem returns the operating system the huawei CCI provider is for.
func (p *CCIProvider) OperatingSystem() string {
	return p.operatingSystem
}
