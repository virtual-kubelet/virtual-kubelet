package vkubelet

import (
	"context"
	"strconv"
	"time"

	"github.com/virtual-kubelet/virtual-kubelet/manager"
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
	k8sClient       kubernetes.Interface
	provider        PodLifecycleHandler
	resourceManager *manager.ResourceManager
	podSyncWorkers  int
	podInformer     corev1informers.PodInformer
	readyCh         chan struct{}
}

// Config is used to configure a new server.
type Config struct {
	Client          *kubernetes.Clientset
	Namespace       string
	NodeName        string
	Provider        PodLifecycleHandler
	ResourceManager *manager.ResourceManager
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
		nodeName:        cfg.NodeName,
		namespace:       cfg.Namespace,
		k8sClient:       cfg.Client,
		resourceManager: cfg.ResourceManager,
		provider:        cfg.Provider,
		podSyncWorkers:  cfg.PodSyncWorkers,
		podInformer:     cfg.PodInformer,
		readyCh:         make(chan struct{}),
	}
}

// Run creates and starts an instance of the pod controller, blocking until it stops.
//
// Note that this does not setup the HTTP routes that are used to expose pod
// info to the Kubernetes API Server, such as logs, metrics, exec, etc.
// See `AttachPodRoutes` and `AttachMetricsRoutes` to set these up.
func (s *Server) Run(ctx context.Context) error {
	q := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "podStatusUpdate")
	s.runProviderSyncWorkers(ctx, q)

	if pn, ok := s.provider.(PodNotifier); ok {
		pn.NotifyPods(ctx, func(pod *corev1.Pod) {
			s.enqueuePodStatusUpdate(ctx, q, pod)
		})
	} else {
		go s.providerSyncLoop(ctx, q)
	}

	pc := NewPodController(s)

	go func() {
		select {
		case <-pc.inSyncCh:
		case <-ctx.Done():
		}
		close(s.readyCh)
	}()

	return pc.Run(ctx, s.podSyncWorkers)
}

// Ready returns a channel which will be closed once the VKubelet is running
func (s *Server) Ready() <-chan struct{} {
	// TODO: right now all this waits on is the in-sync channel. Later, we might either want to expose multiple types
	// of ready, for example:
	// * In Sync
	// * Control Loop running
	// * Provider state synchronized with API Server state
	return s.readyCh
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
