package hypersh

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/docker/go-connections/tlsconfig"
	"github.com/hyperhq/hyper-api/types"
	"github.com/hyperhq/hyper-api/types/container"
	registrytypes "github.com/hyperhq/hyper-api/types/registry"
	"github.com/hyperhq/hypercli/cliconfig"
	"github.com/hyperhq/hypercli/opts"
	"github.com/hyperhq/hypercli/pkg/jsonmessage"
	"github.com/hyperhq/hypercli/pkg/term"
	"github.com/hyperhq/hypercli/reference"
	"github.com/hyperhq/hypercli/registry"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (p *HyperProvider) getContainers(pod *v1.Pod) ([]container.Config, []container.HostConfig, error) {
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

func (p *HyperProvider) containerJSONToPod(container *types.ContainerJSON) (*v1.Pod, error) {
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
	startedAt, err := time.Parse(time.RFC3339, container.State.StartedAt)
	if err != nil {
		return nil, fmt.Errorf("parse StartedAt time failed:%v", container.State.StartedAt)
	}
	finishedAt, err := time.Parse(time.RFC3339, container.State.FinishedAt)
	if err != nil {
		return nil, fmt.Errorf("parse FinishedAt time failed:%v", container.State.FinishedAt)
	}

	var (
		podCondition   v1.PodCondition
		containerState v1.ContainerState
	)
	switch p.hyperStateToPodPhase(container.State.Status) {
	case v1.PodPending:
		podCondition = v1.PodCondition{
			Type:   v1.PodInitialized,
			Status: v1.ConditionFalse,
		}
		containerState = v1.ContainerState{
			Waiting: &v1.ContainerStateWaiting{},
		}
	case v1.PodRunning: // running
		podCondition = v1.PodCondition{
			Type:   v1.PodReady,
			Status: v1.ConditionTrue,
		}
		containerState = v1.ContainerState{
			Running: &v1.ContainerStateRunning{
				StartedAt: metav1.NewTime(startedAt),
			},
		}
	case v1.PodSucceeded: // normal exit
		podCondition = v1.PodCondition{
			Type:   v1.PodReasonUnschedulable,
			Status: v1.ConditionFalse,
		}
		containerState = v1.ContainerState{
			Terminated: &v1.ContainerStateTerminated{
				ExitCode:   int32(container.State.ExitCode),
				FinishedAt: metav1.NewTime(finishedAt),
			},
		}
	case v1.PodFailed: // exit with error
		podCondition = v1.PodCondition{
			Type:   v1.PodReasonUnschedulable,
			Status: v1.ConditionFalse,
		}
		containerState = v1.ContainerState{
			Terminated: &v1.ContainerStateTerminated{
				ExitCode:   int32(container.State.ExitCode),
				FinishedAt: metav1.NewTime(finishedAt),
				Reason:     container.State.Error,
			},
		}
	default: //unkown
		podCondition = v1.PodCondition{
			Type:   v1.PodReasonUnschedulable,
			Status: v1.ConditionUnknown,
		}
		containerState = v1.ContainerState{}
	}

	pod := v1.Pod{
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
			Phase:      p.hyperStateToPodPhase(container.State.Status),
			Conditions: []v1.PodCondition{podCondition},
			Message:    "",
			Reason:     "",
			HostIP:     "",
			PodIP:      container.NetworkSettings.IPAddress,
			ContainerStatuses: []v1.ContainerStatus{
				{
					Name:         podName,
					RestartCount: int32(container.RestartCount),
					Image:        container.Config.Image,
					ImageID:      container.Image,
					ContainerID:  container.ID,
					Ready:        container.State.Running,
					State:        containerState,
				},
			},
		},
	}
	return &pod, nil
}

func (p *HyperProvider) containerToPod(container *types.Container) (*v1.Pod, error) {
	// TODO: convert containers into pods
	podName, found := container.Labels[containerLabel]
	if !found {
		return nil, fmt.Errorf("can not found podName: key %q not found in container label", containerLabel)
	}

	nodeName, found := container.Labels[nodeLabel]
	if !found {
		return nil, fmt.Errorf("can not found nodeName: key %q not found in container label", containerLabel)
	}

	var (
		podCondition v1.PodCondition
		isReady      bool = true
	)
	if strings.ToLower(string(container.State)) == strings.ToLower(string(v1.PodRunning)) {
		podCondition = v1.PodCondition{
			Type:   v1.PodReady,
			Status: v1.ConditionTrue,
		}
	} else {
		podCondition = v1.PodCondition{
			Type:   v1.PodReasonUnschedulable,
			Status: v1.ConditionFalse,
		}
		isReady = false
	}

	pod := v1.Pod{
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
			Conditions: []v1.PodCondition{podCondition},
			Message:    "",
			Reason:     "",
			HostIP:     "",
			PodIP:      "",
			ContainerStatuses: []v1.ContainerStatus{
				{
					Name:        container.Names[0],
					Image:       container.Image,
					ImageID:     container.ImageID,
					ContainerID: container.ID,
					Ready:       isReady,
					State:       v1.ContainerState{},
				},
			},
		},
	}
	return &pod, nil
}

func (p *HyperProvider) hyperStateToPodPhase(state string) v1.PodPhase {
	switch strings.ToLower(state) {
	case "created":
		return v1.PodPending
	case "restarting":
		return v1.PodPending
	case "running":
		return v1.PodRunning
	case "exited":
		return v1.PodSucceeded
	case "paused":
		return v1.PodSucceeded
	case "dead":
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

func (p *HyperProvider) ensureImage(image string) error {
	distributionRef, err := reference.ParseNamed(image)
	if err != nil {
		return err
	}

	if reference.IsNameOnly(distributionRef) {
		distributionRef = reference.WithDefaultTag(distributionRef)
		log.Printf("Using default tag: %s", reference.DefaultTag)
	}

	// Resolve the Repository name from fqn to RepositoryInfo
	repoInfo, err := registry.ParseRepositoryInfo(distributionRef)
	var authConfig types.AuthConfig
	if p.configFile != nil {
		authConfig = p.resolveAuthConfig(p.configFile.AuthConfigs, repoInfo.Index)
	}
	encodedAuth, err := p.encodeAuthToBase64(authConfig)
	if err != nil {
		return err
	}

	options := types.ImagePullOptions{
		RegistryAuth: encodedAuth,
		All:          false,
	}
	responseBody, err := p.hyperClient.ImagePull(context.Background(), distributionRef.String(), options)
	if err != nil {
		return err
	}
	defer responseBody.Close()
	var (
		outFd         uintptr
		isTerminalOut bool
	)
	_, stdout, _ := term.StdStreams()
	if stdout != nil {
		outFd, isTerminalOut = term.GetFdInfo(stdout)
	}
	jsonmessage.DisplayJSONMessagesStream(responseBody, stdout, outFd, isTerminalOut, nil)
	return nil
}

func (p *HyperProvider) resolveAuthConfig(authConfigs map[string]types.AuthConfig, index *registrytypes.IndexInfo) types.AuthConfig {
	configKey := index.Name
	if index.Official {
		configKey = p.electAuthServer()
	}

	// First try the happy case
	if c, found := authConfigs[configKey]; found || index.Official {
		return c
	}

	convertToHostname := func(url string) string {
		stripped := url
		if strings.HasPrefix(url, "http://") {
			stripped = strings.Replace(url, "http://", "", 1)
		} else if strings.HasPrefix(url, "https://") {
			stripped = strings.Replace(url, "https://", "", 1)
		}

		nameParts := strings.SplitN(stripped, "/", 2)

		return nameParts[0]
	}

	// Maybe they have a legacy config file, we will iterate the keys converting
	// them to the new format and testing
	for registry, ac := range authConfigs {
		if configKey == convertToHostname(registry) {
			return ac
		}
	}

	// When all else fails, return an empty auth config
	return types.AuthConfig{}
}

func (p *HyperProvider) electAuthServer() string {
	serverAddress := registry.IndexServer
	if info, err := p.hyperClient.Info(context.Background()); err != nil {
		log.Printf("Warning: failed to get default registry endpoint from daemon (%v). Using system default: %s", err, serverAddress)
	} else {
		serverAddress = info.IndexServerAddress
	}
	return serverAddress
}

// encodeAuthToBase64 serializes the auth configuration as JSON base64 payload
func (p *HyperProvider) encodeAuthToBase64(authConfig types.AuthConfig) (string, error) {
	buf, err := json.Marshal(authConfig)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(buf), nil
}
