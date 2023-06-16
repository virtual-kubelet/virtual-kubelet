package nodeutil

import (
	"context"
	"io"

	dto "github.com/prometheus/client_model/go"
	"github.com/virtual-kubelet/virtual-kubelet/node"
	"github.com/virtual-kubelet/virtual-kubelet/node/api"
	"github.com/virtual-kubelet/virtual-kubelet/node/api/statsv1alpha1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

// Provider contains the methods required to implement a virtual-kubelet provider.
//
// Errors produced by these methods should implement an interface from
// github.com/virtual-kubelet/virtual-kubelet/errdefs package in order for the
// core logic to be able to understand the type of failure
type Provider interface {
	node.PodLifecycleHandler

	// GetContainerLogs retrieves the logs of a container by name from the provider.
	GetContainerLogs(ctx context.Context, namespace, podName, containerName string, opts api.ContainerLogOpts) (io.ReadCloser, error)

	// RunInContainer executes a command in a container in the pod, copying data
	// between in/out/err and the container's stdin/stdout/stderr.
	RunInContainer(ctx context.Context, namespace, podName, containerName string, cmd []string, attach api.AttachIO) error

	// AttachToContainer attaches to the executing process of a container in the pod, copying data
	// between in/out/err and the container's stdin/stdout/stderr.
	AttachToContainer(ctx context.Context, namespace, podName, containerName string, attach api.AttachIO) error

	// GetStatsSummary gets the stats for the node, including running pods
	GetStatsSummary(context.Context) (*statsv1alpha1.Summary, error)

	// GetMetricsResource gets the metrics for the node, including running pods
	GetMetricsResource(context.Context) ([]*dto.MetricFamily, error)

	// PortForward forwards a local port to a port on the pod
	PortForward(ctx context.Context, namespace, pod string, port int32, stream io.ReadWriteCloser) error
}

// ProviderConfig holds objects created by NewNodeFromClient that a provider may need to bootstrap itself.
type ProviderConfig struct {
	Pods       corev1listers.PodLister
	ConfigMaps corev1listers.ConfigMapLister
	Secrets    corev1listers.SecretLister
	Services   corev1listers.ServiceLister
	// Hack to allow the provider to set things on the node
	// Since the provider is bootstrapped after the node object is configured
	// Primarily this is due to carry-over from the pre-1.0 interfaces that expect the provider instead of the direct *caller* to configure the node.
	Node *v1.Node
}

// NewProviderFunc is used from NewNodeFromClient to bootstrap a provider using the client/listers/etc created there.
// If a nil node provider is returned a default one will be used.
type NewProviderFunc func(ProviderConfig) (Provider, node.NodeProvider, error)

// AttachProviderRoutes returns a NodeOpt which uses api.PodHandler to attach the routes to the provider functions.
//
// Note this only attaches routes, you'll need to ensure to set the handler in the node config.
func AttachProviderRoutes(mux api.ServeMux) NodeOpt {
	return func(cfg *NodeConfig) error {
		cfg.routeAttacher = func(p Provider, cfg NodeConfig, pods corev1listers.PodLister) {
			mux.Handle("/", api.PodHandler(api.PodHandlerConfig{
				RunInContainer:    p.RunInContainer,
				AttachToContainer: p.AttachToContainer,
				GetContainerLogs:  p.GetContainerLogs,
				GetPods:           p.GetPods,
				GetPodsFromKubernetes: func(context.Context) ([]*v1.Pod, error) {
					return pods.List(labels.Everything())
				},
				GetStatsSummary:       p.GetStatsSummary,
				GetMetricsResource:    p.GetMetricsResource,
				StreamIdleTimeout:     cfg.StreamIdleTimeout,
				StreamCreationTimeout: cfg.StreamCreationTimeout,
				PortForward:           p.PortForward,
			}, true))
		}
		return nil
	}
}
