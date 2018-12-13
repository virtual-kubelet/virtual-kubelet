package vkubelet

import (
	"context"
	"net"
	"net/http"
	"time"

	pkgerrors "github.com/pkg/errors"
	"go.opencensus.io/trace"
	corev1 "k8s.io/api/core/v1"
	corev1informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/virtual-kubelet/virtual-kubelet/log"
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

// ServeMux defines an interface used to attach routes to an existing http serve mux
// It is used to enable callers creating a new server to completely manage their
// own HTTP server while allowing us to attach the required routes to satisfy
// the Kubelet interfaces.
type ServeMux interface {
	Handle(path string, h http.Handler, methods ...string) Route
	HandleFunc(path string, h http.HandlerFunc, methods ...string) Route
}

// APIConfig is used to configure the API server of the virtual kubelet.
// Routes required to satisfy the kubelet interface are attached to these.
//
// Callers should take care to namespace these serve muxes (muxi?) as they see
// fit, however these routes get called by the Kubernetes API server when a user
// requests things, e.g. `kubectl logs`.
type APIConfig struct {
	ServerMux  ServeMux
	MetricsMux ServeMux
}

// New creates a new virtual-kubelet server.
// This is the entrypoint to this package.
func New(cfg Config) (*Server, error) {
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
func (s *Server) Run(ctx context.Context, cfg APIConfig) (retErr error) {
	ctx = log.WithLogger(ctx, log.G(ctx))

	attachPodRoutes(cfg.Provider, cfg.ServerMux)
	go KubeletServerStart(cfg.Provider, apiL, cfg.CertPath, cfg.KeyPath)

	if cfg.MetricsAddr != "" {
		metricsL, err := net.Listen("tcp", cfg.MetricsAddr)
		if err != nil {
			return nil, pkgerrors.Wrap(err, "error setting up metrics listener")
		}
		defer func() {
			if retErr != nil {
				metricsL.Close()
			}
		}()
		go MetricsServerStart(cfg.Provider, metricsL)
	} else {
		log.G(ctx).Info("Skipping metrics server startup since no address was provided")
	}

	if err := s.registerNode(ctx); err != nil {
		return err
	}

	go func() {
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
	}()

	return NewPodController(s).Run(ctx, s.podSyncWorkers)
}
