package vkubelet

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/virtual-kubelet/virtual-kubelet/manager"
	"github.com/virtual-kubelet/virtual-kubelet/providers/aws"
	"github.com/virtual-kubelet/virtual-kubelet/providers/azure"
	"github.com/virtual-kubelet/virtual-kubelet/providers/azurebatch"
	"github.com/virtual-kubelet/virtual-kubelet/providers/cri"
	"github.com/virtual-kubelet/virtual-kubelet/providers/huawei"
	"github.com/virtual-kubelet/virtual-kubelet/providers/hypersh"
	"github.com/virtual-kubelet/virtual-kubelet/providers/mock"
	"github.com/virtual-kubelet/virtual-kubelet/providers/vic"
	"github.com/virtual-kubelet/virtual-kubelet/providers/web"
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
	taint           string
	provider        Provider
	podWatcher      watch.Interface
	resourceManager *manager.ResourceManager
}

// New creates a new virtual-kubelet server.
func New(nodeName, operatingSystem, namespace, kubeConfig, taint, provider, providerConfig string) (*Server, error) {
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

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	rm := manager.NewResourceManager(clientset)

	daemonEndpointPortEnv := os.Getenv("KUBELET_PORT")
	if daemonEndpointPortEnv == "" {
		daemonEndpointPortEnv = "10250"
	}
	i64value, err := strconv.ParseInt(daemonEndpointPortEnv, 10, 32)
	daemonEndpointPort := int32(i64value)

	internalIP := os.Getenv("VKUBELET_POD_IP")

	var p Provider
	switch provider {
	case "aws":
		p, err = aws.NewFargateProvider(providerConfig, rm, nodeName, operatingSystem, internalIP, daemonEndpointPort)
		if err != nil {
			return nil, err
		}
	case "azure":
		p, err = azure.NewACIProvider(providerConfig, rm, nodeName, operatingSystem, internalIP, daemonEndpointPort)
		if err != nil {
			return nil, err
		}
	case "azurebatch":
		p, err = azurebatch.NewBatchProvider(providerConfig, rm, nodeName, operatingSystem, internalIP, daemonEndpointPort)
		if err != nil {
			return nil, err
		}
	case "hyper":
		p, err = hypersh.NewHyperProvider(providerConfig, rm, nodeName, operatingSystem)
		if err != nil {
			return nil, err
		}
	case "vic":
		p, err = vic.NewVicProvider(providerConfig, rm, nodeName, operatingSystem)
		if err != nil {
			return nil, err
		}
	case "web":
		p, err = web.NewBrokerProvider(nodeName, operatingSystem, daemonEndpointPort)
		if err != nil {
			return nil, err
		}
	case "mock":
		p, err = mock.NewMockProvider(nodeName, operatingSystem, internalIP, daemonEndpointPort)
		if err != nil {
			return nil, err
		}
	case "cri":
		p, err = cri.NewCRIProvider(nodeName, operatingSystem, internalIP, rm, daemonEndpointPort)
		if err != nil {
			return nil, err
		}
	case "huawei":
		p, err = huawei.NewCCIProvider(providerConfig, rm, nodeName, operatingSystem, internalIP, daemonEndpointPort)
		if err != nil {
			return nil, err
		}
	default:
		fmt.Printf("Provider '%s' is not supported\n", provider)
	}

	s := &Server{
		namespace:       namespace,
		nodeName:        nodeName,
		taint:           taint,
		k8sClient:       clientset,
		resourceManager: rm,
		provider:        p,
	}

	if err = s.registerNode(); err != nil {
		return s, err
	}

	go ApiserverStart(p)

	tick := time.Tick(5 * time.Second)
	go func() {
		for range tick {
			s.updateNode()
			s.updatePodStatuses()
		}
	}()

	return s, nil
}

// registerNode registers this virtual node with the Kubernetes API.
func (s *Server) registerNode() error {
	taints := make([]corev1.Taint, 0)

	if s.taint != "" {
		taints = append(taints, corev1.Taint{
			Key:    s.taint,
			Effect: corev1.TaintEffectNoSchedule,
		})
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
				KubeletVersion:  "v1.8.3",
			},
			Capacity:        s.provider.Capacity(),
			Allocatable:     s.provider.Capacity(),
			Conditions:      s.provider.NodeConditions(),
			Addresses:       s.provider.NodeAddresses(),
			DaemonEndpoints: *s.provider.NodeDaemonEndpoints(),
		},
	}

	if _, err := s.k8sClient.CoreV1().Nodes().Create(node); err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	log.Printf("Node '%s' with OS type '%s' registered\n", node.Name, node.Status.NodeInfo.OperatingSystem)

	return nil
}

// Run starts the server, registers it with Kubernetes and begins watching/reconciling the cluster.
// Run will block until Stop is called or a SIGINT or SIGTERM signal is received.
func (s *Server) Run() error {
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
			log.Fatal("Failed to list pods", err)
		}
		s.resourceManager.SetPods(pods)
		s.reconcile()

		opts.ResourceVersion = pods.ResourceVersion
		s.podWatcher, err = s.k8sClient.CoreV1().Pods(s.namespace).Watch(opts)
		if err != nil {
			log.Fatal("Failed to watch pods", err)
		}

		loop:
		for {
			select {
			case ev, ok := <-s.podWatcher.ResultChan():
				if !ok {
					if shouldStop {
						log.Println("Pod watcher is stopped.")
						return nil
					}

					log.Println("Pod watcher connection is closed unexpectedly.")
					break loop
				}

				log.Println("Pod watcher event is received:", ev.Type)
				switch ev.Type {
				case watch.Added:
					s.resourceManager.AddPod(ev.Object.(*corev1.Pod))
				case watch.Modified:
					s.resourceManager.UpdatePod(ev.Object.(*corev1.Pod))
				case watch.Deleted:
					s.resourceManager.DeletePod(ev.Object.(*corev1.Pod))
				}
				s.reconcile()
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
func (s *Server) updateNode() {
	opts := metav1.GetOptions{}
	n, err := s.k8sClient.CoreV1().Nodes().Get(s.nodeName, opts)
	if err != nil {
		log.Println("Failed to retrieve node:", err)
		return
	}

	n.ResourceVersion = "" // Blank out resource version to prevent object has been modified error
	n.Status.Conditions = s.provider.NodeConditions()

	capacity := s.provider.Capacity()
	n.Status.Capacity = capacity
	n.Status.Allocatable = capacity

	n.Status.Addresses = s.provider.NodeAddresses()
	
	n, err = s.k8sClient.CoreV1().Nodes().UpdateStatus(n)
	if err != nil {
		log.Println("Failed to update node:", err)
		return
	}
}

// reconcile is the main reconciliation loop that compares differences between Kubernetes and
// the active provider and reconciles the differences.
func (s *Server) reconcile() {
	providerPods, err := s.provider.GetPods()
	if err != nil {
		log.Println(err)
		return
	}

	for _, pod := range providerPods {
		// Delete pods that don't exist in Kubernetes
		if p := s.resourceManager.GetPod(pod.Namespace, pod.Name); p == nil {
			if err := s.deletePod(pod); err != nil {
				log.Printf("Error deleting pod '%s': %s\n", pod.Name, err)
				continue
			}
		}
	}

	// Create any pods for k8s pods that don't exist in the provider
	pods := s.resourceManager.GetPods()
	for _, pod := range pods {
		p, err := s.provider.GetPod(pod.Namespace, pod.Name)
		if err != nil {
			log.Printf("Error retrieving pod '%s' from provider: %s\n", pod.Name, err)
		}

		if pod.DeletionTimestamp == nil && pod.Status.Phase != corev1.PodFailed && p == nil {
			if err := s.createPod(pod); err != nil {
				log.Printf("Error creating pod '%s': %s\n", pod.Name, err)
				continue
			}
			log.Printf("Pod '%s' created.\n", pod.Name)
		}

		// Delete pod if DeletionTimestamp set
		if pod.DeletionTimestamp != nil {
			var err error
			if err = s.deletePod(pod); err != nil {
				log.Printf("Error deleting pod '%s': %s\n", pod.Name, err)
				continue
			}
		}
	}
}

func (s *Server) createPod(pod *corev1.Pod) error {
	if err := s.populateSecretsAndConfigMapsInEnv(pod); err != nil {
		return err
	}

	if origErr := s.provider.CreatePod(pod); origErr != nil {
		pod.ResourceVersion = "" // Blank out resource version to prevent object has been modified error
		pod.Status.Phase = corev1.PodFailed
		pod.Status.Reason = PodStatusReason_ProviderFailed
		pod.Status.Message = origErr.Error()

		_, err := s.k8sClient.CoreV1().Pods(pod.Namespace).UpdateStatus(pod)
		if err != nil {
			log.Println("Failed to update pod status:", err)
			return origErr
		}

		return origErr
	}

	return nil
}

func (s *Server) deletePod(pod *corev1.Pod) error {
	var delErr error
	if delErr = s.provider.DeletePod(pod); delErr != nil && errors.IsNotFound(delErr) {
		return fmt.Errorf("Error deleting pod '%s': %s", pod.Name, delErr)
	}

	if !errors.IsNotFound(delErr) {
		var grace int64
		if err := s.k8sClient.CoreV1().Pods(pod.Namespace).Delete(pod.Name, &metav1.DeleteOptions{GracePeriodSeconds: &grace}); err != nil && errors.IsNotFound(err) {
			if errors.IsNotFound(err) {
				return nil
			}

			return fmt.Errorf("Failed to delete kubernetes pod: %s", err)
		}

		log.Printf("Pod '%s' deleted.\n", pod.Name)
	}

	return nil
}

// updatePodStatuses syncs the providers pod status with the kubernetes pod status.
func (s *Server) updatePodStatuses() {
	// Update all the pods with the provider status.
	pods := s.resourceManager.GetPods()
	for _, pod := range pods {
		if pod.DeletionTimestamp != nil && pod.Status.Phase == corev1.PodSucceeded {
			continue
		}

		if pod.Status.Phase == corev1.PodFailed && pod.Status.Reason == PodStatusReason_ProviderFailed {
			continue
		}

		status, err := s.provider.GetPodStatus(pod.Namespace, pod.Name)
		if err != nil {
			log.Printf("Error retrieving pod '%s' status from provider: %s\n", pod.Name, err)
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
