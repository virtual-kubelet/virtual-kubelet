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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

const (
	PodStatusReason_ProviderFailed = "ProviderFailed"
)

// Server masquarades itself as a kubelet and allows for the virtual node to be backed by non-vm/node providers.
type Server struct {
	nodeName        string
	namespace       string
	k8sClient       *kubernetes.Clientset
	taint           *corev1.Taint
	provider        providers.Provider
	podWatcher      watch.Interface
	resourceManager *manager.ResourceManager
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
}

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
		for range tick {
			ctx, span := trace.StartSpan(ctx, "syncActualState")
			s.updateNode(ctx)
			s.updatePodStatuses(ctx)
			span.End()
		}
	}()

	return s, nil
}

// Run starts the server, registers it with Kubernetes and begins watching/reconciling the cluster.
// Run will block until Stop is called or a SIGINT or SIGTERM signal is received.
func (s *Server) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		opts := metav1.ListOptions{
			FieldSelector: fields.OneTermEqualSelector("spec.nodeName", s.nodeName).String(),
		}

		pods, err := s.k8sClient.CoreV1().Pods(s.namespace).List(opts)
		if err != nil {
			return pkgerrors.Wrap(err, "error getting pod list")
		}
		s.resourceManager.SetPods(pods)
		s.reconcile(ctx)

		opts.ResourceVersion = pods.ResourceVersion
		s.podWatcher, err = s.k8sClient.CoreV1().Pods(s.namespace).Watch(opts)
		if err != nil {
			return pkgerrors.Wrap(err, "failed to watch pods")
		}


		if err := s.watchForPodEvent(ctx); err != nil {
			if pkgerrors.Cause(err) == context.Canceled {
				return err
			}
			log.G(ctx).Error(err)
			break
		}

		time.Sleep(5 * time.Second)
	}

	return nil
}

// Stop shutsdown the server.
// It does not shutdown pods assigned to the virtual node.
func (s *Server) Stop() {
	if s.podWatcher != nil {
		s.podWatcher.Stop()
	}
}

// reconcile is the main reconciliation loop that compares differences between Kubernetes and
// the active provider and reconciles the differences.
func (s *Server) reconcile(ctx context.Context) {
	ctx, span := trace.StartSpan(ctx, "reconcile")
	defer span.End()

	logger := log.G(ctx)
	logger.Debug("Start reconcile")
	defer logger.Debug("End reconcile")

	providerPods, err := s.provider.GetPods(ctx)
	if err != nil {
		logger.WithError(err).Error("Error getting pod list from provider")
		return
	}

	var deletePods []*corev1.Pod
	for _, pod := range providerPods {
		// Delete pods that don't exist in Kubernetes
		if p := s.resourceManager.GetPod(pod.Namespace, pod.Name); p == nil || p.DeletionTimestamp != nil {
			deletePods = append(deletePods, pod)
		}
	}
	span.Annotate(nil, "Got provider pods")

	var failedDeleteCount int64
	for _, pod := range deletePods {
		logger := logger.WithField("pod", pod.Name)
		logger.Debug("Deleting pod '%s'\n", pod.Name)
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

	pods := s.resourceManager.GetPods()

	var createPods []*corev1.Pod
	cleanupPods := deletePods[:0]

	for _, pod := range pods {
		var providerPod *corev1.Pod
		for _, p := range providerPods {
			if p.Namespace == pod.Namespace && p.Name == pod.Name {
				providerPod = p
				break
			}
		}

		// Delete pod if DeletionTimestamp is set
		if pod.DeletionTimestamp != nil {
			cleanupPods = append(cleanupPods, pod)
			continue
		}

		if providerPod == nil &&
			pod.DeletionTimestamp == nil &&
			pod.Status.Phase != corev1.PodSucceeded &&
			pod.Status.Phase != corev1.PodFailed &&
			pod.Status.Reason != PodStatusReason_ProviderFailed {
			createPods = append(createPods, pod)
		}
	}

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

	var failedCleanupCount int64
	for _, pod := range cleanupPods {
		logger := logger.WithField("pod", pod.Name)
		log.Trace(logger, "Pod pending deletion")
		var err error
		if err = s.deletePod(ctx, pod); err != nil {
			logger.WithError(err).Error("Error deleting pod")
			failedCleanupCount++
			continue
		}
		log.Trace(logger, "Pod deletion complete")
	}

	span.Annotate(
		[]trace.Attribute{
			trace.Int64Attribute("expected_cleaned_up_pods", int64(len(cleanupPods))),
			trace.Int64Attribute("cleaned_up_pod_failures", failedCleanupCount),
		},
		"Cleaned up provider pods marked for deletion",
	)
}
