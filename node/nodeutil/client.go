package nodeutil

import (
	"os"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/typed/coordination/v1beta1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// ClientsetFromEnv returns a kuberentes client set from:
// 1. the passed in kubeconfig path
// 2. If the kubeconfig path is empty or non-existent, then the in-cluster config is used.
func ClientsetFromEnv(kubeConfigPath string) (*kubernetes.Clientset, error) {
	var (
		config *rest.Config
		err    error
	)

	if kubeConfigPath != "" {
		if _, err := os.Stat(kubeConfigPath); err != nil {
			config, err = rest.InClusterConfig()
		} else {
			config, err = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
				&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeConfigPath},
				&clientcmd.ConfigOverrides{},
			).ClientConfig()
		}
	} else {
		config, err = rest.InClusterConfig()
	}

	if err != nil {
		return nil, errors.Wrap(err, "error getting rest client config")
	}

	return kubernetes.NewForConfig(config)
}

// PodInformerFilter is a filter that you should use when creating a pod informer for use with the pod controller.
func PodInformerFilter(node string) kubeinformers.SharedInformerOption {
	return kubeinformers.WithTweakListOptions(func(options *metav1.ListOptions) {
		options.FieldSelector = fields.OneTermEqualSelector("spec.nodeName", node).String()
	})
}

// NodeLeaseV1Beta1Client creates a v1beta1 Lease client for use with node leases from the passed in client.
//
// Use this with node.WithNodeEnableLeaseV1Beta1 when creating a node controller.
func NodeLeaseV1Beta1Client(client kubernetes.Interface) v1beta1.LeaseInterface {
	return client.CoordinationV1beta1().Leases(corev1.NamespaceNodeLease)
}
