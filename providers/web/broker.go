// Package web provides an implementation of the virtual kubelet provider interface
// by forwarding all calls to a web endpoint. The web endpoint to which requests
// must be forwarded must be specified through an environment variable called
// `WEB_ENDPOINT_URL`. This endpoint must implement the following HTTP APIs:
//  - POST /createPod
//  - PUT /updatePod
//  - DELETE /deletePod
//  - GET /getPod?namespace=[namespace]&name=[pod name]
//  - GE /getContainerLogs?namespace=[namespace]&podName=[pod name]&containerName=[container name]&tail=[tail value]
//  - GET /getPodStatus?namespace=[namespace]&name=[pod name]
//  - GET /getPods
//  - GET /capacity
//  - GET /nodeConditions
//  - GET /nodeAddresses
package web

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/cenkalti/backoff"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/remotecommand"
)

// BrokerProvider implements the virtual-kubelet provider interface by forwarding kubelet calls to a web endpoint.
type BrokerProvider struct {
	nodeName           string
	operatingSystem    string
	endpoint           *url.URL
	client             *http.Client
	daemonEndpointPort int32
}

// NewBrokerProvider creates a new BrokerProvider
func NewBrokerProvider(nodeName, operatingSystem string, daemonEndpointPort int32) (*BrokerProvider, error) {
	var provider BrokerProvider

	provider.nodeName = nodeName
	provider.operatingSystem = operatingSystem
	provider.client = &http.Client{}
	provider.daemonEndpointPort = daemonEndpointPort

	if ep := os.Getenv("WEB_ENDPOINT_URL"); ep != "" {
		epurl, err := url.Parse(ep)
		if err != nil {
			return nil, err
		}
		provider.endpoint = epurl
	}

	return &provider, nil
}

// CreatePod accepts a Pod definition and forwards the call to the web endpoint
func (p *BrokerProvider) CreatePod(pod *v1.Pod) error {
	return p.createUpdatePod(pod, "POST", "/createPod")
}

// UpdatePod accepts a Pod definition and forwards the call to the web endpoint
func (p *BrokerProvider) UpdatePod(pod *v1.Pod) error {
	return p.createUpdatePod(pod, "PUT", "/updatePod")
}

// DeletePod accepts a Pod definition and forwards the call to the web endpoint
func (p *BrokerProvider) DeletePod(pod *v1.Pod) error {
	urlPath, err := url.Parse("/deletePod")
	if err != nil {
		return err
	}

	// encode pod definition as JSON and post request
	podJSON, err := json.Marshal(pod)
	if err != nil {
		return err
	}

	_, err = p.doRequest("DELETE", urlPath, podJSON, false)
	return err
}

// GetPod returns a pod by name that is being managed by the web server
func (p *BrokerProvider) GetPod(namespace, name string) (*v1.Pod, error) {
	urlPathStr := fmt.Sprintf(
		"/getPod?namespace=%s&name=%s",
		url.QueryEscape(namespace),
		url.QueryEscape(name))

	var pod v1.Pod
	err := p.doGetRequest(urlPathStr, &pod)

	// if we get a "404 Not Found" then we return nil to indicate that no pod
	// with this name was found
	if err != nil && err.Error() == "404 Not Found" {
		return nil, nil
	}

	return &pod, err
}

// GetContainerLogs returns the logs of a container running in a pod by name.
func (p *BrokerProvider) GetContainerLogs(namespace, podName, containerName string, tail int) (string, error) {
	urlPathStr := fmt.Sprintf(
		"/getContainerLogs?namespace=%s&podName=%s&containerName=%s&tail=%d",
		url.QueryEscape(namespace),
		url.QueryEscape(podName),
		url.QueryEscape(containerName),
		tail)

	response, err := p.doGetRequestBytes(urlPathStr)
	if err != nil {
		return "", err
	}

	return string(response), nil
}

// Get full pod name as defined in the provider context
// TODO: Implementation
func (p *BrokerProvider) GetPodFullName(namespace string, pod string) string {
	return ""
}

// ExecInContainer executes a command in a container in the pod, copying data
// between in/out/err and the container's stdin/stdout/stderr.
// TODO: Implementation
func (p *BrokerProvider) ExecInContainer(name string, uid types.UID, container string, cmd []string, in io.Reader, out, err io.WriteCloser, tty bool, resize <-chan remotecommand.TerminalSize, timeout time.Duration) error {
	log.Printf("receive ExecInContainer %q\n", container)
	return nil
}

// GetPodStatus retrieves the status of a given pod by name.
func (p *BrokerProvider) GetPodStatus(namespace, name string) (*v1.PodStatus, error) {
	urlPathStr := fmt.Sprintf(
		"/getPodStatus?namespace=%s&name=%s",
		url.QueryEscape(namespace),
		url.QueryEscape(name))

	var podStatus v1.PodStatus
	err := p.doGetRequest(urlPathStr, &podStatus)

	// if we get a "404 Not Found" then we return nil to indicate that no pod
	// with this name was found
	if err != nil && err.Error() == "404 Not Found" {
		return nil, nil
	}

	return &podStatus, err
}

// GetPods retrieves a list of all pods scheduled to run.
func (p *BrokerProvider) GetPods() ([]*v1.Pod, error) {
	var pods []*v1.Pod
	err := p.doGetRequest("/getPods", &pods)

	return pods, err
}

// Capacity returns a resource list containing the capacity limits
func (p *BrokerProvider) Capacity() v1.ResourceList {
	var resourceList v1.ResourceList
	err := p.doGetRequest("/capacity", &resourceList)

	// TODO: This API should support reporting an error.
	if err != nil {
		panic(err)
	}

	return resourceList
}

// NodeConditions returns a list of conditions (Ready, OutOfDisk, etc), for updates to the node status
func (p *BrokerProvider) NodeConditions() []v1.NodeCondition {
	var nodeConditions []v1.NodeCondition
	err := p.doGetRequest("/nodeConditions", &nodeConditions)

	// TODO: This API should support reporting an error.
	if err != nil {
		panic(err)
	}

	return nodeConditions
}

// NodeAddresses returns a list of addresses for the node status
// within Kubernetes.
func (p *BrokerProvider) NodeAddresses() []v1.NodeAddress {
	var nodeAddresses []v1.NodeAddress
	err := p.doGetRequest("/nodeAddresses", &nodeAddresses)

	// TODO: This API should support reporting an error.
	if err != nil {
		panic(err)
	}

	return nodeAddresses
}

// NodeDaemonEndpoints returns NodeDaemonEndpoints for the node status
// within Kubernetes.
func (p *BrokerProvider) NodeDaemonEndpoints() *v1.NodeDaemonEndpoints {
	return &v1.NodeDaemonEndpoints{
		KubeletEndpoint: v1.DaemonEndpoint{
			Port: p.daemonEndpointPort,
		},
	}
}

// OperatingSystem returns the operating system for this provider.
func (p *BrokerProvider) OperatingSystem() string {
	return p.operatingSystem
}

func (p *BrokerProvider) doGetRequest(urlPathStr string, v interface{}) error {
	response, err := p.doGetRequestBytes(urlPathStr)
	if err != nil {
		return err
	}

	return json.Unmarshal(response, &v)
}

func (p *BrokerProvider) doGetRequestBytes(urlPathStr string) ([]byte, error) {
	urlPath, err := url.Parse(urlPathStr)
	if err != nil {
		return nil, err
	}

	return p.doRequest("GET", urlPath, nil, true)
}

func (p *BrokerProvider) createUpdatePod(pod *v1.Pod, method, postPath string) error {
	// build the post url
	postPathURL, err := url.Parse(postPath)
	if err != nil {
		return err
	}

	// encode pod definition as JSON and post request
	podJSON, err := json.Marshal(pod)
	if err != nil {
		return err
	}
	_, err = p.doRequest(method, postPathURL, podJSON, false)
	return err
}

func (p *BrokerProvider) doRequest(method string, urlPath *url.URL, body []byte, readResponse bool) ([]byte, error) {
	// build full URL
	requestURL := p.endpoint.ResolveReference(urlPath)

	// build the request
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}
	request, err := http.NewRequest(method, requestURL.String(), bodyReader)
	request.Header.Add("Content-Type", "application/json")

	// issue request
	retry := backoff.NewExponentialBackOff()
	retry.MaxElapsedTime = 5 * time.Minute

	var response *http.Response
	err = backoff.Retry(func() error {
		response, err = p.client.Do(request)
		return err
	}, retry)
	if err != nil {
		return nil, err
	}

	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return nil, errors.New(response.Status)
	}

	// read response body if asked to
	if readResponse {
		return ioutil.ReadAll(response.Body)
	}

	return nil, nil
}
