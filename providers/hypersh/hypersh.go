package hypersh

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/docker/go-connections/sockets"
	"github.com/docker/go-connections/tlsconfig"
	hyper "github.com/hyperhq/hyper-api/client"
	"github.com/hyperhq/hyper-api/types"
	"github.com/hyperhq/hyper-api/types/container"
	"github.com/hyperhq/hyper-api/types/filters"
	"github.com/hyperhq/hyper-api/types/network"
	"github.com/hyperhq/hypercli/cliconfig"
	"github.com/hyperhq/hypercli/opts"
	"github.com/virtual-kubelet/virtual-kubelet/manager"
	"github.com/virtual-kubelet/virtual-kubelet/providers"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	containers, hostConfigs, err := getContainers(pod)
	if err != nil {
		return err
	}
	// TODO: get registry creds
	// TODO: get volumes

	// Iterate over the containers to create and start them.
	for k, ctr := range containers {
		//one container in a Pod in hyper.sh currently
		containerName := fmt.Sprintf("pod-%s-%s", pod.Name, pod.Spec.Containers[k].Name)

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
	pod, err = containerJSONToPod(&container)
	if err != nil {
		return nil, err
	} else {
		return pod, nil
	}
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
		pod, err := containerToPod(&container)
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
// within Kuberentes.
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

// OperatingSystem returns the operating system for this provider.
// This is a noop to default to Linux for now.
func (p *HyperProvider) OperatingSystem() string {
	return providers.OperatingSystemLinux
}

func getContainers(pod *v1.Pod) ([]container.Config, []container.HostConfig, error) {
	containers := make([]container.Config, len(pod.Spec.Containers))
	hostConfigs := make([]container.HostConfig, len(pod.Spec.Containers))
	for x, ctr := range pod.Spec.Containers {
		// Do container.Config
		var c container.Config
		c.Image = ctr.Image
		c.Cmd = ctr.Command
		ports := map[nat.Port]struct{}{}
		portBindings := nat.PortMap{}
		for _, p := range ctr.Ports {
			//TODO: p.HostPort is 0 by default, but it's invalid in hyper.sh
			if p.HostPort == 0 {
				p.HostPort = p.ContainerPort
			}
			port, err := nat.NewPort(strings.ToLower(string(p.Protocol)), fmt.Sprintf("%d", p.HostPort))
			if err != nil {
				return nil, nil, fmt.Errorf("creating new port in container conversion failed: %v", err)
			}
			ports[port] = struct{}{}

			portBindings[port] = []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: fmt.Sprintf("%d", p.HostPort),
				},
			}
		}
		c.ExposedPorts = ports

		// TODO: do volumes

		envs := make([]string, len(ctr.Env))
		for z, e := range ctr.Env {
			envs[z] = fmt.Sprintf("%s=%s", e.Name, e.Value)
		}
		c.Env = envs

		// Do container.HostConfig
		var hc container.HostConfig
		cpuLimit := ctr.Resources.Limits.Cpu().Value()
		memoryLimit := ctr.Resources.Limits.Memory().Value()

		hc.Resources = container.Resources{
			CPUShares: cpuLimit,
			Memory:    memoryLimit,
		}

		hc.PortBindings = portBindings

		containers[x] = c
		hostConfigs[x] = hc
	}
	return containers, hostConfigs, nil
}

func containerJSONToPod(container *types.ContainerJSON) (*v1.Pod, error) {
	// TODO: convert containers into pods
	podName, found := container.Config.Labels[containerLabel]
	if !found {
		return nil, fmt.Errorf("can not found podName: key %q not found in container label", containerLabel)
	}

	nodeName, found := container.Config.Labels[nodeLabel]
	if !found {
		return nil, fmt.Errorf("can not found nodeName: key %q not found in container label", containerLabel)
	}

	created, err := time.Parse(time.RFC3339, container.Created)
	if err != nil {
		return nil, fmt.Errorf("parse Created time failed:%v", container.Created)
	}

	p := v1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:              podName,
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(created),
		},
		Spec: v1.PodSpec{
			NodeName: nodeName,
			Volumes:  []v1.Volume{},
			Containers: []v1.Container{
				{
					Name:    podName,
					Image:   container.Config.Image,
					Command: container.Config.Cmd,
				},
			},
		},
		Status: v1.PodStatus{
			Phase:      hyperStateToPodPhase(container.State.Status),
			Conditions: []v1.PodCondition{},
			Message:    "",
			Reason:     "",
			HostIP:     "",
			PodIP:      container.NetworkSettings.IPAddress,
			ContainerStatuses: []v1.ContainerStatus{
				{
					Name:         "",
					State:        v1.ContainerState{},
					Ready:        container.State.Running,
					RestartCount: int32(container.RestartCount),
					Image:        container.Config.Image,
					ImageID:      container.Image,
					ContainerID:  container.ID,
				},
			},
		},
	}

	return &p, nil
}

func containerToPod(container *types.Container) (*v1.Pod, error) {
	// TODO: convert containers into pods
	podName, found := container.Labels[containerLabel]
	if !found {
		return nil, fmt.Errorf("can not found podName: key %q not found in container label", containerLabel)
	}

	nodeName, found := container.Labels[nodeLabel]
	if !found {
		return nil, fmt.Errorf("can not found nodeName: key %q not found in container label", containerLabel)
	}

	p := v1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:              podName,
			Namespace:         "default",
			ClusterName:       "",
			UID:               "",
			CreationTimestamp: metav1.NewTime(time.Unix(container.Created, 0)),
		},
		Spec: v1.PodSpec{
			NodeName: nodeName,
			Volumes:  []v1.Volume{},
			Containers: []v1.Container{
				{
					Name:      podName,
					Image:     container.Image,
					Command:   strings.Split(container.Command, " "),
					Resources: v1.ResourceRequirements{},
				},
			},
		},
		Status: v1.PodStatus{
			//Phase:    "",
			Conditions: []v1.PodCondition{},
			Message:    "",
			Reason:     "",
			HostIP:     "",
			PodIP:      "",
			ContainerStatuses: []v1.ContainerStatus{
				{
					Name:        container.Names[0],
					Ready:       string(container.State) == string(v1.PodRunning),
					Image:       container.Image,
					ImageID:     container.ImageID,
					ContainerID: container.ID,
				},
			},
		},
	}

	return &p, nil
}

func hyperStateToPodPhase(state string) v1.PodPhase {
	switch strings.ToLower(state) {
	case "running":
		return v1.PodRunning
	case "paused":
		return v1.PodSucceeded
	case "restarting":
		return v1.PodPending
	case "created":
		return v1.PodPending
	case "dead":
		return v1.PodFailed
	case "exited":
		return v1.PodFailed
	}
	return v1.PodUnknown
}

func (p *HyperProvider) getServerHost(region string, tlsOptions *tlsconfig.Options) (host string, dft bool, err error) {
	dft = false
	host = region
	if host == "" {
		host = os.Getenv("HYPER_DEFAULT_REGION")
		region = p.getDefaultRegion()
	}
	if _, err := url.ParseRequestURI(host); err != nil {
		host = "tcp://" + region + "." + cliconfig.DefaultHyperEndpoint
		dft = true
	}
	host, err = opts.ParseHost(tlsOptions != nil, host)
	return
}

func (p *HyperProvider) getDefaultRegion() string {
	cc, ok := p.configFile.CloudConfig[cliconfig.DefaultHyperFormat]
	if ok && cc.Region != "" {
		return cc.Region
	}
	return cliconfig.DefaultHyperRegion
}
