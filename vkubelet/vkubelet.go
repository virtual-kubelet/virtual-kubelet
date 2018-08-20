package vkubelet

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	pkgerrors "github.com/pkg/errors"
	"github.com/virtual-kubelet/virtual-kubelet/log"
	"github.com/virtual-kubelet/virtual-kubelet/manager"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	PodStatusReason_ProviderFailed = "ProviderFailed"
)

// Server masquarades itself as a kubelet and allows for the virtual node to be backed by non-vm/node providers.
type Server struct {
	nodeName        string
	namespace       string
	k8sClient       *kubernetes.Clientset
	taint           corev1.Taint
	disableTaint    bool
	provider        Provider
	podWatcher      watch.Interface
	resourceManager *manager.ResourceManager
}

func getEnv(key, defaultValue string) string {
	value, found := os.LookupEnv(key)
	if found {
		return value
	}
	return defaultValue
}

// New creates a new virtual-kubelet server.
func New(nodeName, operatingSystem, namespace, kubeConfig, provider, providerConfig, taintKey string, disableTaint bool, metricsAddr string) (*Server, error) {
	var config *rest.Config

	// Check if the kubeConfig file exists.
	if _, err := os.Stat(kubeConfig); !os.IsNotExist(err) {
		// Get the kubeconfig from the filepath.
		config, err = clientcmd.BuildConfigFromFlags("", kubeConfig)
		if err != nil {
			return nil, err
		}
	} else {
		// Set to in-cluster config.
		config, err = rest.InClusterConfig()
		if err != nil {
			return nil, err
		}
	}

	if masterURI := os.Getenv("MASTER_URI"); masterURI != "" {
		config.Host = masterURI
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	rm, err := manager.NewResourceManager(clientset)
	if err != nil {
		return nil, pkgerrors.Wrap(err, "error creating resource manager")
	}

	daemonEndpointPortEnv := os.Getenv("KUBELET_PORT")
	if daemonEndpointPortEnv == "" {
		daemonEndpointPortEnv = "10250"
	}
	i64value, err := strconv.ParseInt(daemonEndpointPortEnv, 10, 32)
	daemonEndpointPort := int32(i64value)

	internalIP := os.Getenv("VKUBELET_POD_IP")

	var defaultTaintKey string
	var defaultTaintValue string
	if taintKey != "" {
		defaultTaintKey = taintKey
		defaultTaintValue = ""
	} else {
		defaultTaintKey = "virtual-kubelet.io/provider"
		defaultTaintValue = provider
	}
	vkTaintKey := getEnv("VKUBELET_TAINT_KEY", defaultTaintKey)
	vkTaintValue := getEnv("VKUBELET_TAINT_VALUE", defaultTaintValue)
	vkTaintEffectEnv := getEnv("VKUBELET_TAINT_EFFECT", "NoSchedule")
	var vkTaintEffect corev1.TaintEffect
	switch vkTaintEffectEnv {
	case "NoSchedule":
		vkTaintEffect = corev1.TaintEffectNoSchedule
	case "NoExecute":
		vkTaintEffect = corev1.TaintEffectNoExecute
	case "PreferNoSchedule":
		vkTaintEffect = corev1.TaintEffectPreferNoSchedule
	default:
		return nil, pkgerrors.Errorf("taint effect %q is not supported", vkTaintEffectEnv)
	}

	taint := corev1.Taint{
		Key:    vkTaintKey,
		Value:  vkTaintValue,
		Effect: vkTaintEffect,
	}

	p, err := lookupProvider(provider, providerConfig, rm, nodeName, operatingSystem, internalIP, daemonEndpointPort)
	if err != nil {
		return nil, err
	}

	s := &Server{
		namespace:       namespace,
		nodeName:        nodeName,
		taint:           taint,
		disableTaint:    disableTaint,
		k8sClient:       clientset,
		resourceManager: rm,
		provider:        p,
	}

	ctx := context.TODO()
	ctx = log.WithLogger(ctx, log.G(ctx))

	if err = s.registerNode(ctx); err != nil {
		return s, err
	}

	go KubeletServerStart(p)

	if metricsAddr != "" {
		go MetricsServerStart(p, metricsAddr)
	} else {
		log.G(ctx).Info("Skipping metrics server startup since no address was provided")
	}

	tick := time.Tick(5 * time.Second)

	go func() {
		for range tick {
			s.updateNode(ctx)
			s.updatePodStatuses(ctx)
		}
	}()

	return s, nil
}

// registerNode registers this virtual node with the Kubernetes API.
func (s *Server) registerNode(ctx context.Context) error {
	taints := make([]corev1.Taint, 0)

	if !s.disableTaint {
		taints = append(taints, s.taint)
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

	if _, err := s.k8sClient.CoreV1().Nodes().Create(node); err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	log.G(ctx).Info("Registered node")

	return nil
}

// Run starts the server, registers it with Kubernetes and begins watching/reconciling the cluster.
// Run will block until Stop is called or a SIGINT or SIGTERM signal is received.
func (s *Server) Run(ctx context.Context) error {
	shouldStop := false

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		shouldStop = true
		s.Stop()
	}()

	for {
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

	loop:
		for {
			select {
			case ev, ok := <-s.podWatcher.ResultChan():
				if !ok {
					if shouldStop {
						log.G(ctx).Info("Pod watcher is stopped")
						return nil
					}

					log.G(ctx).Error("Pod watcher connection is closed unexpectedly")
					break loop
				}

				log.G(ctx).WithField("type", ev.Type).Debug("Pod watcher event is received")
				reconcile := false
				switch ev.Type {
				case watch.Added:
					reconcile = s.resourceManager.UpdatePod(ev.Object.(*corev1.Pod))
				case watch.Modified:
					reconcile = s.resourceManager.UpdatePod(ev.Object.(*corev1.Pod))
				case watch.Deleted:
					reconcile = s.resourceManager.DeletePod(ev.Object.(*corev1.Pod))
				}

				if reconcile {
					s.reconcile(ctx)
				}
			}
		}

		time.Sleep(5 * time.Second)
	}
}

// Stop shutsdown the server.
// It does not shutdown pods assigned to the virtual node.
func (s *Server) Stop() {
	if s.podWatcher != nil {
		s.podWatcher.Stop()
	}
}

// updateNode updates the node status within Kubernetes with updated NodeConditions.
func (s *Server) updateNode(ctx context.Context) {
	opts := metav1.GetOptions{}
	n, err := s.k8sClient.CoreV1().Nodes().Get(s.nodeName, opts)
	if err != nil && !errors.IsNotFound(err) {
		log.G(ctx).WithError(err).Error("Failed to retrieve node")
		return
	}

	if errors.IsNotFound(err) {
		if err = s.registerNode(ctx); err != nil {
			log.G(ctx).WithError(err).Error("Failed to register node")
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
		return
	}
}

// reconcile is the main reconciliation loop that compares differences between Kubernetes and
// the active provider and reconciles the differences.
func (s *Server) reconcile(ctx context.Context) {
	logger := log.G(ctx)
	logger.Debug("Start reconcile")
	defer logger.Debug("End reconcile")

	providerPods, err := s.provider.GetPods(ctx)
	if err != nil {
		logger.WithError(err).Error("Error getting pod list from provider")
		return
	}

	for _, pod := range providerPods {
		// Delete pods that don't exist in Kubernetes
		if p := s.resourceManager.GetPod(pod.Namespace, pod.Name); p == nil || p.DeletionTimestamp != nil {
			logger := logger.WithField("pod", pod.Name)
			logger.Debug("Deleting pod '%s'\n", pod.Name)
			if err := s.deletePod(ctx, pod); err != nil {
				logger.WithError(err).Error("Error deleting pod")
				continue
			}
		}
	}

	// Create any pods for k8s pods that don't exist in the provider
	pods := s.resourceManager.GetPods()
	for _, pod := range pods {
		logger := logger.WithField("pod", pod.Name)
		var providerPod *corev1.Pod
		for _, p := range providerPods {
			if p.Namespace == pod.Namespace && p.Name == pod.Name {
				providerPod = p
				break
			}
		}

		if pod.DeletionTimestamp == nil && pod.Status.Phase != corev1.PodFailed && providerPod == nil {
			logger.Debug("Creating pod")
			if err := s.createPod(ctx, pod); err != nil {
				logger.WithError(err).Error("Error creating pod")
				continue
			}
		}

		// Delete pod if DeletionTimestamp is set
		if pod.DeletionTimestamp != nil {
			log.Trace(logger, "Pod pending deletion")
			var err error
			if err = s.deletePod(ctx, pod); err != nil {
				logger.WithError(err).Error("Error deleting pod")
				continue
			}
			log.Trace(logger, "Pod deletion complete")
		}
	}
}

func (s *Server) createPod(ctx context.Context, pod *corev1.Pod) error {
	if err := s.populateSecretsAndConfigMapsInEnv(pod); err != nil {
		return err
	}

	logger := log.G(ctx).WithField("pod", pod.Name)

	if origErr := s.provider.CreatePod(ctx, pod); origErr != nil {
		pod.ResourceVersion = "" // Blank out resource version to prevent object has been modified error
		pod.Status.Phase = corev1.PodFailed
		pod.Status.Reason = PodStatusReason_ProviderFailed
		pod.Status.Message = origErr.Error()

		_, err := s.k8sClient.CoreV1().Pods(pod.Namespace).UpdateStatus(pod)
		if err != nil {
			logger.WithError(err).Warn("Failed to update pod status")
		}

		return origErr
	}

	logger.Info("Pod created")

	return nil
}

func (s *Server) deletePod(ctx context.Context, pod *corev1.Pod) error {
	var delErr error
	if delErr = s.provider.DeletePod(ctx, pod); delErr != nil && errors.IsNotFound(delErr) {
		return delErr
	}

	logger := log.G(ctx).WithField("pod", pod.Name)
	if !errors.IsNotFound(delErr) {
		var grace int64
		if err := s.k8sClient.CoreV1().Pods(pod.Namespace).Delete(pod.Name, &metav1.DeleteOptions{GracePeriodSeconds: &grace}); err != nil && errors.IsNotFound(err) {
			if errors.IsNotFound(err) {
				logger.Error("Pod doesn't exist")
				return nil
			}

			return fmt.Errorf("Failed to delete kubernetes pod: %s", err)
		}

		s.resourceManager.DeletePod(pod)
		logger.Info("Pod deleted")
	}

	return nil
}

// updatePodStatuses syncs the providers pod status with the kubernetes pod status.
func (s *Server) updatePodStatuses(ctx context.Context) {
	// Update all the pods with the provider status.
	pods := s.resourceManager.GetPods()
	for _, pod := range pods {
		if pod.Status.Phase == corev1.PodSucceeded || (pod.Status.Phase == corev1.PodFailed && pod.Status.Reason == PodStatusReason_ProviderFailed) {
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
