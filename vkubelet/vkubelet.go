package vkubelet

import (
	"context"
	"strconv"
	"time"

	"github.com/virtual-kubelet/virtual-kubelet/log"
	"github.com/virtual-kubelet/virtual-kubelet/manager"
	"github.com/virtual-kubelet/virtual-kubelet/providers"
	"github.com/virtual-kubelet/virtual-kubelet/trace"
	corev1 "k8s.io/api/core/v1"
	corev1informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/workqueue"
)

const (
	podStatusReasonProviderFailed = "ProviderFailed"
)

// Server masquarades itself as a kubelet and allows for the virtual node to be backed by non-vm/node providers.
type Server struct {
	namespace       string
	nodeName        string
	k8sClient       *kubernetes.Clientset
	provider        providers.Provider
	resourceManager *manager.ResourceManager
	podSyncWorkers  int

	podInformer       corev1informers.PodInformer
	configMapInformer corev1informers.ConfigMapInformer
	secretsInformer   corev1informers.SecretInformer

	secretRefs    *refCounter
	configMapRefs *refCounter
}

// Config is used to configure a new server.
type Config struct {
	Client            *kubernetes.Clientset
	Namespace         string
	NodeName          string
	Provider          providers.Provider
	ResourceManager   *manager.ResourceManager
	PodSyncWorkers    int
	PodInformer       corev1informers.PodInformer
	ConfigMapInformer corev1informers.ConfigMapInformer
	SecretsInformer   corev1informers.SecretInformer
}

// New creates a new virtual-kubelet server.
// This is the entrypoint to this package.
//
// This creates but does not start the server.
// You must call `Run` on the returned object to start the server.
func New(cfg Config) *Server {
	return &Server{
		nodeName:          cfg.NodeName,
		namespace:         cfg.Namespace,
		k8sClient:         cfg.Client,
		resourceManager:   cfg.ResourceManager,
		provider:          cfg.Provider,
		podSyncWorkers:    cfg.PodSyncWorkers,
		podInformer:       cfg.PodInformer,
		configMapInformer: cfg.ConfigMapInformer,
		secretsInformer:   cfg.SecretsInformer,
		secretRefs:        newRefCounter(),
		configMapRefs:     newRefCounter(),
	}
}

// Run creates and starts an instance of the pod controller, blocking until it stops.
//
// Note that this does not setup the HTTP routes that are used to expose pod
// info to the Kubernetes API Server, such as logs, metrics, exec, etc.
// See `AttachPodRoutes` and `AttachMetricsRoutes` to set these up.
func (s *Server) Run(ctx context.Context) error {
	log.G(ctx).Debug("Starting pod controller")

	// This is used to cancel other controllers if one errors out.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	chErr := make(chan error, 3)
	pc := NewPodController(s)
	go func() {
		chErr <- s.runPodController(ctx, pc)
	}()

	log.G(ctx).Debug("Wait for pod controller ready")

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-pc.Ready():
	}

	log.G(ctx).Debug("Pod controller ready")

	if cmu, ok := s.provider.(ConfigMapUpdater); ok {
		go func() {
			chErr <- s.runConfigMapController(ctx, cmu)
		}()
	} else {
		log.G(ctx).Info("provider does not implement ConfigMapUpdater")
	}

	if su, ok := s.provider.(SecretUpdater); ok {
		go func() {
			chErr <- s.runSecretsController(ctx, su)
		}()
	} else {
		log.G(ctx).Info("provider does not implement SecretUpdater")
	}

	select {
	case <-ctx.Done():
	case err := <-chErr:
		return err
	}

	return nil
}

func (s *Server) runPodController(ctx context.Context, pc *PodController) error {
	q := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "podStatusUpdate")
	go s.runProviderSyncWorkers(ctx, q)

	if pn, ok := s.provider.(providers.PodNotifier); ok {
		pn.NotifyPods(ctx, func(pod *corev1.Pod) {
			s.enqueuePodStatusUpdate(ctx, q, pod)
		})
	} else {
		go s.providerSyncLoop(ctx, q)
	}

	return pc.Run(ctx, s.podSyncWorkers)
}

func (s *Server) runConfigMapController(ctx context.Context, cmu ConfigMapUpdater) error {
	q := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "configMapUpdates")
	return NewConfigMapController(s.configMapInformer, cmu, s.configMapRefs, q).Run(ctx, 1)
}

func (s *Server) runSecretsController(ctx context.Context, su SecretUpdater) error {
	q := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "secretUpdates")
	return NewSecretController(s.secretsInformer, su, s.secretRefs, q).Run(ctx, 1)
}

// providerSyncLoop syncronizes pod states from the provider back to kubernetes
// Deprecated: This is only used when the provider does not support async updates
// Providers should implement async update support, even if it just means copying
// something like this in.
func (s *Server) providerSyncLoop(ctx context.Context, q workqueue.RateLimitingInterface) {
	const sleepTime = 5 * time.Second

	t := time.NewTimer(sleepTime)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			t.Stop()

			ctx, span := trace.StartSpan(ctx, "syncActualState")
			s.updatePodStatuses(ctx, q)
			span.End()

			// restart the timer
			t.Reset(sleepTime)
		}
	}
}

func (s *Server) runProviderSyncWorkers(ctx context.Context, q workqueue.RateLimitingInterface) {
	for i := 0; i < s.podSyncWorkers; i++ {
		go func(index int) {
			workerID := strconv.Itoa(index)
			s.runProviderSyncWorker(ctx, workerID, q)
		}(i)
	}
}

func (s *Server) runProviderSyncWorker(ctx context.Context, workerID string, q workqueue.RateLimitingInterface) {
	for s.processPodStatusUpdate(ctx, workerID, q) {
	}
}

func (s *Server) processPodStatusUpdate(ctx context.Context, workerID string, q workqueue.RateLimitingInterface) bool {
	ctx, span := trace.StartSpan(ctx, "processPodStatusUpdate")
	defer span.End()

	// Add the ID of the current worker as an attribute to the current span.
	ctx = span.WithField(ctx, "workerID", workerID)

	return handleQueueItem(ctx, q, s.podStatusHandler)
}
