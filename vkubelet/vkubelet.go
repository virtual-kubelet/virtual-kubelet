package vkubelet

import (
	"context"
	"net"
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
	podCh           chan *podNotification
	podInformer     corev1informers.PodInformer
}

// Config is used to configure a new server.
type Config struct {
	APIConfig       APIConfig
	Client          *kubernetes.Clientset
	MetricsAddr     string
	Namespace       string
	NodeName        string
	Provider        providers.Provider
	ResourceManager *manager.ResourceManager
	Taint           *corev1.Taint
	PodSyncWorkers  int
	PodInformer     corev1informers.PodInformer
}

// APIConfig is used to configure the API server of the virtual kubelet.
type APIConfig struct {
	CertPath string
	KeyPath  string
	Addr     string
}

type podNotification struct {
	pod *corev1.Pod
	ctx context.Context
}

// New creates a new virtual-kubelet server.
func New(ctx context.Context, cfg Config) (s *Server, retErr error) {
	s = &Server{
		namespace:       cfg.Namespace,
		nodeName:        cfg.NodeName,
		taint:           cfg.Taint,
		k8sClient:       cfg.Client,
		resourceManager: cfg.ResourceManager,
		provider:        cfg.Provider,
		podSyncWorkers:  cfg.PodSyncWorkers,
		podCh:           make(chan *podNotification, cfg.PodSyncWorkers),
		podInformer:     cfg.PodInformer,
	}

	ctx = log.WithLogger(ctx, log.G(ctx))

	apiL, err := net.Listen("tcp", cfg.APIConfig.Addr)
	if err != nil {
		return nil, pkgerrors.Wrap(err, "error setting up API listener")
	}
	defer func() {
		if retErr != nil {
			apiL.Close()
		}
	}()

	go KubeletServerStart(cfg.Provider, apiL, cfg.APIConfig.CertPath, cfg.APIConfig.KeyPath)

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
		return s, err
	}

	tick := time.Tick(5 * time.Second)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick:
				ctx, span := trace.StartSpan(ctx, "syncActualState")
				s.updateNode(ctx)
				s.updatePodStatuses(ctx)
				span.End()
			}
		}
	}()

	return s, nil
}

// Run creates and starts an instance of the pod controller, blocking until it stops.
func (s *Server) Run(ctx context.Context) error {
	return NewPodController(s).Run(ctx, s.podSyncWorkers)
}
