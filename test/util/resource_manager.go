package util

import (
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"

	"github.com/virtual-kubelet/virtual-kubelet/manager"
)

// FakeResourceManager returns an instance of the resource manager that will return the specified objects when its "GetX" methods are called.
// Objects can be any valid Kubernetes object (corev1.Pod, corev1.ConfigMap, corev1.Secret, ...).
func FakeResourceManager(objects ...runtime.Object) *manager.ResourceManager {
	// Create a fake Kubernetes client that will list the specified objects.
	kubeClient := fake.NewSimpleClientset(objects...)
	// Create a shared informer factory from where we can grab informers and listers for pods, configmaps, secrets and services.
	kubeInformerFactory := informers.NewSharedInformerFactory(kubeClient, 30*time.Second)
	// Grab informers for pods, configmaps and secrets.
	pInformer := kubeInformerFactory.Core().V1().Pods()
	mInformer := kubeInformerFactory.Core().V1().ConfigMaps()
	sInformer := kubeInformerFactory.Core().V1().Secrets()
	svcInformer := kubeInformerFactory.Core().V1().Services()
	// Start all the required informers.
	go pInformer.Informer().Run(wait.NeverStop)
	go mInformer.Informer().Run(wait.NeverStop)
	go sInformer.Informer().Run(wait.NeverStop)
	go svcInformer.Informer().Run(wait.NeverStop)
	// Wait for the caches to be synced.
	if !cache.WaitForCacheSync(wait.NeverStop, pInformer.Informer().HasSynced, mInformer.Informer().HasSynced, sInformer.Informer().HasSynced, svcInformer.Informer().HasSynced) {
		panic("failed to wait for caches to be synced")
	}
	// Create a new instance of the resource manager using the listers for pods, configmaps and secrets.
	r, err := manager.NewResourceManager(pInformer.Lister(), sInformer.Lister(), mInformer.Lister(), svcInformer.Lister())
	if err != nil {
		panic(err)
	}
	return r
}
