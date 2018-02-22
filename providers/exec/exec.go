package exec

import (
	osexec "os/exec"
	"k8s.io/api/core/v1"
	"github.com/virtual-kubelet/virtual-kubelet/providers"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	ps "github.com/mitchellh/go-ps"
	"log"
	"os"
	"github.com/virtual-kubelet/virtual-kubelet/manager"
	"path/filepath"
	"fmt"
	"github.com/pkg/errors"
	"time"
	"bufio"
	"bytes"
	"net"
)

// Exec implements the virtual-kubelet provider interface and spawn non-Dockerize process using os/exec
type ExecProvider struct {
	state           	*state
	resourceManager 	*manager.ResourceManager
	nodeName        	string
	cpu             	string
	memory          	string
	pods            	string
	operatingSystem 	string
	logDir          	string
	stateDir			string
	daemonEndpointPort	int32
}

func NewExecProvider(config string, rm *manager.ResourceManager, nodeName string, operatingSystem string, daemonEndpointPort int32) (*ExecProvider, error) {
	var p ExecProvider
	var err error

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


	if operatingSystem != providers.OperatingSystemLinux {
		// TODO: Support Windows
		return nil, errors.Errorf("The Exec provider only supports %s", providers.OperatingSystemLinux)
	}

	// TODO: Could we use etcd to store that information (maybe using the PodStatus) ?
	// TODO: What if the state is lost but a rogue process is still running ?
	p.state, err = NewState(filepath.Join(p.stateDir, "exec.db"))
	if err != nil {
		log.Fatal(err)
	}
	p.operatingSystem = operatingSystem
	p.nodeName = nodeName
	p.resourceManager = rm
	p.daemonEndpointPort = daemonEndpointPort

	return &p, nil
}

// CreatePod takes a Kubernetes Pod and deploys it within the provider.
func (p *ExecProvider) CreatePod(pod *v1.Pod) error {
	if ok, err := p.validateExecRequirements(pod); !ok {
		return err
	}
	processes := map[string]Process{}

	// TODO: Accept the Pod directly and spawn process in a go routine (status would get reported via GetPodStatus)
	// TODO: Better error handling, if a  "container" fails it should probably kill them all and start over
	for _, container := range pod.Spec.Containers {
		c := p.createCommand(container)
		logPath := filepath.Join(p.logDir, pod.Namespace, pod.Name)
		err := os.MkdirAll(logPath, os.ModePerm)
		if err != nil {
			log.Printf("Error creating log folder for %s/%s-%s: %s\n", pod.Namespace, pod.Name, container.Name, err.Error())
			return err
		}
		f, err := os.Create(filepath.Join(logPath, fmt.Sprintf("%s.log", container.Name)))
		defer f.Close()
		if err != nil {
			log.Printf("Error creating log file for %s/%s-%s: %s\n", pod.Namespace, pod.Name, container.Name, err.Error())
			return err
		}
		// Same file for stdout and stderr to make `kubectl logs` retrieve both
		c.Stdout = f
		c.Stderr = f
		err = c.Start()
		if err != nil {
			log.Printf("Error when starting command for %s/%s: %s\n", pod.Namespace, pod.Name, err.Error())
			return err
		}

		processes[container.Name] = Process{
			Image:          container.Image,
			Pid:            c.Process.Pid,
			LogPath:        f.Name(),
			StartTimestamp: time.Now().Unix(),
		}
		err = p.state.Put(Entry{PodName: pod.Name, Namespace: pod.Namespace, Processes: processes})
		if err != nil {
			log.Printf("Error handling state of %s/%s: %s\n", pod.Namespace, pod.Name, err.Error())
			return err
		}

		err = c.Process.Release()
		if err != nil {
			log.Printf("Error releasing process of %s/%s: %s\n", pod.Namespace, pod.Name, err.Error())
			return err
		}
	}
	return nil
}

func (p *ExecProvider) createCommand(container v1.Container) *osexec.Cmd {
	return osexec.Command(container.Command[0],
		append(
			container.Command[1:],
			container.Args...)...
	)
}

// UpdatePod takes a Kubernetes Pod and updates it within the provider.
func (p *ExecProvider) UpdatePod(pod *v1.Pod) error {
	// TODO: Not supported now, it could eventually kill and restart all "containers"
	return nil
}

// DeletePod takes a Kubernetes Pod and deletes it from the provider.
func (p *ExecProvider) DeletePod(pod *v1.Pod) error {
	e, err := p.state.Get(pod.Namespace, pod.Name)
	if err != nil || e == nil {
		return err
	}

	for _, proc := range e.Processes {
		process, err := os.FindProcess(proc.Pid)
		if err != nil {
			return err
		}
		err = process.Kill()
		if err != nil {
			return err
		}
	}

	err = p.state.Remove(pod.Namespace, pod.Name)
	return err
}

// GetPod retrieves a pod by name from the provider (can be cached).
func (p *ExecProvider) GetPod(namespace, name string) (*v1.Pod, error) {
	entry, err := p.state.Get(namespace, name)
	if err != nil || entry == nil {
		return nil, err
	}

	return p.toPodSpec(entry), nil
}


// GetContainerLogs retrieves the logs of a container by name from the provider.
func (p *ExecProvider) GetContainerLogs(namespace, podName, containerName string, tail int) (string, error) {
	e, err := p.state.Get(namespace, podName)
	if err != nil {
		return "", err
	}

	return p.tailFile(e.Processes[containerName].LogPath, tail)
}

func (p *ExecProvider) tailFile(filePath string, tail int) (string, error) {
	// Buffered channel to store the last N lines
	ch := make(chan string, tail)

	file, err := os.Open(filePath)
	defer file.Close()
	if err != nil {
		return "", err
	}

	// TODO: Reading the whole file is far from ideal
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		select {
		case ch <- scanner.Text():
		default:
			// Channel full, discarding one first
			<- ch
			ch <- scanner.Text()
		}
	}
	close(ch)
	// Merge lines
	var buffer bytes.Buffer
	for l := range ch {
		buffer.WriteString(l)
		buffer.WriteString("\n")
	}
	return buffer.String(), nil
}



// GetPodStatus retrieves the status of a pod by name from the provider.
func (p *ExecProvider) GetPodStatus(namespace, podName string) (*v1.PodStatus, error) {
	status := v1.PodStatus{ Phase: v1.PodRunning}

	e, err := p.state.Get(namespace, podName)
	if err != nil {
		status.Phase = v1.PodUnknown
		return &status, err
	}

	for n, proc := range(e.Processes) {
		cs := v1.ContainerStatus{
			Name: n,
			Image: proc.Image,
		}
		p, err := ps.FindProcess(proc.Pid)
		if err != nil || p == nil {
			status.Phase = v1.PodFailed
			cs.Ready = false
		} else {
			cs.Ready = true
			cs.State = v1.ContainerState{
				Running: &v1.ContainerStateRunning{
					StartedAt: metav1.Unix(proc.StartTimestamp, 0),
				},
			}
		}
		status.ContainerStatuses = append(status.ContainerStatuses, cs)
	}

	return &status, nil
}

// GetPods retrieves a list of all pods running on the provider (can be cached).
func (p *ExecProvider) GetPods() ([]*v1.Pod, error) {
	var pods []*v1.Pod
	entries, err := p.state.GetAll()

	for _, e := range(entries) {
		pods = append(pods, p.toPodSpec(&e))
	}

	return pods, err
}

// Capacity returns a resource list with the capacity constraints of the provider.
func (p *ExecProvider) Capacity() v1.ResourceList {
	return v1.ResourceList{
		"cpu":    resource.MustParse(p.cpu),
		"memory": resource.MustParse(p.memory),
		"pods":   resource.MustParse(p.pods),
	}
}

// NodeConditions returns a list of conditions (Ready, OutOfDisk, etc), which is polled periodically to update the node status
// within Kuberentes.
func (p *ExecProvider) NodeConditions() []v1.NodeCondition {
	// TODO: Implement this
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
func (p *ExecProvider) NodeAddresses() []v1.NodeAddress {
	// Any IP would do and it does not needs to be accessible because UDP
	conn, _ := net.Dial("udp", "8.8.8.8:53")
	defer conn.Close()
	addr := conn.LocalAddr().(*net.UDPAddr)
	return []v1.NodeAddress{
		{
			Type:    "InternalIP",
			Address: addr.IP.String(),
		},
	}
}

// NodeDaemonEndpoints returns NodeDaemonEndpoints for the node status
// within Kubernetes.
func (p *ExecProvider) NodeDaemonEndpoints() *v1.NodeDaemonEndpoints {
	return &v1.NodeDaemonEndpoints{
		KubeletEndpoint: v1.DaemonEndpoint{
			Port: p.daemonEndpointPort,
		},
	}
}

// OperatingSystem returns the operating system the provider is for.
func (p *ExecProvider) OperatingSystem() string {
	return p.operatingSystem
}

// Stop is called on shutdown, but should not stop any pods assigned to this node
func (p *ExecProvider) Stop() {
	if p.state != nil {
		p.state.Close()
	}
}

func (p *ExecProvider) validateExecRequirements(pod *v1.Pod) (bool, error){
	if !pod.Spec.HostNetwork || !pod.Spec.HostPID {
		return false, errors.New("Only a Pod with HostNetwork=true and HostPID=true can run on")
	}
	for _, c := range(pod.Spec.Containers) {
		if len(c.Command) == 0 {
			return false, errors.New("All containers needs atleast a Command to run")
		}
	}

	return true, nil
}

// TODO: Ideally we would convert more information
func (p *ExecProvider) toPodSpec(e *Entry) *v1.Pod {
	pod := v1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:              e.PodName,
			Namespace:         e.Namespace,
		},
	}
	return &pod
}