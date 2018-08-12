package hypersh

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/virtual-kubelet/virtual-kubelet/manager"
	"github.com/virtual-kubelet/virtual-kubelet/providers"

	"github.com/docker/go-connections/sockets"
	"github.com/docker/go-connections/tlsconfig"
	hyper "github.com/hyperhq/hyper-api/client"
	"github.com/hyperhq/hyper-api/types"
	"github.com/hyperhq/hyper-api/types/filters"
	"github.com/hyperhq/hyper-api/types/network"
	"github.com/hyperhq/hypercli/cliconfig"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/remotecommand"
)

var host = "tcp://*.hyper.sh:443"

const (
	verStr            = "v1.23"
	containerLabel    = "hyper-virtual-kubelet"
	nodeLabel         = containerLabel + "-node"
	instanceTypeLabel = "sh_hyper_instancetype"
)

// HyperProvider implements the virtual-kubelet provider interface and communicates with hyper.sh APIs.
type HyperProvider struct {
	hyperClient     *hyper.Client
	configFile      *cliconfig.ConfigFile
	resourceManager *manager.ResourceManager
	nodeName        string
	operatingSystem string
	region          string
	host            string
	accessKey       string
	secretKey       string
	cpu             string
	memory          string
	instanceType    string
	pods            string
}

// NewHyperProvider creates a new HyperProvider
func NewHyperProvider(config string, rm *manager.ResourceManager, nodeName, operatingSystem string) (*HyperProvider, error) {
	var (
		p          HyperProvider
		err        error
		host       string
		dft        bool
		tlsOptions = &tlsconfig.Options{InsecureSkipVerify: false}
	)

	p.resourceManager = rm

	// Get config from environment variable
	if h := os.Getenv("HYPER_HOST"); h != "" {
		p.host = h
	}
	if ak := os.Getenv("HYPER_ACCESS_KEY"); ak != "" {
		p.accessKey = ak
	}
	if sk := os.Getenv("HYPER_SECRET_KEY"); sk != "" {
		p.secretKey = sk
	}
	if p.host == "" {
		// ignore HYPER_DEFAULT_REGION when HYPER_HOST was specified
		if r := os.Getenv("HYPER_DEFAULT_REGION"); r != "" {
			p.region = r
		}
	}
	if it := os.Getenv("HYPER_INSTANCE_TYPE"); it != "" {
		p.instanceType = it
	} else {
		p.instanceType = "s4"
	}

	if p.accessKey != "" || p.secretKey != "" {
		//use environment variable
		if p.accessKey == "" || p.secretKey == "" {
			return nil, fmt.Errorf("WARNING: Need to specify HYPER_ACCESS_KEY and HYPER_SECRET_KEY at the same time.")
		}
		log.Printf("Use AccessKey and SecretKey from HYPER_ACCESS_KEY and HYPER_SECRET_KEY")
		if p.region == "" {
			p.region = cliconfig.DefaultHyperRegion
		}
		if p.host == "" {
			host, _, err = p.getServerHost(p.region, tlsOptions)
			if err != nil {
				return nil, err
			}
			p.host = host
		}
	} else {
		// use config file, default path is ~/.hyper
		if config == "" {
			config = cliconfig.ConfigDir()
		}
		configFile, err := cliconfig.Load(config)
		if err != nil {
			return nil, fmt.Errorf("WARNING: Error loading config file %q: %v\n", config, err)
		}
		p.configFile = configFile
		log.Printf("config file under %q was loaded\n", config)

		if p.host == "" {
			host, dft, err = p.getServerHost(p.region, tlsOptions)
			if err != nil {
				return nil, err
			}
			p.host = host
		}
		// Get Region, AccessKey and SecretKey from config file
		cc, ok := configFile.CloudConfig[p.host]
		if !ok {
			cc, ok = configFile.CloudConfig[cliconfig.DefaultHyperFormat]
		}
		if ok {
			p.accessKey = cc.AccessKey
			p.secretKey = cc.SecretKey

			if p.region == "" && dft {
				if p.region = cc.Region; p.region == "" {
					p.region = p.getDefaultRegion()
				}
			}
			if !dft {
				if p.region = cc.Region; p.region == "" {
					p.region = cliconfig.DefaultHyperRegion
				}
			}
		} else {
			return nil, fmt.Errorf("WARNING: can not find entrypoint %q in config file", cliconfig.DefaultHyperFormat)
		}
		if p.accessKey == "" || p.secretKey == "" {
			return nil, fmt.Errorf("WARNING: AccessKey or SecretKey is empty in config %q", config)
		}
	}

	log.Printf("\n Host: %s\n AccessKey: %s**********\n SecretKey: %s**********\n InstanceType: %s\n", p.host, p.accessKey[0:1], p.secretKey[0:1], p.instanceType)
	httpClient, err := newHTTPClient(p.host, tlsOptions)

	customHeaders := map[string]string{}
	ver := "0.1"
	customHeaders["User-Agent"] = fmt.Sprintf("Virtual-Kubelet-Client/%s (%s)", ver, runtime.GOOS)

	p.operatingSystem = operatingSystem
	p.nodeName = nodeName

	p.hyperClient, err = hyper.NewClient(p.host, verStr, httpClient, customHeaders, p.accessKey, p.secretKey, p.region)
	if err != nil {
		return nil, err
	}
	//test connect to hyper.sh
	_, err = p.hyperClient.Info(context.Background())
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func newHTTPClient(host string, tlsOptions *tlsconfig.Options) (*http.Client, error) {
	if tlsOptions == nil {
		// let the api client configure the default transport.
		return nil, nil
	}

	config, err := tlsconfig.Client(*tlsOptions)
	if err != nil {
		return nil, err
	}
	tr := &http.Transport{
		TLSClientConfig: config,
	}
	proto, addr, _, err := hyper.ParseHost(host)
	if err != nil {
		return nil, err
	}

	sockets.ConfigureTransport(tr, proto, addr)

	return &http.Client{
		Transport: tr,
	}, nil
}

// CreatePod accepts a Pod definition and creates
// a hyper.sh deployment
func (p *HyperProvider) CreatePod(pod *v1.Pod) error {
	log.Printf("receive CreatePod %q\n", pod.Name)

	//Ignore daemonSet Pod
	if pod != nil && pod.OwnerReferences != nil && len(pod.OwnerReferences) != 0 && pod.OwnerReferences[0].Kind == "DaemonSet" {
		log.Printf("Skip to create DaemonSet pod %q\n", pod.Name)
		return nil
	}

	// Get containers
	containers, hostConfigs, err := p.getContainers(pod)
	if err != nil {
		return err
	}

	// TODO: get volumes

	// Iterate over the containers to create and start them.
	for k, ctr := range containers {
		//one container in a Pod in hyper.sh currently
		containerName := fmt.Sprintf("pod-%s-%s", pod.Name, pod.Spec.Containers[k].Name)

		if err = p.ensureImage(ctr.Image); err != nil {
			return err
		}

		// Add labels to the pod containers.
		ctr.Labels = map[string]string{
			containerLabel:    pod.Name,
			nodeLabel:         p.nodeName,
			instanceTypeLabel: p.instanceType,
		}
		hostConfigs[k].NetworkMode = "bridge"

		// Create the container.
		resp, err := p.hyperClient.ContainerCreate(context.Background(), &ctr, &hostConfigs[k], &network.NetworkingConfig{}, containerName)
		if err != nil {
			return err
		}
		log.Printf("container %q for pod %q was created\n", resp.ID, pod.Name)

		// Iterate throught the warnings.
		for _, warning := range resp.Warnings {
			log.Printf("warning while creating container %q for pod %q: %s", containerName, pod.Name, warning)
		}

		// Start the container.
		if err := p.hyperClient.ContainerStart(context.Background(), resp.ID, ""); err != nil {
			return err
		}
		log.Printf("container %q for pod %q was started\n", resp.ID, pod.Name)
	}
	return nil
}

// UpdatePod is a noop, hyper.sh currently does not support live updates of a pod.
func (p *HyperProvider) UpdatePod(pod *v1.Pod) error {
	return nil
}

// DeletePod deletes the specified pod out of hyper.sh.
func (p *HyperProvider) DeletePod(pod *v1.Pod) (err error) {
	log.Printf("receive DeletePod %q\n", pod.Name)
	var (
		containerName = fmt.Sprintf("pod-%s-%s", pod.Name, pod.Name)
		container     types.ContainerJSON
	)
	// Inspect hyper container
	container, err = p.hyperClient.ContainerInspect(context.Background(), containerName)
	if err != nil {
		return err
	}
	// Check container label
	if v, ok := container.Config.Labels[containerLabel]; ok {
		// Check value of label
		if v != pod.Name {
			return fmt.Errorf("the label %q of hyper container %q should be %q, but it's %q currently", containerLabel, container.Name, pod.Name, v)
		}
		rmOptions := types.ContainerRemoveOptions{
			RemoveVolumes: true,
			Force:         true,
		}
		// Delete hyper container
		resp, err := p.hyperClient.ContainerRemove(context.Background(), container.ID, rmOptions)
		if err != nil {
			return err
		}
		// Iterate throught the warnings.
		for _, warning := range resp {
			log.Printf("warning while deleting container %q for pod %q: %s", container.ID, pod.Name, warning)
		}
		log.Printf("container %q for pod %q was deleted\n", container.ID, pod.Name)
	} else {
		return fmt.Errorf("hyper container %q has no label %q", container.Name, containerLabel)
	}
	return nil
}

// GetPod returns a pod by name that is running inside hyper.sh
// returns nil if a pod by that name is not found.
func (p *HyperProvider) GetPod(namespace, name string) (pod *v1.Pod, err error) {
	var (
		containerName = fmt.Sprintf("pod-%s-%s", name, name)
		container     types.ContainerJSON
	)
	// Inspect hyper container
	container, err = p.hyperClient.ContainerInspect(context.Background(), containerName)
	if err != nil {
		return nil, err
	}
	// Convert hyper container into Pod
	pod, err = p.containerJSONToPod(&container)
	if err != nil {
		return nil, err
	} else {
		return pod, nil
	}
}

// GetContainerLogs retrieves the logs of a container by name from the provider.
func (p *HyperProvider) GetContainerLogs(namespace, podName, containerName string, tail int) (string, error) {
	return "", nil
}

// Get full pod name as defined in the provider context
// TODO: Implementation
func (p *HyperProvider) GetPodFullName(namespace string, pod string) string {
	return ""
}

// ExecInContainer executes a command in a container in the pod, copying data
// between in/out/err and the container's stdin/stdout/stderr.
// TODO: Implementation
func (p *HyperProvider) ExecInContainer(name string, uid apitypes.UID, container string, cmd []string, in io.Reader, out, err io.WriteCloser, tty bool, resize <-chan remotecommand.TerminalSize, timeout time.Duration) error {
	log.Printf("receive ExecInContainer %q\n", container)
	return nil
}

// GetPodStatus returns the status of a pod by name that is running inside hyper.sh
// returns nil if a pod by that name is not found.
func (p *HyperProvider) GetPodStatus(namespace, name string) (*v1.PodStatus, error) {
	pod, err := p.GetPod(namespace, name)
	if err != nil {
		return nil, err
	}
	return &pod.Status, nil
}

// GetPods returns a list of all pods known to be running within hyper.sh.
func (p *HyperProvider) GetPods() ([]*v1.Pod, error) {
	log.Printf("receive GetPods\n")
	filter, err := filters.FromParam(fmt.Sprintf("{\"label\":{\"%s=%s\":true}}", nodeLabel, p.nodeName))
	if err != nil {
		return nil, err
	}
	// Filter by label.
	containers, err := p.hyperClient.ContainerList(context.Background(), types.ContainerListOptions{
		Filter: filter,
		All:    true,
	})
	if err != nil {
		return nil, err
	}
	log.Printf("found %d pods\n", len(containers))

	var pods = []*v1.Pod{}
	for _, container := range containers {
		pod, err := p.containerToPod(&container)
		if err != nil {
			log.Printf("WARNING: convert container %q to pod error: %v\n", container.ID, err)
			continue
		}
		pods = append(pods, pod)
	}
	return pods, nil
}

// Capacity returns a resource list containing the capacity limits set for hyper.sh.
func (p *HyperProvider) Capacity() v1.ResourceList {
	// TODO: These should be configurable
	return v1.ResourceList{
		"cpu":    resource.MustParse("20"),
		"memory": resource.MustParse("100Gi"),
		"pods":   resource.MustParse("20"),
	}
}

// NodeConditions returns a list of conditions (Ready, OutOfDisk, etc), for updates to the node status
// within Kubernetes.
func (p *HyperProvider) NodeConditions() []v1.NodeCondition {
	// TODO: Make these dynamic and augment with custom hyper.sh specific conditions of interest
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
func (p *HyperProvider) NodeAddresses() []v1.NodeAddress {
	return nil
}

// NodeDaemonEndpoints returns NodeDaemonEndpoints for the node status
// within Kubernetes.
func (p *HyperProvider) NodeDaemonEndpoints() *v1.NodeDaemonEndpoints {
	return nil
}

// OperatingSystem returns the operating system for this provider.
// This is a noop to default to Linux for now.
func (p *HyperProvider) OperatingSystem() string {
	return providers.OperatingSystemLinux
}

// Labels returns provider specific labels
func (p *HyperProvider) Labels() map[string]string {
	return nil
}
