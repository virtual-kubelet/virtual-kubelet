package nuczzz

import (
	"context"
	"io"
	"math/rand"
	"time"

	"github.com/pkg/errors"
	"github.com/sanity-io/litter"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/nuczzz/virtual-kubelet/cmd/virtual-kubelet/internal/provider"
	"github.com/nuczzz/virtual-kubelet/internal/kubernetes/k8s"
	"github.com/nuczzz/virtual-kubelet/internal/utils"
	"github.com/nuczzz/virtual-kubelet/log"
	"github.com/nuczzz/virtual-kubelet/node/api"
	"github.com/nuczzz/virtual-kubelet/node/api/statsv1alpha1"
	"github.com/nuczzz/virtual-kubelet/trace"
)

const (
	ProviderName = "nuczzz"
)

type Provider struct {
	provider.InitConfig

	namespace string
	cpu       resource.Quantity
	memory    resource.Quantity
	pods      resource.Quantity
	startTime time.Time
}

var _ provider.Provider = &Provider{}

func NewNuczzzProvider(ctx context.Context) provider.InitFunc {
	return func(config provider.InitConfig) (provider.Provider, error) {
		var vkConfig = &Config{}
		if err := parseConfig(config.ConfigPath, vkConfig); err != nil {
			return nil, errors.Wrap(err, "parseConfig error")
		}

		log.G(ctx).Infof("parseConfig success: %s", litter.Sdump(vkConfig))

		// init client of real kubernetes cluster
		if err := k8s.InitRealKubernetes(ctx.Done(), vkConfig.Kubeconfig); err != nil {
			return nil, errors.Wrap(err, "InitRealKubernetes error")
		}
		log.G(ctx).Infof("InitRealKubernetes success")

		return &Provider{
			InitConfig: config,
			namespace:  vkConfig.Namespace,
			cpu:        resource.MustParse(vkConfig.Capacity.CPU),
			memory:     resource.MustParse(vkConfig.Capacity.Memory),
			pods:       resource.MustParse(vkConfig.Capacity.Pods),
			startTime:  time.Now(),
		}, nil
	}
}

func copyContainers(old []corev1.Container) []corev1.Container {
	ret := make([]corev1.Container, 0, len(old))
	for i := range old {
		ret = append(ret, copyContainer(old[i]))
	}
	return ret
}

func copyContainer(old corev1.Container) corev1.Container {
	return corev1.Container{
		Name:            old.Name,
		Image:           old.Image,
		ImagePullPolicy: old.ImagePullPolicy,
		Ports:           old.Ports,
		Resources:       *old.Resources.DeepCopy(),
	}
}

func (p *Provider) CreatePod(ctx context.Context, pod *corev1.Pod) error {
	ctx, span := trace.StartSpan(ctx, "CreatePod")
	defer span.End()

	newPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        pod.Name,
			Namespace:   p.namespace,
			Labels:      pod.Labels,
			Annotations: pod.Annotations,
		},
		Spec: corev1.PodSpec{
			Containers: copyContainers(pod.Spec.Containers),
		},
	}

	if _, err := k8s.CreatePod(ctx, p.namespace, newPod); err != nil {
		return errors.Wrap(err, "CreatePod error")
	}

	return nil
}

func (p *Provider) UpdatePod(ctx context.Context, pod *corev1.Pod) error {
	ctx, span := trace.StartSpan(ctx, "UpdatePod")
	defer span.End()

	// TODO: patch pod

	return nil
}

func (p *Provider) DeletePod(ctx context.Context, pod *corev1.Pod) error {
	ctx, span := trace.StartSpan(ctx, "DeletePod")
	defer span.End()

	return k8s.DeletePod(ctx, p.namespace, pod.Name)
}

func (p *Provider) GetPod(ctx context.Context, namespace, name string) (*corev1.Pod, error) {
	ctx, span := trace.StartSpan(ctx, "GetPod")
	defer span.End()

	pod, err := k8s.GetPod(p.namespace, name)
	if err != nil {
		return nil, errors.Wrap(err, "k8s GetPod error")
	}

	copyPod := pod.DeepCopy()
	copyPod.Namespace = namespace

	return copyPod, nil
}

func (p *Provider) GetPodStatus(ctx context.Context, namespace, name string) (*corev1.PodStatus, error) {
	ctx, span := trace.StartSpan(ctx, "GetPodStatus")
	defer span.End()

	pod, err := k8s.GetPod(p.namespace, name)
	if err != nil {
		return nil, errors.Wrap(err, "k8s GetPod error")
	}

	return pod.Status.DeepCopy(), nil
}

func (p *Provider) GetPods(ctx context.Context) ([]*corev1.Pod, error) {
	ctx, span := trace.StartSpan(ctx, "GetPods")
	defer span.End()

	pods, err := k8s.ListPods()
	if err != nil {
		return nil, errors.Wrap(err, "k8s GetPod error")
	}

	ret := make([]*corev1.Pod, 0, len(pods))
	for _, pod := range pods {
		// todo: change namespace?
		ret = append(ret, pod.DeepCopy())
	}

	return ret, nil
}

func (p *Provider) GetContainerLogs(
	ctx context.Context,
	namespace, podName, containerName string,
	opts api.ContainerLogOpts,
) (io.ReadCloser, error) {
	var options = &corev1.PodLogOptions{
		Container:  containerName,
		Timestamps: opts.Timestamps,
		Follow:     opts.Follow,
		Previous:   opts.Previous,
	}
	if opts.Tail > 0 {
		options.TailLines = utils.Int64Ptr(int64(opts.Tail))
	}
	if opts.LimitBytes > 0 {
		options.LimitBytes = utils.Int64Ptr(int64(opts.LimitBytes))
	}
	if opts.SinceSeconds > 0 {
		options.SinceSeconds = utils.Int64Ptr(int64(opts.SinceSeconds))
	}
	if !opts.SinceTime.IsZero() {
		t := metav1.NewTime(opts.SinceTime)
		options.SinceTime = &t
	}

	return k8s.GetClientSet().CoreV1().Pods(p.namespace).GetLogs(podName, options).Stream(ctx)
}

type terminalSizeQueue struct {
	ch <-chan api.TermSize
}

var _ remotecommand.TerminalSizeQueue = &terminalSizeQueue{}

func newTerminalSizeQueue(ch <-chan api.TermSize) remotecommand.TerminalSizeQueue {
	return &terminalSizeQueue{ch: ch}
}

func (tsq *terminalSizeQueue) Next() *remotecommand.TerminalSize {
	next := <-tsq.ch
	return &remotecommand.TerminalSize{
		Width:  next.Width,
		Height: next.Height,
	}
}

func (p *Provider) RunInContainer(
	ctx context.Context,
	namespace, podName, containerName string,
	cmd []string,
	attach api.AttachIO,
) error {
	req := k8s.GetClientSet().CoreV1().RESTClient().Post().
		Resource("pods").
		Namespace(p.namespace).
		Name(podName).
		SubResource("exec")
	req.VersionedParams(&corev1.PodExecOptions{
		Container: containerName,
		Command:   cmd,
		Stdin:     true,
		Stdout:    true,
		Stderr:    true,
		TTY:       true,
	}, scheme.ParameterCodec)

	executor, err := remotecommand.NewSPDYExecutor(k8s.GetConfig(), "POST", req.URL())
	if err != nil {
		return errors.Wrap(err, "NewSPDYExecutor error")
	}

	return executor.Stream(remotecommand.StreamOptions{
		Stdin:             attach.Stdin(),
		Stdout:            attach.Stdout(),
		Stderr:            attach.Stderr(),
		TerminalSizeQueue: newTerminalSizeQueue(attach.Resize()),
		Tty:               attach.TTY(),
	})
}

func (p *Provider) GetStatsSummary(ctx context.Context) (*statsv1alpha1.Summary, error) {
	ctx, span := trace.StartSpan(ctx, "GetStatsSummary") //nolint: ineffassign,staticcheck
	defer span.End()

	// Grab the current timestamp so we can report it as the time the stats were generated.
	now := metav1.Now()

	// Create the Summary object that will later be populated with node and pod stats.
	res := &statsv1alpha1.Summary{}

	// Populate the Summary object with basic node stats.
	res.Node = statsv1alpha1.NodeStats{
		NodeName:  p.NodeName,
		StartTime: metav1.NewTime(p.startTime),
	}

	pods, err := p.GetPods(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "GetPods error")
	}

	// Populate the Summary object with dummy stats for each pod known by this provider.
	for _, pod := range pods {
		var (
			// totalUsageNanoCores will be populated with the sum of the values of UsageNanoCores computes across all containers in the pod.
			totalUsageNanoCores uint64
			// totalUsageBytes will be populated with the sum of the values of UsageBytes computed across all containers in the pod.
			totalUsageBytes uint64
		)

		// Create a PodStats object to populate with pod stats.
		pss := statsv1alpha1.PodStats{
			PodRef: statsv1alpha1.PodReference{
				Name:      pod.Name,
				Namespace: pod.Namespace,
				UID:       string(pod.UID),
			},
			StartTime: pod.CreationTimestamp,
		}

		// Iterate over all containers in the current pod to compute dummy stats.
		for _, container := range pod.Spec.Containers {
			// Grab a dummy value to be used as the total CPU usage.
			// The value should fit a uint32 in order to avoid overflows later on when computing pod stats.
			dummyUsageNanoCores := uint64(rand.Uint32())
			totalUsageNanoCores += dummyUsageNanoCores
			// Create a dummy value to be used as the total RAM usage.
			// The value should fit a uint32 in order to avoid overflows later on when computing pod stats.
			dummyUsageBytes := uint64(rand.Uint32())
			totalUsageBytes += dummyUsageBytes
			// Append a ContainerStats object containing the dummy stats to the PodStats object.
			pss.Containers = append(pss.Containers, statsv1alpha1.ContainerStats{
				Name:      container.Name,
				StartTime: pod.CreationTimestamp,
				CPU: &statsv1alpha1.CPUStats{
					Time:           now,
					UsageNanoCores: &dummyUsageNanoCores,
				},
				Memory: &statsv1alpha1.MemoryStats{
					Time:       now,
					UsageBytes: &dummyUsageBytes,
				},
			})
		}

		// Populate the CPU and RAM stats for the pod and append the PodsStats object to the Summary object to be returned.
		pss.CPU = &statsv1alpha1.CPUStats{
			Time:           now,
			UsageNanoCores: &totalUsageNanoCores,
		}
		pss.Memory = &statsv1alpha1.MemoryStats{
			Time:       now,
			UsageBytes: &totalUsageBytes,
		}
		res.Pods = append(res.Pods, pss)
	}

	// Return the dummy stats.
	return res, nil
}

func (p *Provider) ConfigureNode(ctx context.Context, node *corev1.Node) {
	ctx, span := trace.StartSpan(ctx, "ConfigureNode") //nolint: ineffassign,staticcheck
	defer span.End()

	node.Status.Capacity = p.capacity()
	node.Status.Allocatable = p.capacity()
	node.Status.Conditions = p.nodeConditions()
	node.Status.Addresses = p.nodeAddresses()
	node.Status.DaemonEndpoints = p.nodeDaemonEndpoints()
	os := p.OperatingSystem
	if os == "" {
		os = "linux"
	}
	node.Status.NodeInfo.OperatingSystem = os
	node.Status.NodeInfo.Architecture = "amd64"
	node.ObjectMeta.Labels["alpha.service-controller.kubernetes.io/exclude-balancer"] = "true"
	node.ObjectMeta.Labels["node.kubernetes.io/exclude-from-external-load-balancers"] = "true"
}

func (p *Provider) capacity() corev1.ResourceList {
	return corev1.ResourceList{
		corev1.ResourceCPU:    p.cpu,
		corev1.ResourceMemory: p.memory,
		corev1.ResourcePods:   p.pods,
	}
}

func (p *Provider) nodeConditions() []corev1.NodeCondition {
	return []corev1.NodeCondition{
		{
			Type:               corev1.NodeReady,
			Status:             corev1.ConditionTrue,
			LastHeartbeatTime:  metav1.Now(),
			LastTransitionTime: metav1.Now(),
			Reason:             "virtualKubeletReady",
			Message:            "virtual kubelet is ready",
		},
	}
}

func (p *Provider) nodeAddresses() []corev1.NodeAddress {
	return []corev1.NodeAddress{
		{
			Type:    corev1.NodeInternalIP,
			Address: p.InternalIP,
		},
	}
}

func (p *Provider) nodeDaemonEndpoints() corev1.NodeDaemonEndpoints {
	return corev1.NodeDaemonEndpoints{
		KubeletEndpoint: corev1.DaemonEndpoint{
			Port: p.DaemonPort,
		},
	}
}
