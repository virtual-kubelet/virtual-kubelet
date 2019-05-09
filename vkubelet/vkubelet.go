package vkubelet

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/cpuguy83/strongerrors/status/ocstatus"
	"github.com/pkg/errors"
	pkgerrors "github.com/pkg/errors"
	"github.com/virtual-kubelet/virtual-kubelet/log"
	"github.com/virtual-kubelet/virtual-kubelet/manager"
	"github.com/virtual-kubelet/virtual-kubelet/providers"
	"github.com/virtual-kubelet/virtual-kubelet/trace"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	corev1informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// Server masquarades itself as a kubelet and allows for the virtual node to be backed by non-vm/node providers.
type Server struct {
	namespace       string
	nodeName        string
	k8sClient       *kubernetes.Clientset
	provider        providers.Provider
	resourceManager *manager.ResourceManager
	podSyncWorkers  int
	podInformer     corev1informers.PodInformer
	readyCh         chan struct{}

	lock             *sync.Mutex
	podStateMachines map[string]podStateMachineInterface
}

// Config is used to configure a new server.
type Config struct {
	Client          *kubernetes.Clientset
	Namespace       string
	NodeName        string
	Provider        providers.Provider
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

		podStateMachines: make(map[string]podStateMachineInterface),
		lock:             &sync.Mutex{},
	}
}

// Run creates and starts an instance of the pod controller, blocking until it stops.
//
// Note that this does not setup the HTTP routes that are used to expose pod
// info to the Kubernetes API Server, such as logs, metrics, exec, etc.
// See `AttachPodRoutes` and `AttachMetricsRoutes` to set these up.
func (s *Server) Run(ctx context.Context) error {
	pn, ok := s.provider.(providers.PodNotifier)

	if !ok {
		log.G(ctx).Warn("Initializing deprecated provider with synchronous interface")
		ps := &providerSync{p: s.provider}
		pn = ps
		s.provider = ps
	}
	pn.NotifyPods(ctx, func(pod *corev1.Pod) {
		s.handleNotifyPod(ctx, pod)
	})

	pc := NewPodController(s)
	if err := s.createPodStateMachinesFromProvider(ctx, pc); err != nil {
		return err
	}

	go func() {
		select {
		case <-pc.inSyncCh:
		case <-ctx.Done():
		}
		close(s.readyCh)
	}()

	return pc.Run(ctx)
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

// This handles pod state notifications from the provider. These are not de-duped.
func (s *Server) handleNotifyPod(ctx context.Context, pod *corev1.Pod) {
	ctx, span := trace.StartSpan(ctx, "handleNotifyPod")
	defer span.End()

	key, err := cache.MetaNamespaceKeyFunc(pod)
	if err != nil {
		log.G(ctx).WithError(err).Error("Unable to calculate key")
		return
	}

	s.lock.Lock()
	defer s.lock.Unlock()

	if psm, ok := s.podStateMachines[key]; ok {
		psm.updateProviderPod(ctx, pod)
	} else {
		log.G(ctx).Error("Cannot find pod on notify")
	}
}

// This creates (potentially) dangling pods from the provider at startup time. It does this in serial
// so it doesn't need to hold s.lock
func (s *Server) createPodStateMachinesFromProvider(ctx context.Context, pc *PodController) error {
	ctx, span := trace.StartSpan(ctx, "createPodStateMachinesFromProvider")
	defer span.End()

	providerPods, err := s.provider.GetPods(ctx)
	span.SetStatus(ocstatus.FromError(err))
	if err != nil {
		return errors.Wrapf(err, "Could not fetch pods from provider")
	}
	for idx := range providerPods {
		pod := providerPods[idx]
		key, err := cache.MetaNamespaceKeyFunc(pod)
		if err != nil {
			log.G(ctx).WithError(err).Error("Could not get key for pod")
		} else {
			psm := newPodStateMachineFromProvider(ctx, pod, pc, key)
			s.podStateMachines[key] = psm
			go psm.run(ctx)
		}
	}
	return nil
}

func (s *Server) updateRawPodStatus(ctx context.Context, pod *corev1.Pod) (*corev1.Pod, error) {
	ctx, span := trace.StartSpan(ctx, "updateRawPodStatus")

	// Since our patch only applies to the status subtype, we should be safe in doing this
	// We don't really have a better option, as this method is only called from the async pod notifier,
	// which provides (ordered)
	pod.ObjectMeta.ResourceVersion = ""
	patch, err := json.Marshal(pod)
	if err != nil {
		span.SetStatus(ocstatus.FromError(err))
		return nil, pkgerrors.Wrap(err, "Unable to serialize patch JSON")
	}

	newPod, err := s.k8sClient.CoreV1().Pods(pod.Namespace).Patch(pod.Name, types.MergePatchType, patch, "status")
	if err != nil {
		span.SetStatus(ocstatus.FromError(err))
		return nil, pkgerrors.Wrap(err, "error while patching pod status in kubernetes")
	}

	log.G(ctx).WithFields(log.Fields{
		"old phase":  string(pod.Status.Phase),
		"old reason": pod.Status.Reason,
		"new phase":  string(newPod.Status.Phase),
		"new reason": newPod.Status.Reason,
	}).Debug("Updated pod status in kubernetes")

	return newPod, nil
}
