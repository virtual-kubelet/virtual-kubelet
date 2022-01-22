package k8s

import (
	"context"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
)

func initPodInformer(stopChan <-chan struct{}, factory informers.SharedInformerFactory, ls *listerSet) error {
	informer := factory.Core().V1().Pods().Informer()
	go informer.Run(stopChan)

	if !cache.WaitForCacheSync(stopChan, informer.HasSynced) {
		return errors.Errorf("WaitForCacheSync expect true but got fase")
	}

	ls.PodLister = factory.Core().V1().Pods().Lister()
	return nil
}

func GetPod(namespace, name string) (*corev1.Pod, error) {
	return k8sListerSet.Pods(namespace).Get(name)
}

func ListPodsWithSelector(selector labels.Selector) ([]*corev1.Pod, error) {
	return k8sListerSet.List(selector)
}

func ListPods() ([]*corev1.Pod, error) {
	return ListPodsWithSelector(labels.Everything())
}

func CreatePod(ctx context.Context, namespace string, pod *corev1.Pod) (*corev1.Pod, error) {
	return k8sClient.CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
}

func UpdatePod(ctx context.Context, namespace string, pod *corev1.Pod) (*corev1.Pod, error) {
	return k8sClient.CoreV1().Pods(namespace).Update(ctx, pod, metav1.UpdateOptions{})
}

func DeletePod(ctx context.Context, namespace, name string) error {
	return k8sClient.CoreV1().Pods(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}
