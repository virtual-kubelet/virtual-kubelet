package framework

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Framework encapsulates the configuration for the current run, and provides helper methods to be used during testing.
type Framework struct {
	KubeClient kubernetes.Interface
	Namespace  string
	NodeName   string
}

// NewTestingFramework returns a new instance of the testing framework.
func NewTestingFramework(kubeconfig, namespace, nodeName string) *Framework {
	return &Framework{
		KubeClient: createKubeClient(kubeconfig),
		Namespace:  namespace,
		NodeName:   nodeName,
	}
}

// createKubeClient creates a new Kubernetes client based on the specified kubeconfig file.
// If no value for kubeconfig is specified, in-cluster configuration is assumed.
func createKubeClient(kubeconfig string) *kubernetes.Clientset {
	var (
		cfg *rest.Config
		err error
	)
	if kubeconfig == "" {
		cfg, err = rest.InClusterConfig()
	} else {
		cfg, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	}
	if err != nil {
		panic(err)
	}
	return kubernetes.NewForConfigOrDie(cfg)
}
