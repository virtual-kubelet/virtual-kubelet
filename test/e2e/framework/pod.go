package framework

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	watchapi "k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/watch"
	podutil "k8s.io/kubernetes/pkg/api/v1/pod"
)

const (
	defaultWatchTimeout       = 2 * time.Minute
	hostnameNodeSelectorLabel = "kubernetes.io/hostname"
)

// CreateDummyPodObjectWithPrefix creates a dujmmy pod object using the specified prefix as the value of .metadata.generateName.
// A variable number of strings can be provided.
// For each one of these strings, a container that uses the string as its image will be appended to the pod.
// This method DOES NOT create the pod in the Kubernetes API.
func (f *Framework) CreateDummyPodObjectWithPrefix(prefix string, images ...string) *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: prefix,
			Namespace:    f.Namespace,
		},
		Spec: corev1.PodSpec{
			NodeSelector: map[string]string{
				hostnameNodeSelectorLabel: f.NodeName,
			},
			Tolerations: []corev1.Toleration{
				{
					Key:    f.TaintKey,
					Value:  f.TaintValue,
					Effect: corev1.TaintEffect(f.TaintEffect),
				},
			},
		},
	}
	for idx, img := range images {
		pod.Spec.Containers = append(pod.Spec.Containers, corev1.Container{
			Name:  fmt.Sprintf("%s%d", prefix, idx),
			Image: img,
		})
	}
	return pod
}

// CreatePod creates the specified pod in the Kubernetes API.
func (f *Framework) CreatePod(pod *corev1.Pod) (*corev1.Pod, error) {
	return f.KubeClient.CoreV1().Pods(f.Namespace).Create(pod)
}

// DeletePod deletes the pod with the specified name and namespace in the Kubernetes API.
func (f *Framework) DeletePod(namespace, name string) error {
	return f.KubeClient.CoreV1().Pods(namespace).Delete(name, &metav1.DeleteOptions{})
}

// WaitUntilPodCondition establishes a watch on the pod with the specified name and namespace.
// Then, it waits for the specified condition function to be verified.
func (f *Framework) WaitUntilPodCondition(namespace, name string, fn watch.ConditionFunc) error {
	// Create a field selector that matches the specified Pod resource.
	fs := fields.ParseSelectorOrDie(fmt.Sprintf("metadata.namespace==%s,metadata.name==%s", namespace, name))
	// Create a ListWatch so we can receive events for the matched Pod resource.
	lw := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			options.FieldSelector = fs.String()
			return f.KubeClient.CoreV1().Pods(namespace).List(options)
		},
		WatchFunc: func(options metav1.ListOptions) (watchapi.Interface, error) {
			options.FieldSelector = fs.String()
			return f.KubeClient.CoreV1().Pods(namespace).Watch(options)
		},
	}
	// Watch for updates to the Pod resource until fn is satisfied, or until the timeout is reached.
	ctx, cfn := context.WithTimeout(context.Background(), defaultWatchTimeout)
	defer cfn()
	last, err := watch.UntilWithSync(ctx, lw, &corev1.Pod{}, nil, fn)
	if err != nil {
		return err
	}
	if last == nil {
		return fmt.Errorf("no events received for pod %q", name)
	}
	return nil
}

// WaitUntilPodReady blocks until the pod with the specified name and namespace is reported to be running and ready.
func (f *Framework) WaitUntilPodReady(namespace, name string) error {
	return f.WaitUntilPodCondition(namespace, name, func(event watchapi.Event) (bool, error) {
		pod := event.Object.(*corev1.Pod)
		return pod.Status.Phase == corev1.PodRunning && podutil.IsPodReady(pod) && pod.Status.PodIP != "", nil
	})
}

// WaitUntilPodDeleted blocks until the pod with the specified name and namespace is marked for deletion (or, alternatively, effectively deleted).
func (f *Framework) WaitUntilPodDeleted(namespace, name string) error {
	return f.WaitUntilPodCondition(namespace, name, func(event watchapi.Event) (bool, error) {
		pod := event.Object.(*corev1.Pod)
		return event.Type == watchapi.Deleted || pod.DeletionTimestamp != nil, nil
	})
}
