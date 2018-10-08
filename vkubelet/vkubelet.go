package vkubelet

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	pkgerrors "github.com/pkg/errors"
	"github.com/virtual-kubelet/virtual-kubelet/log"
	"github.com/virtual-kubelet/virtual-kubelet/manager"
	"github.com/virtual-kubelet/virtual-kubelet/providers"
	"go.opencensus.io/trace"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
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
	resourceManager *manager.ResourceManager
	podCreationCh   chan *corev1.Pod
	podDeletionCh   chan *corev1.Pod
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
		podCreationCh:   make(chan *corev1.Pod, 1024),
		podDeletionCh:   make(chan *corev1.Pod, 1024),
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
			ctx, span := trace.StartSpan(ctx, "reconciliationTick")
			s.updateNode(ctx)
			s.updatePodStatuses(ctx)
			span.End()
		}
	}()

	return s, nil
}

// registerNode registers this virtual node with the Kubernetes API.
func (s *Server) registerNode(ctx context.Context) error {
	ctx, span := trace.StartSpan(ctx, "registerNode")
	defer span.End()

	taints := make([]corev1.Taint, 0)

	if s.taint != nil {
		taints = append(taints, *s.taint)
	}

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: s.nodeName,
			Labels: map[string]string{
				"type":                                                    "virtual-kubelet",
				"kubernetes.io/role":                                      "agent",
				"beta.kubernetes.io/os":                                   strings.ToLower(s.provider.OperatingSystem()),
				"kubernetes.io/hostname":                                  s.nodeName,
				"alpha.service-controller.kubernetes.io/exclude-balancer": "true",
			},
		},
		Spec: corev1.NodeSpec{
			Taints: taints,
		},
		Status: corev1.NodeStatus{
			NodeInfo: corev1.NodeSystemInfo{
				OperatingSystem: s.provider.OperatingSystem(),
				Architecture:    "amd64",
				KubeletVersion:  "v1.11.2",
			},
			Capacity:        s.provider.Capacity(ctx),
			Allocatable:     s.provider.Capacity(ctx),
			Conditions:      s.provider.NodeConditions(ctx),
			Addresses:       s.provider.NodeAddresses(ctx),
			DaemonEndpoints: *s.provider.NodeDaemonEndpoints(ctx),
		},
	}
	addNodeAttributes(span, node)
	if _, err := s.k8sClient.CoreV1().Nodes().Create(node); err != nil && !errors.IsAlreadyExists(err) {
		span.SetStatus(trace.Status{Code: trace.StatusCodeUnknown, Message: err.Error()})
		return err
	}
	span.Annotate(nil, "Registered node with k8s")

	log.G(ctx).Info("Registered node")

	return nil
}

// Run starts the server, registers it with Kubernetes and begins watching/reconciling the cluster.
// Run will block until Stop is called or a SIGINT or SIGTERM signal is received.
func (s *Server) Run(ctx context.Context) {
	for i := 0; i < 10; i++ {
		go s.startPodCreator(ctx, i)
		go s.startPodTerminator(ctx, i)
	}

	var controller cache.Controller
	_, controller = cache.NewInformer(

		&cache.ListWatch{

			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				opts := metav1.ListOptions{
					FieldSelector: fields.OneTermEqualSelector("spec.nodeName", s.nodeName).String(),
				}

				if controller != nil {
					opts.ResourceVersion = controller.LastSyncResourceVersion()
				}

				return s.k8sClient.Core().Pods(s.namespace).List(opts)
			},

			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				opts := metav1.ListOptions{
					FieldSelector: fields.OneTermEqualSelector("spec.nodeName", s.nodeName).String(),
				}

				if controller != nil {
					opts.ResourceVersion = controller.LastSyncResourceVersion()
				}

				return s.k8sClient.Core().Pods(s.namespace).Watch(opts)
			},
		},

		&corev1.Pod{},

		1*time.Minute,

		cache.ResourceEventHandlerFuncs{
			AddFunc:    func(obj interface{}) {
				s.onAddPod(ctx, obj)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				s.onUpdatePod(ctx, newObj)
			},
			DeleteFunc: func(obj interface{}) {
				s.onDeletePod(ctx, obj)
			},
		},
	)

	stopCh := make(chan struct{})

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		log.G(ctx).Info("Stop pod cache controller.")
		close(stopCh)
	}()

	log.G(ctx).Info("Start to run pod cache controller.")
	controller.Run(stopCh)
}

type taintsStringer []corev1.Taint

func (t taintsStringer) String() string {
	var s string
	for _, taint := range t {
		if s == "" {
			s = taint.Key + "=" + taint.Value + ":" + string(taint.Effect)
		} else {
			s += ", " + taint.Key + "=" + taint.Value + ":" + string(taint.Effect)
		}
	}
	return s
}

func addNodeAttributes(span *trace.Span, n *corev1.Node) {
	span.AddAttributes(
		trace.StringAttribute("UID", string(n.UID)),
		trace.StringAttribute("name", n.Name),
		trace.StringAttribute("cluster", n.ClusterName),
	)
	if span.IsRecordingEvents() {
		span.AddAttributes(trace.StringAttribute("taints", taintsStringer(n.Spec.Taints).String()))
	}
}

// updateNode updates the node status within Kubernetes with updated NodeConditions.
func (s *Server) updateNode(ctx context.Context) {
	ctx, span := trace.StartSpan(ctx, "updateNode")
	defer span.End()

	opts := metav1.GetOptions{}
	n, err := s.k8sClient.CoreV1().Nodes().Get(s.nodeName, opts)
	if err != nil && !errors.IsNotFound(err) {
		log.G(ctx).WithError(err).Error("Failed to retrieve node")
		span.SetStatus(trace.Status{Code: trace.StatusCodeUnknown, Message: err.Error()})
		return
	}
	addNodeAttributes(span, n)
	span.Annotate(nil, "Fetched node details from k8s")

	if errors.IsNotFound(err) {
		if err = s.registerNode(ctx); err != nil {
			log.G(ctx).WithError(err).Error("Failed to register node")
			span.SetStatus(trace.Status{Code: trace.StatusCodeUnknown, Message: err.Error()})
		} else {
			span.Annotate(nil, "Registered node in k8s")
		}
		return
	}

	n.ResourceVersion = "" // Blank out resource version to prevent object has been modified error
	n.Status.Conditions = s.provider.NodeConditions(ctx)

	capacity := s.provider.Capacity(ctx)
	n.Status.Capacity = capacity
	n.Status.Allocatable = capacity

	n.Status.Addresses = s.provider.NodeAddresses(ctx)

	n, err = s.k8sClient.CoreV1().Nodes().UpdateStatus(n)
	if err != nil {
		log.G(ctx).WithError(err).Error("Failed to update node")
		span.SetStatus(trace.Status{Code: trace.StatusCodeUnknown, Message: err.Error()})
		return
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

func addPodAttributes(span *trace.Span, pod *corev1.Pod) {
	span.AddAttributes(
		trace.StringAttribute("uid", string(pod.UID)),
		trace.StringAttribute("namespace", pod.Namespace),
		trace.StringAttribute("name", pod.Name),
	)
}

func (s *Server) onAddPod(ctx context.Context, obj interface{}) {
	pod := obj.(*corev1.Pod)

	logger := log.G(ctx).WithField("method", "onAddPod")
	if pod == nil {
		logger.Error("obj is not a valid pod:", obj)
		return
	}

	if !s.resourceManager.UpdatePod(pod) {
		logger.Infof("Pod '%s/%s' is already added", pod.GetNamespace(), pod.GetName())
		return
	}

	if pod.DeletionTimestamp != nil {
		logger.Infof("Deleting pod '%s/%s'", pod.GetNamespace(), pod.GetName())
		s.podDeletionCh <- pod
	} else {
		logger.Infof("Adding pod '%s/%s'", pod.GetNamespace(), pod.GetName())
		s.podCreationCh <- pod
	}
}

func (s *Server) onUpdatePod(ctx context.Context, obj interface{}) {
	pod := obj.(*corev1.Pod)

	logger := log.G(ctx).WithField("method", "onUpdatePod")
	if pod == nil {
		logger.Error("obj is not a valid pod:", obj)
		return
	}

	if !s.resourceManager.UpdatePod(pod) {
		logger.Infof("Pod '%s/%s' is already updated", pod.GetNamespace(), pod.GetName())
		return
	}

	if pod.DeletionTimestamp != nil {
		logger.Infof("Deleting pod '%s/%s'", pod.GetNamespace(), pod.GetName())
		s.podDeletionCh <- pod
	} else {
		logger.Infof("Adding pod '%s/%s'", pod.GetNamespace(), pod.GetName())
		s.podCreationCh <- pod
	}
}

func (s *Server) onDeletePod(ctx context.Context, obj interface{}) {
	pod := obj.(*corev1.Pod)

	logger := log.G(ctx).WithField("method", "onDeletePod")
	if pod == nil {
		logger.Errorf("obj is not a valid pod: %v", obj)
		return
	}

	if !s.resourceManager.DeletePod(pod) {
		logger.Infof("Pod '%s/%s' is already deleted", pod.GetNamespace(), pod.GetName())
		return
	}

	logger.Infof("Deleting pod '%s/%s'", pod.GetNamespace(), pod.GetName())
	s.podDeletionCh <- pod
}

func (s *Server) startPodCreator(ctx context.Context, id int) {
	logger := log.G(ctx).WithField("podCreator", id)
	logger.Info("Start pod creator")

	for p := range s.podCreationCh {
		logger.Infof("Creating pod '%s/%s' ", p.GetNamespace(), p.GetName())
		if err := s.createPod(ctx, p); err != nil {
			logger.WithError(err).Errorf("Failed to create pod '%s/%s'", p.GetNamespace(), p.GetName())
		}
	}
}

func (s *Server) startPodTerminator(ctx context.Context, id int) {
	logger := log.G(ctx).WithField("podTerminator", id)
	logger.Info("Start pod terminator")

	for p := range s.podDeletionCh {
		logger.Infof("Deleting pod '%s/%s' ", p.GetNamespace(), p.GetName())
		if err := s.deletePod(ctx, p); err != nil {
			logger.WithError(err).Errorf("Failed to delete pod '%s/%s'", p.GetNamespace(), p.GetName())
		}
	}
}

func (s *Server) createPod(ctx context.Context, pod *corev1.Pod) error {
	ctx, span := trace.StartSpan(ctx, "createPod")
	defer span.End()
	addPodAttributes(span, pod)

	if err := s.populateSecretsAndConfigMapsInEnv(pod); err != nil {
		span.SetStatus(trace.Status{Code: trace.StatusCodeInvalidArgument, Message: err.Error()})
		return err
	}

	logger := log.G(ctx).WithField("pod", pod.GetName()).WithField("namespace", pod.GetNamespace())

	if origErr := s.provider.CreatePod(ctx, pod); origErr != nil {
		podPhase := corev1.PodPending
		if pod.Spec.RestartPolicy == corev1.RestartPolicyNever {
			podPhase = corev1.PodFailed
		}

		pod.ResourceVersion = "" // Blank out resource version to prevent object has been modified error
		pod.Status.Phase = podPhase
		pod.Status.Reason = PodStatusReason_ProviderFailed
		pod.Status.Message = origErr.Error()

		_, err := s.k8sClient.CoreV1().Pods(pod.Namespace).UpdateStatus(pod)
		if err != nil {
			logger.WithError(err).Warn("Failed to update pod status")
		} else {
			span.Annotate(nil, "Updated k8s pod status")
		}

		span.SetStatus(trace.Status{Code: trace.StatusCodeUnknown, Message: origErr.Error()})
		return origErr
	}
	span.Annotate(nil, "Created pod in provider")

	logger.Info("Pod created")

	return nil
}

func (s *Server) deletePod(ctx context.Context, pod *corev1.Pod) error {
	ctx, span := trace.StartSpan(ctx, "deletePod")
	defer span.End()
	addPodAttributes(span, pod)

	var delErr error
	if delErr = s.provider.DeletePod(ctx, pod); delErr != nil && errors.IsNotFound(delErr) {
		span.SetStatus(trace.Status{Code: trace.StatusCodeUnknown, Message: delErr.Error()})
		return delErr
	}
	span.Annotate(nil, "Deleted pod from provider")

	logger := log.G(ctx).WithField("pod", pod.Name)
	if !errors.IsNotFound(delErr) {
		var grace int64
		if err := s.k8sClient.CoreV1().Pods(pod.Namespace).Delete(pod.Name, &metav1.DeleteOptions{GracePeriodSeconds: &grace}); err != nil && errors.IsNotFound(err) {
			if errors.IsNotFound(err) {
				span.Annotate(nil, "Pod does not exist in k8s, nothing to delete")
				return nil
			}

			span.SetStatus(trace.Status{Code: trace.StatusCodeUnknown, Message: err.Error()})
			return fmt.Errorf("Failed to delete kubernetes pod: %s", err)
		}
		span.Annotate(nil, "Deleted pod from k8s")

		s.resourceManager.DeletePod(pod)
		span.Annotate(nil, "Deleted pod from internal state")
		logger.Info("Pod deleted")
	}

	return nil
}

// updatePodStatuses syncs the providers pod status with the kubernetes pod status.
func (s *Server) updatePodStatuses(ctx context.Context) {
	ctx, span := trace.StartSpan(ctx, "updatePodStatuses")
	defer span.End()

	// Update all the pods with the provider status.
	pods := s.resourceManager.GetPods()
	span.AddAttributes(trace.Int64Attribute("nPods", int64(len(pods))))

	for _, pod := range pods {
		if pod.Status.Phase == corev1.PodSucceeded ||
			pod.Status.Phase == corev1.PodFailed ||
			pod.Status.Reason == PodStatusReason_ProviderFailed {
			continue
		}

		status, err := s.provider.GetPodStatus(ctx, pod.Namespace, pod.Name)
		if err != nil {
			log.G(ctx).WithField("pod", pod.Name).Error("Error retrieving pod status")
			return
		}

		// Update the pod's status
		if status != nil {
			pod.Status = *status
			s.k8sClient.CoreV1().Pods(pod.Namespace).UpdateStatus(pod)
		}
	}
}

// populateSecretsAndConfigMapsInEnv populates Secrets and ConfigMap into environment variables
func (s *Server) populateSecretsAndConfigMapsInEnv(pod *corev1.Pod) error {
	for _, c := range pod.Spec.Containers {
		for i, e := range c.Env {
			if e.ValueFrom != nil {
				// Populate ConfigMaps to Env
				if e.ValueFrom.ConfigMapKeyRef != nil {
					vf := e.ValueFrom.ConfigMapKeyRef
					cm, err := s.resourceManager.GetConfigMap(vf.Name, pod.Namespace)
					if vf.Optional != nil && !*vf.Optional && errors.IsNotFound(err) {
						return fmt.Errorf("ConfigMap %s is required by Pod %s and does not exist", vf.Name, pod.Name)
					}

					if err != nil {
						return fmt.Errorf("Error retrieving ConfigMap %s required by Pod %s: %s", vf.Name, pod.Name, err)
					}

					var ok bool
					if c.Env[i].Value, ok = cm.Data[vf.Key]; !ok {
						return fmt.Errorf("ConfigMap %s key %s is required by Pod %s and does not exist", vf.Name, vf.Key, pod.Name)
					}
					continue
				}

				// Populate Secrets to Env
				if e.ValueFrom.SecretKeyRef != nil {
					vf := e.ValueFrom.SecretKeyRef
					sec, err := s.resourceManager.GetSecret(vf.Name, pod.Namespace)
					if vf.Optional != nil && !*vf.Optional && errors.IsNotFound(err) {
						return fmt.Errorf("Secret %s is required by Pod %s and does not exist", vf.Name, pod.Name)
					}
					v, ok := sec.Data[vf.Key]
					if !ok {
						return fmt.Errorf("Secret %s key %s is required by Pod %s and does not exist", vf.Name, vf.Key, pod.Name)
					}
					c.Env[i].Value = string(v)
					continue
				}

				// TODO: Populate Downward API to Env
				if e.ValueFrom.FieldRef != nil {
					continue
				}

				// TODO: Populate resource requests
				if e.ValueFrom.ResourceFieldRef != nil {
					continue
				}
			}
		}
	}

	return nil
}
