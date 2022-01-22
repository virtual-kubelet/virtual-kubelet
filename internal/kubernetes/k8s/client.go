package k8s

import (
	"github.com/pkg/errors"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	listerCorev1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type client struct {
	*kubernetes.Clientset

	config    *rest.Config
	Interface kubernetes.Interface
}

var k8sClient *client

type listerSet struct {
	listerCorev1.PodLister
}

var k8sListerSet listerSet

func InitRealKubernetes(stopChan <-chan struct{}, kubeconfig string) error {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return errors.Wrap(err, "BuildConfigFromFlags error")
	}

	cs, err := kubernetes.NewForConfig(config)
	if err != nil {
		return errors.Wrap(err, "NewForConfig error")
	}

	k8sClient = &client{
		Clientset: cs,
		config:    config,
		Interface: cs,
	}

	factory := informers.NewSharedInformerFactory(k8sClient.Interface, 0)
	if err = initPodInformer(stopChan, factory, &k8sListerSet); err != nil {
		return errors.Wrap(err, "initPodInformer error")
	}

	return nil
}

func GetConfig() *rest.Config {
	return k8sClient.config
}

func GetClientSet() *kubernetes.Clientset {
	return k8sClient.Clientset
}
