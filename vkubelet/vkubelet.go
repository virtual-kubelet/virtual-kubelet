package vkubelet

import (
	"context"
	"time"

	"go.opencensus.io/trace"
	corev1 "k8s.io/api/core/v1"
	corev1informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/virtual-kubelet/virtual-kubelet/manager"
	"github.com/virtual-kubelet/virtual-kubelet/providers"
)

const (
	podStatusReasonProviderFailed = "ProviderFailed"
)

// Server masquarades itself as a kubelet and allows for the virtual node to be backed by non-vm/node providers.
type Server struct {
	nodeName        string
	namespace       string
	k8sClient       *kubernetes.Clientset
	taint           *corev1.Taint
	provider        providers.Provider
	resourceManager *manager.ResourceManager
	podSyncWorkers  int
	podInformer     corev1informers.PodInformer
}

// Config is used to configure a new server.
type Config struct {
	Client          *kubernetes.Clientset
	Namespace       string
	NodeName        string
	Provider        providers.Provider
	ResourceManager *manager.ResourceManager
	Taint           *corev1.Taint
	PodSyncWorkers  int
	PodInformer     corev1informers.PodInformer
}

// New creates a new virtual-kubelet server.
// This is the entrypoint to this package.
//
// This creates but does not start the server.
// You must call `Run` on the returned object to start the server.
func New(cfg Config) *Server {
	return &Server{
		namespace:       cfg.Namespace,
		nodeName:        cfg.NodeName,
		taint:           cfg.Taint,
		k8sClient:       cfg.Client,
		resourceManager: cfg.ResourceManager,
		provider:        cfg.Provider,
		podSyncWorkers:  cfg.PodSyncWorkers,
		podInformer:     cfg.PodInformer,
	}
}

// Run creates and starts an instance of the pod controller, blocking until it stops.
//
// Note that this does not setup the HTTP routes that are used to expose pod
// info to the Kubernetes API Server, such as logs, metrics, exec, etc.
// See `AttachPodRoutes` and `AttachMetricsRoutes` to set these up.
func (s *Server) Run(ctx context.Context) error {
	if err := s.registerNode(ctx); err != nil {
		return err
	}

	go s.providerSyncLoop(ctx)

	return NewPodController(s).Run(ctx, s.podSyncWorkers)
}

// providerSyncLoop syncronizes pod states from the provider back to kubernetes
func (s *Server) providerSyncLoop(ctx context.Context) {
	// TODO(@cpuguy83): Ticker does not seem like the right thing to use here. A
	// ticker keeps ticking while we are updating state, which can be a long
	// operation. This would lead to just a constant re-sync rather than sleeping
	// for 5 seconds between each loop.
	//
	// Leaving this note here as fixing is out of scope for my current changeset.
	tick := time.NewTicker(5 * time.Second)
	defer tick.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			ctx, span := trace.StartSpan(ctx, "syncActualState")
			s.updateNode(ctx)
			s.updatePodStatuses(ctx)
			span.End()
		}
	}
}
