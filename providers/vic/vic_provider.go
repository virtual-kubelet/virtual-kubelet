package vic

import (
	"fmt"
	"io"
	"os"
	"path"
	"syscall"
	"time"

	"golang.org/x/net/context"

	log "github.com/Sirupsen/logrus"
	"github.com/kr/pretty"

	vicerrors "github.com/vmware/vic/lib/apiservers/engine/errors"
	vicproxy "github.com/vmware/vic/lib/apiservers/engine/proxy"
	"github.com/vmware/vic/lib/apiservers/portlayer/client"
	"github.com/vmware/vic/lib/apiservers/portlayer/models"
	vicconst "github.com/vmware/vic/lib/constants"
	"github.com/vmware/vic/pkg/dio"
	viclog "github.com/vmware/vic/pkg/log"
	"github.com/vmware/vic/pkg/retry"
	"github.com/vmware/vic/pkg/trace"

	"github.com/virtual-kubelet/virtual-kubelet/manager"
	"github.com/virtual-kubelet/virtual-kubelet/providers/vic/cache"
	"github.com/virtual-kubelet/virtual-kubelet/providers/vic/operations"
	"github.com/virtual-kubelet/virtual-kubelet/providers/vic/proxy"
	"github.com/virtual-kubelet/virtual-kubelet/providers/vic/utils"

	"net"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/remotecommand"
)

type VicProvider struct {
	resourceManager *manager.ResourceManager
	nodeName        string
	os              string
	podCount        int
	config          VicConfig
	podCache        cache.PodCache

	client         *client.PortLayer
	imageStore     proxy.ImageStore
	isolationProxy proxy.IsolationProxy
	systemProxy    *vicproxy.VicSystemProxy
}

const (
	// Name of filename used in the endpoint vm
	LogFilename = "virtual-kubelet"

	// PanicLevel level, highest level of severity. Logs and then calls panic with the
	// message passed to Debug, Info, ...
	PanicLevel uint8 = iota
	// FatalLevel level. Logs and then calls `os.Exit(1)`. It will exit even if the
	// logging level is set to Panic.
	FatalLevel
	// ErrorLevel level. Logs. Used for errors that should definitely be noted.
	// Commonly used for hooks to send errors to an error tracking service.
	ErrorLevel
	// WarnLevel level. Non-critical entries that deserve eyes.
	WarnLevel
	// InfoLevel level. General operational entries about what's going on inside the
	// application.
	InfoLevel
	// DebugLevel level. Usually only enabled when debugging. Very verbose logging.
	DebugLevel
)

var (
	portlayerUp chan struct{}
)

func NewVicProvider(configFile string, rm *manager.ResourceManager, nodeName, operatingSystem string) (*VicProvider, error) {
	initLogger()

	op := trace.NewOperation(context.Background(), "VicProvider creation: config - %s", configFile)
	defer trace.End(trace.Begin("", op))

	config := NewVicConfig(op, configFile)
	op.Infof("Provider config = %#v", config)

	plClient := vicproxy.NewPortLayerClient(config.PortlayerAddr)

	op.Infof("** Wait for VCH servers to start")
	if !waitForVCH(op, plClient, config.PersonaAddr) {
		msg := "VicProvider timed out waiting for VCH's persona and portlayer servers"
		op.Errorf(msg)
		return nil, fmt.Errorf(msg)
	}

	i, err := proxy.NewImageStore(plClient, config.PersonaAddr, config.PortlayerAddr)
	if err != nil {
		msg := "Couldn't initialize the image store"
		op.Error(msg)
		return nil, fmt.Errorf(msg)
	}

	op.Infof("** creating proxy")
	p := VicProvider{
		config:          config,
		nodeName:        nodeName,
		os:              operatingSystem,
		podCache:        cache.NewVicPodCache(),
		client:          plClient,
		resourceManager: rm,
		systemProxy:     vicproxy.NewSystemProxy(plClient),
	}

	p.imageStore = i
	p.isolationProxy = proxy.NewIsolationProxy(plClient, config.PortlayerAddr, config.HostUUID, i, p.podCache)
	op.Infof("** ready to go")

	return &p, nil
}

func waitForVCH(op trace.Operation, plClient *client.PortLayer, personaAddr string) bool {
	backoffConf := retry.NewBackoffConfig()
	backoffConf.MaxInterval = 2 * time.Second
	backoffConf.InitialInterval = 500 * time.Millisecond
	backoffConf.MaxElapsedTime = 10 * time.Minute

	// Wait for portlayer to start up
	systemProxy := vicproxy.NewSystemProxy(plClient)

	opWaitForPortlayer := func() error {
		op.Infof("** Checking portlayer server is running")
		if !systemProxy.PingPortlayer(context.Background()) {
			return vicerrors.ServerNotReadyError{Name: "Portlayer"}
		}
		return nil
	}
	if err := retry.DoWithConfig(opWaitForPortlayer, vicerrors.IsServerNotReady, backoffConf); err != nil {
		op.Errorf("Wait for portlayer to be ready failed")
		return false
	}

	// Wait for persona to start up
	dockerClient := NewVicDockerClient(personaAddr)
	opWaitForPersona := func() error {
		op.Infof("** Checking persona server is running")
		if err := dockerClient.Ping(op); err != nil {
			return vicerrors.ServerNotReadyError{Name: "Persona"}
		}
		return nil
	}
	if err := retry.DoWithConfig(opWaitForPersona, vicerrors.IsServerNotReady, backoffConf); err != nil {
		op.Errorf("Wait for VIC docker server to be ready failed")
		return false
	}

	return true
}

func initLogger() {
	var logPath string
	if LocalInstance() {
		logPath = path.Join("", ".", LogFilename+".log")
	} else {
		logPath = path.Join("", vicconst.DefaultLogDir, LogFilename+".log")
	}

	os.MkdirAll(vicconst.DefaultLogDir, 0755)
	// #nosec: Expect file permissions to be 0600 or less
	f, err := os.OpenFile(logPath, os.O_RDWR|os.O_CREATE|os.O_APPEND|os.O_SYNC|syscall.O_NOCTTY, 0644)
	if err != nil {
		detail := fmt.Sprintf("failed to open file for VIC's virtual kubelet provider log: %s", err)
		log.Error(detail)
	}

	// use multi-writer so it goes to both screen and session log
	writer := dio.MultiWriter(f, os.Stdout)

	logcfg := viclog.NewLoggingConfig()

	logcfg.SetLogLevel(DebugLevel)
	trace.SetLogLevel(DebugLevel)
	trace.Logger.Out = writer

	err = viclog.Init(logcfg)
	if err != nil {
		return
	}

	trace.InitLogger(logcfg)
}

// CreatePod takes a Kubernetes Pod and deploys it within the provider.
func (v *VicProvider) CreatePod(ctx context.Context, pod *v1.Pod) error {
	op := trace.NewOperation(context.Background(), "CreatePod - %s", pod.Name)
	defer trace.End(trace.Begin(pod.Name, op))

	op.Debugf("Creating %s's pod = %# +v", pod.Name, pretty.Formatter(pod))

	pc, err := operations.NewPodCreator(v.client, v.imageStore, v.isolationProxy, v.podCache, v.config.PersonaAddr, v.config.PortlayerAddr)
	if err != nil {
		return err
	}

	err = pc.CreatePod(op, pod, true)
	if err != nil {
		return err
	}

	//v.resourceManager.AddPod()

	op.Debugf("** pod created ok")
	return nil
}

// UpdatePod takes a Kubernetes Pod and updates it within the provider.
func (v *VicProvider) UpdatePod(ctx context.Context, pod *v1.Pod) error {
	op := trace.NewOperation(context.Background(), "UpdatePod - %s", pod.Name)
	defer trace.End(trace.Begin(pod.Name, op))
	return nil
}

// DeletePod takes a Kubernetes Pod and deletes it from the provider.
func (v *VicProvider) DeletePod(ctx context.Context, pod *v1.Pod) error {
	op := trace.NewOperation(context.Background(), "DeletePod - %s", pod.Name)
	defer trace.End(trace.Begin(pod.Name, op))

	op.Infof("Deleting %s's pod spec = %#v", pod.Name, pod.Spec)

	pd, err := operations.NewPodDeleter(v.client, v.isolationProxy, v.podCache, v.config.PersonaAddr, v.config.PortlayerAddr)
	if err != nil {
		return err
	}

	err = pd.DeletePod(op, pod)

	return err
}

// GetPod retrieves a pod by name from the provider (can be cached).
func (v *VicProvider) GetPod(ctx context.Context, namespace, name string) (*v1.Pod, error) {
	op := trace.NewOperation(context.Background(), "GetPod - %s", name)
	defer trace.End(trace.Begin(name, op))

	// Look for the pod in our cache of running pods
	vp, err := v.podCache.Get(op, namespace, name)
	if err != nil {
		return nil, err
	}

	return vp.Pod, nil
}

// GetContainerLogs retrieves the logs of a container by name from the provider.
func (v *VicProvider) GetContainerLogs(ctx context.Context, namespace, podName, containerName string, tail int) (string, error) {
	op := trace.NewOperation(context.Background(), "GetContainerLogs - pod[%s], container[%s]", podName, containerName)
	defer trace.End(trace.Begin("", op))

	return "", nil
}

// Get full pod name as defined in the provider context
// TODO: Implementation
func (p *VicProvider) GetPodFullName(namespace string, pod string) string {
	return ""
}

// ExecInContainer executes a command in a container in the pod, copying data
// between in/out/err and the container's stdin/stdout/stderr.
func (p *VicProvider) ExecInContainer(name string, uid types.UID, container string, cmd []string, in io.Reader, out, err io.WriteCloser, tty bool, resize <-chan remotecommand.TerminalSize, timeout time.Duration) error {
	log.Printf("receive ExecInContainer %q\n", container)
	return nil
}

// GetPodStatus retrieves the status of a pod by name from the provider.
// This function needs to return a status or the reconcile loop will stop running.
func (v *VicProvider) GetPodStatus(ctx context.Context, namespace, name string) (*v1.PodStatus, error) {
	op := trace.NewOperation(context.Background(), "GetPodStatus - pod[%s], namespace", name, namespace)
	defer trace.End(trace.Begin("GetPodStatus", op))

	now := metav1.NewTime(time.Now())
	errorStatus := &v1.PodStatus{
		Phase:     v1.PodUnknown,
		StartTime: &now,
		Conditions: []v1.PodCondition{
			{
				Type:   v1.PodInitialized,
				Status: v1.ConditionUnknown,
			},
			{
				Type:   v1.PodReady,
				Status: v1.ConditionUnknown,
			},
			{
				Type:   v1.PodScheduled,
				Status: v1.ConditionUnknown,
			},
		},
	}

	// Look for the pod in our cache of running pods
	vp, err := v.podCache.Get(op, namespace, name)
	if err != nil {
		return errorStatus, err
	}

	// Instantiate status object
	statusReporter, err := operations.NewPodStatus(v.client, v.isolationProxy)
	if err != nil {
		return errorStatus, err
	}

	var nodeAddress string
	nodeAddresses := v.NodeAddresses(ctx)
	if len(nodeAddresses) > 0 {
		nodeAddress = nodeAddresses[0].Address
	} else {
		nodeAddress = "0.0.0.0"
	}
	status, err := statusReporter.GetStatus(op, vp.ID, name, nodeAddress)
	if err != nil {
		return errorStatus, err
	}

	if vp.Pod.Status.StartTime != nil {
		status.StartTime = vp.Pod.Status.StartTime
	} else {
		status.StartTime = &now
	}

	for _, container := range vp.Pod.Spec.Containers {
		status.ContainerStatuses = append(status.ContainerStatuses, v1.ContainerStatus{
			Name:         container.Name,
			Image:        container.Image,
			Ready:        true,
			RestartCount: 0,
			State: v1.ContainerState{
				Running: &v1.ContainerStateRunning{
					StartedAt: *status.StartTime,
				},
			},
		})
	}

	return status, nil
}

// GetPods retrieves a list of all pods running on the provider (can be cached).
func (v *VicProvider) GetPods(ctx context.Context) ([]*v1.Pod, error) {
	op := trace.NewOperation(context.Background(), "GetPods")
	defer trace.End(trace.Begin("GetPods", op))

	vps := v.podCache.GetAll(op)
	allPods := make([]*v1.Pod, 0)
	for _, vp := range vps {
		allPods = append(allPods, vp.Pod)
	}

	return allPods, nil
}

// Capacity returns a resource list with the capacity constraints of the provider.
func (v *VicProvider) Capacity(ctx context.Context) v1.ResourceList {
	op := trace.NewOperation(context.Background(), "VicProvider.Capacity")
	defer trace.End(trace.Begin("", op))

	if v.systemProxy == nil {
		err := NilProxy("VicProvider.Capacity", "SystemProxy")
		op.Error(err)
		return v1.ResourceList{}
	}
	info, err := v.systemProxy.VCHInfo(context.Background())
	if err != nil {
		op.Errorf("VicProvider.Capacity failed to get VCHInfo: %s", err.Error())
	}
	op.Infof("VCH Config: %# +v\n", pretty.Formatter(info))

	return KubeResourcesFromVchInfo(op, info)
}

// NodeConditions returns a list of conditions (Ready, OutOfDisk, etc), which is polled periodically to update the node status
// within Kubernetes.
func (v *VicProvider) NodeConditions(ctx context.Context) []v1.NodeCondition {
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
func (v *VicProvider) NodeAddresses(ctx context.Context) []v1.NodeAddress {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return []v1.NodeAddress{}
	}

	var outAddresses []v1.NodeAddress
	for _, addr := range addrs {
		ipnet, ok := addr.(*net.IPNet)
		if !ok || ipnet.IP.IsLoopback() || ipnet.IP.To4() == nil {
			continue
		}
		outAddress := v1.NodeAddress{
			Type:    v1.NodeInternalIP,
			Address: ipnet.IP.String(),
		}
		outAddresses = append(outAddresses, outAddress)
	}

	return outAddresses
}

// NodeDaemonEndpoints returns NodeDaemonEndpoints for the node status
// within Kubernetes.
func (v *VicProvider) NodeDaemonEndpoints(ctx context.Context) *v1.NodeDaemonEndpoints {
	return &v1.NodeDaemonEndpoints{
		KubeletEndpoint: v1.DaemonEndpoint{
			Port: 80,
		},
	}
}

// OperatingSystem returns the operating system the provider is for.
func (v *VicProvider) OperatingSystem() string {
	return v.os
}

//------------------------------------
// Utility Functions
//------------------------------------

// KubeResourcesFromVchInfo returns a K8s node resource list, given the VCHInfo
func KubeResourcesFromVchInfo(op trace.Operation, info *models.VCHInfo) v1.ResourceList {
	nr := make(v1.ResourceList)

	if info != nil {
		cores := utils.CpuFrequencyToCores(info.CPUMhz, "Mhz")
		// translate CPU resources.  K8s wants cores.  We have virtual cores based on mhz.
		cpuQ := resource.Quantity{}
		cpuQ.Set(cores)
		nr[v1.ResourceCPU] = cpuQ

		memQstr := utils.MemsizeToBinaryString(info.Memory, "Mb")
		// translate memory resources.  K8s wants bytes.
		memQ, err := resource.ParseQuantity(memQstr)
		if err == nil {
			nr[v1.ResourceMemory] = memQ
		} else {
			op.Errorf("KubeResourcesFromVchInfo, cannot parse MEM quantity: %s, err: %s", memQstr, err)
		}
	}

	// Estimate the available pod count, based on memory
	podCount := utils.MemsizeToMaxPodCount(info.Memory, "Mb")

	containerCountQ := resource.Quantity{}
	containerCountQ.Set(podCount)
	nr[v1.ResourcePods] = containerCountQ

	op.Infof("Capacity Resource Config: %# +v\n", pretty.Formatter(nr))
	return nr
}

func NilProxy(caller, proxyName string) error {

	return fmt.Errorf("%s: %s not valid", caller, proxyName)
}
