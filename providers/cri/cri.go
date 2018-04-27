package cri

import (
	"log"
	"time"
	"os"
	"path/filepath"
	"bufio"
	"strings"
	"io/ioutil"
	"runtime"
	"syscall"

	"fmt"

	"github.com/virtual-kubelet/virtual-kubelet/providers"
	"github.com/virtual-kubelet/virtual-kubelet/manager"
	"google.golang.org/grpc"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
        k8serr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	criapi "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
	"k8s.io/kubernetes/pkg/kubelet/util"
)

// CRIProvider implements the virtual-kubelet provider interface and stores pods in memory.
type CRIProvider struct {
	resourceManager    *manager.ResourceManager
	podLogRoot         string
	podVolRoot         string
	criSocket          string
	nodeName           string
	operatingSystem    string
	internalIP         string
	daemonEndpointPort int32
	pods               map[string]*v1.Pod
	podStatus          map[types.UID]CRIPod // Indexed by Pod Spec UID
	runtimeClient      criapi.RuntimeServiceClient
	imageClient        criapi.ImageServiceClient
}

type CRIPod struct {
	id         string                    // This is the CRI Pod ID, not the UID from the Pod Spec
	containers map[string]*criapi.ContainerStatus // ContainerStatus is a superset of Container, so no need to store both
	status     *criapi.PodSandboxStatus  // PodStatus is a superset of PodSandbox, so no need to store both
}

// Build an internal representation of the state of the pods and containers on the node
// Call this at the start of every function that needs to read any pod or container state
func (p *CRIProvider) refreshNodeState() error {
	allPods, err := getPodSandboxes(p.runtimeClient)
	if err != nil {
		return err
	}

	for _, pod := range allPods {
		psId := pod.Id

		pss, err := getPodSandboxStatus(p.runtimeClient, psId)
		if err != nil {
			return err
		}

		containers, err := getContainersForSandbox(p.runtimeClient, psId)
		if err != nil {
			return err
		}

		var css = make(map[string]*criapi.ContainerStatus)
		for _, c := range containers {
			cstatus, err := getContainerCRIStatus(p.runtimeClient, c.Id)
			if err != nil {
				return err
			}
			css[cstatus.Metadata.Name] = cstatus
		}

		p.podStatus[types.UID(pss.Metadata.Uid)] = CRIPod{
			id:         pod.Id,
			status:     pss,
			containers: css,
		}
	}
	return nil
}

func getClientAPIs(criSocket string) (criapi.RuntimeServiceClient, criapi.ImageServiceClient, error) {
	// Set up a connection to the server.
	conn, err := getClientConnection(criSocket)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect: %v", err)
	}
	rc := criapi.NewRuntimeServiceClient(conn)
	if rc == nil {
		return nil, nil, fmt.Errorf("failed to create runtime service client")
	}
	ic := criapi.NewImageServiceClient(conn)
	if ic == nil {
		return nil, nil, fmt.Errorf("failed to create image service client")
	}
	return rc, ic, err
}

func getClientConnection(criSocket string) (*grpc.ClientConn, error) {
	addr, dialer, err := util.GetAddressAndDialer(criSocket)
	if err != nil {
		return nil, err
	}

	conn, err := grpc.Dial(addr, grpc.WithInsecure(), grpc.WithTimeout(10*time.Second), grpc.WithDialer(dialer))
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %v", err)
	}
	return conn, nil
}

// NewCRIProvider creates a new CRIProvider
func NewCRIProvider(nodeName, operatingSystem string, internalIP string, resourceManager *manager.ResourceManager, daemonEndpointPort int32) (*CRIProvider, error) {
	criSocket := "unix:///run/containerd/containerd.sock"
	runtimeClient, imageClient, err := getClientAPIs(criSocket)
	if err != nil {
		return nil, err
	}
	provider := CRIProvider{
		resourceManager:    resourceManager,
		podLogRoot:         "/var/log/vk-cri/",
		podVolRoot:         "/run/vk-cri/volumes/",
		criSocket:          criSocket,
		nodeName:           nodeName,
		operatingSystem:    operatingSystem,
		internalIP:         internalIP,
		daemonEndpointPort: daemonEndpointPort,
		pods:               make(map[string]*v1.Pod),
		podStatus:          make(map[types.UID]CRIPod),
		runtimeClient:      runtimeClient,
		imageClient:        imageClient,
	}
	err = os.MkdirAll(provider.podLogRoot, 0755)
	if err != nil {
		return nil, err
	}
	err = os.MkdirAll(provider.podVolRoot, 0755)
	if err != nil {
		return nil, err
	}
	return &provider, err
}

func createPodLabels(pod *v1.Pod) map[string]string {
	labels := make(map[string]string)

	for k, v := range pod.Labels {
		labels[k] = v
	}

	// Note: None of the "special" labels appear to have any meaning outside of Kubelet
	return labels
}

func createPodHostname(pod *v1.Pod) string {
	specHostname := pod.Spec.Hostname
//	specDomain := pod.Spec.Subdomain
	if len(specHostname) == 0 {
		specHostname = pod.Spec.NodeName		// TODO: This is what kube-proxy expects. Double-check
	}
//	if len(specDomain) == 0 {
                return specHostname
//      }
	// TODO: Cannot apply the domain until we get the cluster domain from the equivalent of kube-config
	// If specified, the fully qualified Pod hostname will be "<hostname>.<subdomain>.<pod namespace>.svc.<cluster domain>".
	// If not specified, the pod will not have a domainname at all.
//	return fmt.Sprintf("%s.%s.%s.svc.%s", specHostname, specDomain, Pod.Spec.Namespace, //cluster domain)
}

func createPodDnsConfig(pod *v1.Pod) *criapi.DNSConfig {
	return nil	// Use the container engine defaults for now
}

func convertCRIProtocol(in v1.Protocol) criapi.Protocol {
	switch (in) {
	case v1.ProtocolTCP:
		return criapi.Protocol_TCP
	case v1.ProtocolUDP:
		return criapi.Protocol_UDP
	}
	return criapi.Protocol(-1)
}

func createPortMappings(pod *v1.Pod) []*criapi.PortMapping {
	result := []*criapi.PortMapping{}
	for _, c := range pod.Spec.Containers {
		for _, p := range c.Ports {
			result = append(result, &criapi.PortMapping {
				HostPort:      p.HostPort,
				ContainerPort: p.ContainerPort,
				Protocol:      convertCRIProtocol(p.Protocol),
				HostIp:        p.HostIP,
			})
		}
	}
	return result
}

func existsPrivilegedContainerInSpec(pod *v1.Pod) bool {
	for _, c := range pod.Spec.Containers {
		if c.SecurityContext != nil &&
			c.SecurityContext.Privileged != nil &&
			*c.SecurityContext.Privileged {
			return true
		}
	}
	return false
}

func createPodSandboxLinuxConfig(pod *v1.Pod) *criapi.LinuxPodSandboxConfig {
	// TODO: Map values to the fields below
	return &criapi.LinuxPodSandboxConfig{
		CgroupParent: "",
		SecurityContext: &criapi.LinuxSandboxSecurityContext {
			NamespaceOptions: nil, // *NamespaceOption
			SelinuxOptions: nil, //*SELinuxOption
			RunAsUser: nil, // *Int64Value
			RunAsGroup: nil, //*Int64Value
			ReadonlyRootfs: false,
			SupplementalGroups: []int64{},
			Privileged: existsPrivilegedContainerInSpec(pod), 
			SeccompProfilePath: "",
		},
		Sysctls: make(map[string]string),
	}
}

func generatePodSandboxConfig(pod *v1.Pod, logDir string, attempt uint32) (*criapi.PodSandboxConfig, error) {
	podUID := string(pod.UID)
	// TODO: Probably incomplete
	config := &criapi.PodSandboxConfig{
		Metadata: &criapi.PodSandboxMetadata{
			Name:      pod.Name,
			Namespace: pod.Namespace,
			Uid:       podUID,
			Attempt:   attempt,
		},
		Labels:       createPodLabels(pod),
		Annotations:  pod.Annotations,
		LogDirectory: logDir,
		DnsConfig: createPodDnsConfig(pod),
		Hostname: createPodHostname(pod),
		PortMappings: createPortMappings(pod),
		Linux: createPodSandboxLinuxConfig(pod),
	}
	return config, nil
}

func createCtrEnvVars(in []v1.EnvVar) []*criapi.KeyValue {
        out := make([]*criapi.KeyValue, len(in))
        for i := range in {
                e := in[i]
                out[i] = &criapi.KeyValue{
                        Key:   e.Name,
                        Value: e.Value,
                }
        }
	return out
}

func createCtrLabels(container *v1.Container, pod *v1.Pod) map[string]string {
        labels := make(map[string]string)
        // Note: None of the "special" labels appear to have any meaning outside of Kubelet
        return labels
}

func createCtrAnnotations(container *v1.Container, pod *v1.Pod) map[string]string {
        annotations := make(map[string]string)
        // Note: None of the "special" annotations appear to have any meaning outside of Kubelet
        return annotations
}

func findPodVolumeSpec(pod *v1.Pod, name string) *v1.VolumeSource {
	for _, volume := range pod.Spec.Volumes {
		if volume.Name == name {
			return &volume.VolumeSource
		}
	}
	return nil
}

func convertMountPropagationToCRI(input *v1.MountPropagationMode) criapi.MountPropagation {
	if input != nil {
		switch (*input) {
		case v1.MountPropagationHostToContainer:
			return criapi.MountPropagation_PROPAGATION_HOST_TO_CONTAINER
		case v1.MountPropagationBidirectional:
			return criapi.MountPropagation_PROPAGATION_BIDIRECTIONAL
		}
	}
	return criapi.MountPropagation_PROPAGATION_PRIVATE
}

func createCtrMounts(container *v1.Container, pod *v1.Pod, podVolRoot string, rm *manager.ResourceManager) ([]*criapi.Mount, error) {
	mounts := []*criapi.Mount{}
	for _, mountSpec := range container.VolumeMounts {
		podVolSpec := findPodVolumeSpec(pod, mountSpec.Name)
		if podVolSpec == nil {
			fmt.Printf("Container volume mount %s not found in Pod spec", mountSpec.Name)
			continue
		}
		// Common fields to all mount types
		newMount := criapi.Mount {
			ContainerPath: filepath.Join(mountSpec.MountPath, mountSpec.SubPath),
			Readonly: mountSpec.ReadOnly,
			Propagation: convertMountPropagationToCRI(mountSpec.MountPropagation),
		}
		// Iterate over the volume types we care about
		if podVolSpec.HostPath != nil {
			newMount.HostPath = podVolSpec.HostPath.Path
		} else
		if podVolSpec.EmptyDir != nil {
			// TODO: Currently ignores the SizeLimit
			newMount.HostPath = filepath.Join(podVolRoot, mountSpec.Name)
			// TODO: Maybe not the best place to modify the filesystem, but clear enough for now
			err := os.MkdirAll(newMount.HostPath, 0755)
			if err != nil {
				return nil, fmt.Errorf("Error making emptyDir for path %s: %v", newMount.HostPath, err)
			}
		} else
		if podVolSpec.Secret != nil {
			spec := podVolSpec.Secret
			podSecretDir := filepath.Join(podVolRoot, "/secrets", mountSpec.Name)
			newMount.HostPath = podSecretDir
                        err := os.MkdirAll(newMount.HostPath, 0755)
                        if err != nil {
                                return nil, fmt.Errorf("Error making secret dir for path %s: %v", newMount.HostPath, err)
                        }
                        secret, err := rm.GetSecret(spec.SecretName, pod.Namespace)
	                if spec.Optional != nil && !*spec.Optional && k8serr.IsNotFound(err) {
                                return nil, fmt.Errorf("Secret %s is required by Pod %s and does not exist", spec.SecretName, pod.Name)
                        }
		        if err != nil {
				return nil, fmt.Errorf("Error getting secret %s from API server: %v", spec.SecretName, err)
			}
                        if secret == nil {
                                continue
                        }
			// TODO: Check podVolSpec.Secret.Items and map to specified paths
			// TODO: Check podVolSpec.Secret.StringData
			// TODO: What to do with podVolSpec.Secret.SecretType?
                        for k, v := range secret.Data {
				// TODO: Arguably the wrong place to be writing files, but clear enough for now
				// TODO: Ensure that these files are deleted in failure cases
				fullPath := filepath.Join(podSecretDir, k)
				err = ioutil.WriteFile(fullPath, v, 0644)		// Not encoded
				if err != nil {
					return nil, fmt.Errorf("Could not write secret file %s", fullPath)
				}
                        }
		} else
		if podVolSpec.ConfigMap != nil {
			spec := podVolSpec.ConfigMap
			podConfigMapDir := filepath.Join(podVolRoot, "/configmaps", mountSpec.Name)
			newMount.HostPath = podConfigMapDir
                        err := os.MkdirAll(newMount.HostPath, 0755)
                        if err != nil {
                                return nil, fmt.Errorf("Error making configmap dir for path %s: %v", newMount.HostPath, err)
                        }
			configMap, err := rm.GetConfigMap(spec.Name, pod.Namespace)
	                if spec.Optional != nil && !*spec.Optional && k8serr.IsNotFound(err) {
                                return nil, fmt.Errorf("Configmap %s is required by Pod %s and does not exist", spec.Name, pod.Name)
                        }
		        if err != nil {
				return nil, fmt.Errorf("Error getting configmap %s from API server: %v", spec.Name, err)
			}
			if configMap == nil {
				continue
			}
			// TODO: Check podVolSpec.ConfigMap.Items and map to paths
			// TODO: Check podVolSpec.ConfigMap.BinaryData
			for k, v := range configMap.Data {
				// TODO: Arguably the wrong place to be writing files, but clear enough for now
				// TODO: Ensure that these files are deleted in failure cases
				fullPath := filepath.Join(podConfigMapDir, k)
				err = ioutil.WriteFile(fullPath, []byte(v), 0644)
				if err != nil {
					return nil, fmt.Errorf("Could not write configmap file %s")
				}
			}
		} else {
			continue
		}
		mounts = append(mounts, &newMount)
	}
	return mounts, nil
}

func valueOrDefaultBool(input *bool, defVal bool) bool {
	if input != nil {
		return *input
	}
	return defVal
}

func createCtrLinuxConfig(container *v1.Container, pod *v1.Pod) *criapi.LinuxContainerConfig {
	v1sc := container.SecurityContext
	var sc *criapi.LinuxContainerSecurityContext

	if v1sc != nil {
		sc = &criapi.LinuxContainerSecurityContext{
			Capabilities: nil, // *Capability
			Privileged: valueOrDefaultBool(v1sc.Privileged, false),	// No default Pod value
			NamespaceOptions: nil, // *NamespaceOption
			SelinuxOptions: nil, // *SELinuxOption
			RunAsUser: nil, // *Int64Value
			RunAsGroup: nil, // *Int64Value
			RunAsUsername: "",
			ReadonlyRootfs: false,
			SupplementalGroups: []int64{},
			ApparmorProfile: "",
			SeccompProfilePath: "",
			NoNewPrivs: false,
		}
	}
	//psc := pod.Spec.SecurityContext
	return &criapi.LinuxContainerConfig {
		Resources: nil, //*LinuxContainerResources
		SecurityContext: sc,
	}
}

func generateContainerConfig(container *v1.Container, pod *v1.Pod, imageRef, podVolRoot string, rm *manager.ResourceManager, attempt uint32) (*criapi.ContainerConfig, error) {
	// TODO: Probably incomplete
	config := &criapi.ContainerConfig{
		Metadata: &criapi.ContainerMetadata{
			Name:    container.Name,
			Attempt: attempt,
		},
		Image:       &criapi.ImageSpec{Image: imageRef},
		Command:     container.Command,
		Args:        container.Args,
		WorkingDir:  container.WorkingDir,
                Envs:        createCtrEnvVars(container.Env),
		Labels:      createCtrLabels(container, pod),
		Annotations: createCtrAnnotations(container, pod),
		Linux:       createCtrLinuxConfig(container, pod),
		LogPath:     fmt.Sprintf("%s-%d.log", container.Name, attempt),
		Stdin:       container.Stdin,
		StdinOnce:   container.StdinOnce,
		Tty:         container.TTY,
	}
	mounts, err := createCtrMounts(container, pod, podVolRoot, rm)
	if err != nil {
		return nil, err
	}
	config.Mounts = mounts
	return config, nil
}

func (p *CRIProvider) CreatePod(pod *v1.Pod) error {
	log.Printf("receive CreatePod %q\n", pod.Name)

//	fmt.Printf("%v\n", pod)
	var attempt uint32		// TODO: Does this matter?
	logPath := filepath.Join(p.podLogRoot, string(pod.UID))
	volPath := filepath.Join(p.podVolRoot, string(pod.UID))
        err := p.refreshNodeState()
        if err != nil {
                return err
        }
        pConfig, err := generatePodSandboxConfig(pod, logPath, attempt)
        if err != nil {
                return err
        }
	fmt.Printf("%v\n", pConfig)
	existing := p.findPodByName(pod.Namespace, pod.Name)

	// TODO: Is re-using an existing sandbox with the UID the correct behavior?
	// TODO: Should delete the sandbox if container creation fails
	var pId string
	if existing == nil {
		err = os.MkdirAll(logPath, 0755)
		if err != nil {
			return err
		}
		err = os.MkdirAll(volPath, 0755)
		if err != nil {
			return err
		}
		// TODO: Is there a race here?
		pId, err = runPodSandbox(p.runtimeClient, pConfig)
		if err != nil {
			return err
		}
	} else {
		pId = existing.status.Metadata.Uid
	}

	for _, c := range pod.Spec.Containers {
		fmt.Printf("Pulling image %s\n", c.Image)
		imageRef, err := pullImage(p.imageClient, c.Image)
		if err != nil {
			return err
		}
		fmt.Printf("Creating container %s\n", c.Name)
		cConfig, err := generateContainerConfig(&c, pod, imageRef, volPath, p.resourceManager, attempt)
		fmt.Printf("%v\n", cConfig)
		if err != nil {
			return err
		}
		cId, err := createContainer(p.runtimeClient, cConfig, pConfig, pId)
		if err != nil {
			return err
		}
		fmt.Printf("Starting container %s\n", c.Name)
		err = startContainer(p.runtimeClient, cId)
	}

	return err
}

func (p *CRIProvider) UpdatePod(pod *v1.Pod) error {
	log.Printf("receive UpdatePod %q\n", pod.Name)

	return nil
}

func (p *CRIProvider) DeletePod(pod *v1.Pod) error {
	log.Printf("receive DeletePod %q\n", pod.Name)

	err := p.refreshNodeState()
	if err != nil {
		return err
	}

	ps, ok := p.podStatus[pod.UID]
	if !ok {
		return fmt.Errorf("Pod %s not found", pod.UID)
	}

	// TODO: Check pod status for running state
	err = stopPodSandbox(p.runtimeClient, ps.status.Id)
	if err != nil {
		// Note the error, but shouldn't prevent us trying to delete
		log.Print(err)
	}

	// Remove any emptyDir volumes
	err = os.RemoveAll(filepath.Join(p.podVolRoot, string(pod.UID)))
	if  err != nil {
		log.Print(err)
	}
	err = removePodSandbox(p.runtimeClient, ps.status.Id)

	return err
}

func (p *CRIProvider) GetPod(namespace, name string) (*v1.Pod, error) {
	log.Printf("receive GetPod %q\n", name)

	err := p.refreshNodeState()
	if err != nil {
		return nil, err
	}

	pod := p.findPodByName(namespace, name)
	if pod == nil {
		return nil, fmt.Errorf("Pod %s in namespace %s could not be found on the node", name, namespace)
	}

	return createPodSpecFromCRI(pod, p.nodeName), nil
}

func readLogFile(filename string, tail int) (string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// TODO: This is not an efficient algorithm for tailing very large logs files
        scanner := bufio.NewScanner(file)
	lines := []string{}
        for scanner.Scan() {
                lines = append(lines, scanner.Text())
        }
        if tail > 0 && tail < len(lines) {
                lines = lines[len(lines)-tail:len(lines)]
        }
        return strings.Join(lines, "\n"), nil
}

// GetContainerLogs retrieves the logs of a container by name from the provider.
func (p *CRIProvider) GetContainerLogs(namespace, podName, containerName string, tail int) (string, error) {
        log.Printf("receive GetContainerLogs %q\n", containerName)

        err := p.refreshNodeState()
        if err != nil {
                return "", err
        }

	pod := p.findPodByName(namespace, podName)
	if pod == nil {
		return "", fmt.Errorf("Pod %s in namespace %s not found", podName, namespace)
	}
	container := pod.containers[containerName]
	if container == nil {
		return "", fmt.Errorf("Cannot find container %s in pod %s namespace %s", containerName, pod, namespace)
	}

	return readLogFile(container.LogPath, tail)
}

func (p *CRIProvider) findPodByName(namespace, name string) *CRIPod {
	var found *CRIPod

	for _, pod := range p.podStatus {
		if pod.status.Metadata.Name == name && pod.status.Metadata.Namespace == namespace {
			found = &pod
			break
		}
	}
	return found
}

// returns nil if a pod by that name is not found.
func (p *CRIProvider) GetPodStatus(namespace, name string) (*v1.PodStatus, error) {
	log.Printf("receive GetPodStatus %q\n", name)

	err := p.refreshNodeState()
	if err != nil {
		return nil, err
	}

	pod := p.findPodByName(namespace, name)
	if pod == nil {
		return nil, fmt.Errorf("Pod %s in namespace %s could not be found on the node", name, namespace)
	}

	return createPodStatusFromCRI(pod), nil
}

func createContainerStateFromCRI(state criapi.ContainerState, status *criapi.ContainerStatus) *v1.ContainerState {
	var result *v1.ContainerState
	switch state {
	case criapi.ContainerState_CONTAINER_UNKNOWN:
		fallthrough
	case criapi.ContainerState_CONTAINER_CREATED:
		result = &v1.ContainerState {
				Waiting: &v1.ContainerStateWaiting {
					Reason: status.Reason,
					Message: status.Message,
				},
		}
	case criapi.ContainerState_CONTAINER_RUNNING:
		result = &v1.ContainerState {
				Running: &v1.ContainerStateRunning {
					StartedAt: metav1.NewTime(time.Unix(0, status.StartedAt)),
				},
		}
	case criapi.ContainerState_CONTAINER_EXITED:
		result = &v1.ContainerState {
				Terminated: &v1.ContainerStateTerminated {
					ExitCode:   status.ExitCode,
					Reason:     status.Reason,
					Message:    status.Message,
					StartedAt:  metav1.NewTime(time.Unix(0, status.StartedAt)),
					FinishedAt: metav1.NewTime(time.Unix(0, status.FinishedAt)),
				},
		}
	}
	return result
}

func createContainerSpecsFromCRI(containerMap map[string]*criapi.ContainerStatus) ([]v1.Container, []v1.ContainerStatus) {
	containers := make([]v1.Container, 0, len(containerMap))
	containerStatuses := make([]v1.ContainerStatus, 0, len(containerMap))
	for _, c := range containerMap {
		// TODO: Fill out more fields
		container := v1.Container{
			Name:  c.Metadata.Name,
			Image: c.Image.Image,
			//Command:    Command is buried in the Info JSON,
		}
		containers = append(containers, container)
		// TODO: Fill out more fields
		containerStatus := v1.ContainerStatus{
			Name:                 c.Metadata.Name,
			Image:                c.Image.Image,
			ImageID:              c.ImageRef,
			ContainerID:          c.Id,
			Ready:                c.State == criapi.ContainerState_CONTAINER_RUNNING,
			State:                *createContainerStateFromCRI(c.State, c),
			// LastTerminationState:
			// RestartCount:
		}

		containerStatuses = append(containerStatuses, containerStatus)
	}
	return containers, containerStatuses
}

func createPodStatusFromCRI(p *CRIPod) *v1.PodStatus {
	_, cStatuses := createContainerSpecsFromCRI(p.containers)

	// TODO: How to determine PodSucceeded and PodFailed?
	phase := v1.PodPending
	if p.status.State == criapi.PodSandboxState_SANDBOX_READY {
		phase = v1.PodRunning
	}
	startTime := metav1.NewTime(time.Unix(0, p.status.CreatedAt))
	return &v1.PodStatus{
		Phase:             phase,
		Conditions:        []v1.PodCondition{},
		Message:           "",
		Reason:            "",
		HostIP:            "",
		PodIP:             p.status.Network.Ip,
		StartTime:         &startTime,
		ContainerStatuses: cStatuses,
	}
}

func createPodSpecFromCRI(p *CRIPod, nodeName string) *v1.Pod {
	cSpecs, _ := createContainerSpecsFromCRI(p.containers)

	// TODO: Fill out more fields here
	podSpec := v1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      p.status.Metadata.Name,
			Namespace: p.status.Metadata.Namespace,
			// ClusterName:       TODO: What is this??
			UID:               types.UID(p.status.Metadata.Uid),
			CreationTimestamp: metav1.NewTime(time.Unix(0, p.status.CreatedAt)),
		},
		Spec: v1.PodSpec{
			NodeName:   nodeName,
			Volumes:    []v1.Volume{},
			Containers: cSpecs,
		},
		Status: *createPodStatusFromCRI(p),
	}

//	fmt.Printf("Created Pod Spec %v\n", podSpec)
	return &podSpec
}

// TODO: Shouldn't this be all pods or just running pods?
func (p *CRIProvider) GetPods() ([]*v1.Pod, error) {
	log.Printf("receive GetPods\n")

	var pods []*v1.Pod

	err := p.refreshNodeState()
	if err != nil {
		return nil, err
	}

	for _, ps := range p.podStatus {
		pods = append(pods, createPodSpecFromCRI(&ps, p.nodeName))
	}

	return pods, nil
}

func getSystemTotalMemory() uint64 {
	in := &syscall.Sysinfo_t{}
	err := syscall.Sysinfo(in)
	if err != nil {
		return 0
	}
	return uint64(in.Totalram) * uint64(in.Unit)
}

func (p *CRIProvider) Capacity() v1.ResourceList {
	log.Printf("receive Capacity\n")

	err := p.refreshNodeState()
	if err != nil {
		log.Printf("Error getting pod status: %v\n", err)
	}

	var cpuQ resource.Quantity
	cpuQ.Set(int64(runtime.NumCPU()))
	var memQ resource.Quantity
	memQ.Set(int64(getSystemTotalMemory()))

	return v1.ResourceList{
		"cpu":    cpuQ,
		"memory": memQ,
		"pods":   resource.MustParse("1000"),
	}
}

// TODO: For now, use the same node conditions as the MockProvider
func (p *CRIProvider) NodeConditions() []v1.NodeCondition {
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
func (p *CRIProvider) NodeAddresses() []v1.NodeAddress {
	log.Printf("receive NodeAddresses - returning %s\n", p.internalIP)

	return []v1.NodeAddress{
		{
			Type:    "InternalIP",
			Address: p.internalIP,
		},
	}
}

// NodeDaemonEndpoints returns NodeDaemonEndpoints for the node status
// within Kubernetes.
func (p *CRIProvider) NodeDaemonEndpoints() *v1.NodeDaemonEndpoints {
	log.Printf("receive NodeDaemonEndpoints - returning %v\n", p.daemonEndpointPort)

	return &v1.NodeDaemonEndpoints{
		KubeletEndpoint: v1.DaemonEndpoint{
			Port: p.daemonEndpointPort,
		},
	}
}

// OperatingSystem returns the operating system for this provider.
// This is a noop to default to Linux for now.
func (p *CRIProvider) OperatingSystem() string {
	log.Printf("receive OperatingSystem - returning %s\n", providers.OperatingSystemLinux)

	return providers.OperatingSystemLinux
}

