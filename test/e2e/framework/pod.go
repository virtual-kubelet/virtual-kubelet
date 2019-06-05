package framework

import (
	"context"
	"fmt"
	"strings"
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

const defaultWatchTimeout = 2 * time.Minute

// CreateDummyPodObjectWithPrefix creates a dujmmy pod object using the specified prefix as the value of .metadata.generateName.
// A variable number of strings can be provided.
// For each one of these strings, a container that uses the string as its image will be appended to the pod.
// This method DOES NOT create the pod in the Kubernetes API.
func (f *Framework) CreateDummyPodObjectWithPrefix(testName string, prefix string, images ...string) *corev1.Pod {
	// Safe the test name
	if testName != "" {
		testName = strings.Replace(testName, "/", "-", -1)
		testName = strings.ToLower(testName)
		prefix = prefix + "-" + testName + "-"
	}
	enableServiceLink := false

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: prefix,
			Namespace:    f.Namespace,
		},
		Spec: corev1.PodSpec{
			NodeName:           f.NodeName,
			EnableServiceLinks: &enableServiceLink,
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

// DeletePod deletes the pod with the specified name and namespace in the Kubernetes API using the default grace period.
func (f *Framework) DeletePod(namespace, name string) error {
	return f.KubeClient.CoreV1().Pods(namespace).Delete(name, &metav1.DeleteOptions{})
}

// DeletePodImmediately forcibly deletes the pod with the specified name and namespace in the Kubernetes API.
// This is equivalent to running "kubectl delete --force --grace-period 0 --namespace <namespace> pod <name>".
func (f *Framework) DeletePodImmediately(namespace, name string) error {
	grace := int64(0)
	propagation := metav1.DeletePropagationBackground
	return f.KubeClient.CoreV1().Pods(namespace).Delete(name, &metav1.DeleteOptions{
		GracePeriodSeconds: &grace,
		PropagationPolicy:  &propagation,
	})
}

// WaitUntilPodCondition establishes a watch on the pod with the specified name and namespace.
// Then, it waits for the specified condition function to be verified.
func (f *Framework) WaitUntilPodCondition(namespace, name string, fn watch.ConditionFunc) (*corev1.Pod, error) {
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
		return nil, err
	}
	if last == nil {
		return nil, fmt.Errorf("no events received for pod %q", name)
	}
	pod := last.Object.(*corev1.Pod)
	return pod, nil
}

// WaitUntilPodReady blocks until the pod with the specified name and namespace is reported to be running and ready.
func (f *Framework) WaitUntilPodReady(namespace, name string) (*corev1.Pod, error) {
	return f.WaitUntilPodCondition(namespace, name, func(event watchapi.Event) (bool, error) {
		pod := event.Object.(*corev1.Pod)
		return pod.Status.Phase == corev1.PodRunning && podutil.IsPodReady(pod) && pod.Status.PodIP != "", nil
	})
}

// WaitUntilPodDeleted blocks until the pod with the specified name and namespace is deleted from apiserver.
func (f *Framework) WaitUntilPodDeleted(namespace, name string) (*corev1.Pod, error) {
	return f.WaitUntilPodCondition(namespace, name, func(event watchapi.Event) (bool, error) {
		pod := event.Object.(*corev1.Pod)
		return event.Type == watchapi.Deleted || pod.ObjectMeta.DeletionTimestamp != nil, nil
	})
}

// WaitUntilPodInPhase blocks until the pod with the specified name and namespace is in one of the specified phases
func (f *Framework) WaitUntilPodInPhase(namespace, name string, phases ...corev1.PodPhase) (*corev1.Pod, error) {
	return f.WaitUntilPodCondition(namespace, name, func(event watchapi.Event) (bool, error) {
		pod := event.Object.(*corev1.Pod)
		for _, p := range phases {
			if pod.Status.Phase == p {
				return true, nil
			}
		}
		return false, nil
	})
}

// WaitUntilPodEventWithReason establishes a watch on events involving the specified pod.
// Then, it waits for an event with the specified reason to be created/updated.
func (f *Framework) WaitUntilPodEventWithReason(pod *corev1.Pod, reason string) error {
	// Create a field selector that matches Event resources involving the specified pod.
	fs := fields.ParseSelectorOrDie(fmt.Sprintf("involvedObject.kind==Pod,involvedObject.uid==%s", pod.UID))
	// Create a ListWatch so we can receive events for the matched Event resource.
	lw := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			options.FieldSelector = fs.String()
			return f.KubeClient.CoreV1().Events(pod.Namespace).List(options)
		},
		WatchFunc: func(options metav1.ListOptions) (watchapi.Interface, error) {
			options.FieldSelector = fs.String()
			return f.KubeClient.CoreV1().Events(pod.Namespace).Watch(options)
		},
	}
	// Watch for updates to the Event resource until fn is satisfied, or until the timeout is reached.
	ctx, cfn := context.WithTimeout(context.Background(), defaultWatchTimeout)
	defer cfn()
	last, err := watch.UntilWithSync(ctx, lw, &corev1.Event{}, nil, func(event watchapi.Event) (b bool, e error) {
		switch event.Type {
		case watchapi.Error:
			fallthrough
		case watchapi.Deleted:
			return false, fmt.Errorf("got event of unexpected type %q", event.Type)
		default:
			return event.Object.(*corev1.Event).Reason == reason, nil
		}
	})
	if err != nil {
		return err
	}
	if last == nil {
		return fmt.Errorf("no events involving pod \"%s/%s\" have been seen", pod.Namespace, pod.Name)
	}
	return nil
}

// GetRunningPods gets the running pods from the provider of the virtual kubelet
func (f *Framework) GetRunningPods() (*corev1.PodList, error) {
	result := &corev1.PodList{}

	err := f.KubeClient.CoreV1().
		RESTClient().
		Get().
		Resource("nodes").
		Name(f.NodeName).
		SubResource("proxy").
		Suffix("runningpods/").
		Do().
		Into(result)

	return result, err
}
