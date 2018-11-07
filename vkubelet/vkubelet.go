package vkubelet

import (
	"context"
	"net"
	"time"

	pkgerrors "github.com/pkg/errors"
	"github.com/virtual-kubelet/virtual-kubelet/log"
	"github.com/virtual-kubelet/virtual-kubelet/manager"
	"github.com/virtual-kubelet/virtual-kubelet/providers"
	"go.opencensus.io/trace"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	podStatusReasonProviderFailed = "ProviderFailed"
)

// Server masquarades itself as a kubelet and allows for the virtual node to be backed by non-vm/node providers.
type Server struct {
	nodeName        string
	namespace       string
	k8sClient       kubernetes.Interface
	taint           *corev1.Taint
	provider        providers.Provider
	resourceManager *manager.ResourceManager
	podSyncWorkers  int
	podCh           chan *podNotification
}

// Config is used to configure a new server.
type Config struct {
	APIConfig       APIConfig
	Client          kubernetes.Interface
	MetricsAddr     string
	Namespace       string
	NodeName        string
	Provider        providers.Provider
	ResourceManager *manager.ResourceManager
	Taint           *corev1.Taint
	PodSyncWorkers  int
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

	reconcileTick := time.Tick(1 * time.Minute)

	go func() {
                for range reconcileTick {
                        s.clearStaleProviderPods(ctx)
                }
	}()

	return s, nil
}

// Run starts the server, registers it with Kubernetes and begins watching/reconciling the cluster.
// Run will block until Stop is called or a SIGINT or SIGTERM signal is received.
func (s *Server) Run(ctx context.Context) error {
	if err := s.watchForPodEvent(ctx); err != nil {
		if pkgerrors.Cause(err) == context.Canceled {
			return err
		}
		log.G(ctx).Error(err)
	}

	return nil
}

func (s *Server) calculatePodDivergence(ctx context.Context) (podsToDelete []*corev1.Pod, podsToCreate []*corev1.Pod, e error) {
	ctx, span := trace.StartSpan(ctx, "calculatePodDivergence")
	defer span.End()

	logger := log.G(ctx).WithField("method", "calculatePodDivergence")

	providerPods, err := s.provider.GetPods(ctx)
	if err != nil {
		logger.WithError(err).Error("Error getting pod list from provider")
		return nil, nil, err
	}
	span.Annotate(nil, "Got provider pods")

	var deletePods []*corev1.Pod
	for _, pod := range providerPods {
		// Delete pod that don't exist in Kubernetes. Deleting pod should not return
		if p := s.resourceManager.GetPod(pod.Namespace, pod.Name); p == nil {
			deletePods = append(deletePods, pod)
		}
	}

	pods := s.resourceManager.GetPods()

	var createPods []*corev1.Pod
	for _, pod := range pods {
		var providerPod *corev1.Pod
		for _, p := range providerPods {
			if p.Namespace == pod.Namespace && p.Name == pod.Name {
				providerPod = p
				break
			}
		}

		// Pod appears both in provider and in Kubernetes. Do nothing
		if providerPod != nil {
			continue
		}

		// Delete pod if DeletionTimestamp is set
		if pod.DeletionTimestamp != nil {
			deletePods = append(deletePods, pod)
			continue
		}

		// Create pod if it's non-terminated in Kubernetes, but not in provider
		if pod.Status.Phase != corev1.PodSucceeded &&
			pod.Status.Phase != corev1.PodFailed &&
			pod.Status.Reason != podStatusReasonProviderFailed {
			createPods = append(createPods, pod)
		}
	}

	return deletePods, createPods, nil
}

func (s *Server) clearStaleProviderPods(ctx context.Context) {
	ctx, span := trace.StartSpan(ctx, "clearStaleProviderPods")
	defer span.End()

	logger := log.G(ctx).WithField("method", "clearStaleProviderPods")
	logger.Debug("Start clear stale provider pods")
	defer logger.Debug("End clear stale provider pods")

	deletePods, _, err := s.calculatePodDivergence(ctx)
	if err != nil {
		logger.WithError(err).Error("Error calculating pod divergence")
		return
	}

	var failedDeleteCount int64
	for _, pod := range deletePods {
		logger := logger.WithField("pod", pod.GetName()).WithField("namespace", pod.GetNamespace())
		logger.Debug("Deleting pod")
		if err := s.deletePod(ctx, pod); err != nil {
			logger.WithError(err).Error("Error deleting pod")
			failedDeleteCount++
			continue
		}
	}
	span.Annotate(
		[]trace.Attribute{
			trace.Int64Attribute("expected_delete_pods_count", int64(len(deletePods))),
			trace.Int64Attribute("failed_delete_pods_count", failedDeleteCount),
		},
		"Cleaned up stale provider pods",
	)
}

// reconcile is the main reconciliation loop that compares differences between Kubernetes and
// the active provider and reconciles the differences.
func (s *Server) reconcile(ctx context.Context) {
	ctx, span := trace.StartSpan(ctx, "reconcile")
	defer span.End()

	logger := log.G(ctx).WithField("method", "reconcile")
	logger.Debug("Start reconcile")
	defer logger.Debug("End reconcile")

	deletePods, createPods, err := s.calculatePodDivergence(ctx)
	if err != nil {
		logger.WithError(err).Error("Error calculating pod divergence")
		return
	}

	var failedDeleteCount int64
	for _, pod := range deletePods {
		logger := logger.WithField("pod", pod.GetName()).WithField("namespace", pod.GetNamespace())
		logger.Debug("Deleting pod")
		if err := s.deletePod(ctx, pod); err != nil {
			logger.WithError(err).Error("Error deleting pod")
			failedDeleteCount++
			continue
		}
	}
	span.Annotate(
		[]trace.Attribute{
			trace.Int64Attribute("expected_delete_pods_count", int64(len(deletePods))),
			trace.Int64Attribute("failed_delete_pods_count", failedDeleteCount),
		},
		"Cleaned up stale provider pods",
	)

	var failedCreateCount int64
	for _, pod := range createPods {
		logger := logger.WithField("pod", pod.Name)
		logger.Debug("Creating pod")
		if err := s.createPod(ctx, pod); err != nil {
			failedCreateCount++
			logger.WithError(err).Error("Error creating pod")
			continue
		}
	}
	span.Annotate(
		[]trace.Attribute{
			trace.Int64Attribute("expected_created_pods", int64(len(createPods))),
			trace.Int64Attribute("failed_pod_creates", failedCreateCount),
		},
		"Created pods in provider",
	)
}
