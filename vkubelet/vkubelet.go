package vkubelet

import (
	"context"
	"net"
	"sync"
	"sync/atomic"
	"time"

	pkgerrors "github.com/pkg/errors"
	"github.com/virtual-kubelet/virtual-kubelet/log"
	"github.com/virtual-kubelet/virtual-kubelet/manager"
	"github.com/virtual-kubelet/virtual-kubelet/providers"
	"go.opencensus.io/trace"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/informers"
	informersv1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
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
	podInformer     informersv1.PodInformer
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
	InformerFactory informers.SharedInformerFactory
}

// APIConfig is used to configure the API server of the virtual kubelet.
type APIConfig struct {
	CertPath string
	KeyPath  string
	Addr     string
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
	}

	filtered := informersv1.New(cfg.InformerFactory, cfg.Namespace, func(opts *metav1.ListOptions) {
		opts.FieldSelector = fields.OneTermEqualSelector("spec.nodeName", cfg.NodeName).String()
	})
	s.podInformer = filtered.Pods()

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

	return s, nil
}

// Run starts the server, registers it with Kubernetes and begins watching/reconciling the cluster.
// Run will block until the passed in context is canceled.
func (s *Server) Run(ctx context.Context) error {
	logger := log.G(ctx).WithField("method", "Run")
	logger.Debug("waiting for cache sync")

	go s.podInformer.Informer().Run(ctx.Done())

	if !cache.WaitForCacheSync(ctx.Done(), s.podInformer.Informer().HasSynced) {
		return ctx.Err()
	}

	logger.Debug("reconciling provider pods")
	s.reconcile(ctx)

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

	go func() {
		tick := time.NewTicker(time.Minute)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				s.reconcile(ctx)
			}
		}
	}()

	return s.watchForPodEvent(ctx)
}

// reconcile ensures that pods that exist in the provider which don't exist in
// kubernetes are cleaned up.
// This catches leaked pods due to missed events as well as pre-existing pods on
// startup that no longer exist in kubernetes.
func (s *Server) reconcile(ctx context.Context) {
	ctx, span := trace.StartSpan(ctx, "reconcile")
	defer span.End()

	logger := log.G(ctx).WithField("caller", "reconcile")
	ctx = log.WithLogger(ctx, logger)

	providerPods, err := s.provider.GetPods(ctx)
	if err != nil {
		logger.WithError(err).Error("Error getting pod list from provider")
		return
	}

	var deletePods []*corev1.Pod
	for _, pod := range providerPods {
		// Delete pods that don't exist in Kubernetes
		key, err := cache.MetaNamespaceKeyFunc(pod)
		if err != nil {
			logger.WithError(err).WithField("pod", pod.GetName()).WithField("namespace", pod.GetNamespace()).Error("error generating store key for pod")
		}

		p, err := s.podInformer.Lister().Pods(pod.GetNamespace()).Get(pod.GetName())
		if err != nil {
			log.G(ctx).WithError(err).WithField(key, "key").Debug("Error reading from pod cache")
			continue
		}

		if p == nil || p.DeletionTimestamp != nil {
			deletePods = append(deletePods, pod)
		}
	}
	span.Annotate(nil, "Got provider pods")

	sema := make(chan struct{}, s.podSyncWorkers)
	var wg sync.WaitGroup

	var failedDeleteCount int64
	for _, pod := range deletePods {
		wg.Add(1)
		go func(pod *corev1.Pod) {
			sema <- struct{}{}
			defer func() {
				wg.Done()
				<-sema
			}()

			logger := logger.WithField("pod", pod.GetName()).WithField("namespace", pod.GetNamespace())
			logger.Debug("Deleting pod")

			if err := s.deletePod(ctx, pod); err != nil {
				logger.WithError(err).Error("Error deleting pod")
				atomic.AddInt64(&failedDeleteCount, 1)
				return
			}
		}(pod)
	}

	wg.Wait()
	span.Annotate(
		[]trace.Attribute{
			trace.Int64Attribute("expected_delete_pods_count", int64(len(deletePods))),
			trace.Int64Attribute("failed_delete_pods_count", failedDeleteCount),
		},
		"Cleaned up stale provider pods",
	)
}
