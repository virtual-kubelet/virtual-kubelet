package framework

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	watchapi "k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/watch"
)

// CreateDummyPodObjectWithPrefix creates a dujmmy pod object using the specified prefix as the value of .metadata.generateName.
// A variable number of strings can be provided.
// For each one of these strings, a container that uses the string as its image will be appended to the pod.
// This method DOES NOT create the pod in the Kubernetes API.
func (f *Framework) CreateDummyPodObjectWithPrefix(testName string, prefix string, images ...string) *corev1.Pod {
	// Safe the test name
	if testName != "" {
		testName = stripParentTestName(strings.ToLower(testName))
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
func (f *Framework) CreatePod(ctx context.Context, pod *corev1.Pod) (*corev1.Pod, error) {
	return f.KubeClient.CoreV1().Pods(f.Namespace).Create(ctx, pod, metav1.CreateOptions{})
}

// DeletePod deletes the pod with the specified name and namespace in the Kubernetes API using the default grace period.
func (f *Framework) DeletePod(ctx context.Context, namespace, name string) error {
	return f.KubeClient.CoreV1().Pods(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

// DeletePodImmediately forcibly deletes the pod with the specified name and namespace in the Kubernetes API.
// This is equivalent to running "kubectl delete --force --grace-period 0 --namespace <namespace> pod <name>".
func (f *Framework) DeletePodImmediately(ctx context.Context, namespace, name string) error {
	grace := int64(0)
	propagation := metav1.DeletePropagationBackground
	return f.KubeClient.CoreV1().Pods(namespace).Delete(ctx, name, metav1.DeleteOptions{
		GracePeriodSeconds: &grace,
		PropagationPolicy:  &propagation,
	})
}

// WaitUntilPodCondition establishes a watch on the pod with the specified name and namespace.
// Then, it waits for the specified condition function to be verified.
func (f *Framework) WaitUntilPodCondition(namespace, name string, fn watch.ConditionFunc) (*corev1.Pod, error) {
	// Watch for updates to the Pod resource until fn is satisfied, or until the timeout is reached.
	ctx, cfn := context.WithTimeout(context.Background(), f.WatchTimeout)
	defer cfn()
	// Create a field selector that matches the specified Pod resource.
	fs := fields.ParseSelectorOrDie(fmt.Sprintf("metadata.namespace==%s,metadata.name==%s", namespace, name))
	// Create a ListWatch so we can receive events for the matched Pod resource.
	lw := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			options.FieldSelector = fs.String()
			return f.KubeClient.CoreV1().Pods(namespace).List(ctx, options)
		},
		WatchFunc: func(options metav1.ListOptions) (watchapi.Interface, error) {
			options.FieldSelector = fs.String()
			return f.KubeClient.CoreV1().Pods(namespace).Watch(ctx, options)
		},
	}
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
		return pod.Status.Phase == corev1.PodRunning && IsPodReady(pod) && pod.Status.PodIP != "", nil
	})
}

// IsPodReady returns true if a pod is ready.
func IsPodReady(pod *corev1.Pod) bool {
	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

// WaitUntilPodDeleted blocks until the pod with the specified name and namespace is deleted from apiserver.
func (f *Framework) WaitUntilPodDeleted(namespace, name string) (*corev1.Pod, error) {
	return f.WaitUntilPodCondition(namespace, name, func(event watchapi.Event) (bool, error) {
		pod := event.Object.(*corev1.Pod)
		return event.Type == watchapi.Deleted || pod.DeletionTimestamp != nil, nil
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
	// Watch for updates to the Event resource until fn is satisfied, or until the timeout is reached.
	ctx, cfn := context.WithTimeout(context.Background(), f.WatchTimeout)
	defer cfn()

	// Create a field selector that matches Event resources involving the specified pod.
	fs := fields.ParseSelectorOrDie(fmt.Sprintf("involvedObject.kind==Pod,involvedObject.uid==%s", pod.UID))
	// Create a ListWatch so we can receive events for the matched Event resource.
	lw := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			options.FieldSelector = fs.String()
			return f.KubeClient.CoreV1().Events(pod.Namespace).List(ctx, options)
		},
		WatchFunc: func(options metav1.ListOptions) (watchapi.Interface, error) {
			options.FieldSelector = fs.String()
			return f.KubeClient.CoreV1().Events(pod.Namespace).Watch(ctx, options)
		},
	}

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

// GetRunningPodsFromProvider gets the running pods from the provider of the virtual kubelet
func (f *Framework) GetRunningPodsFromProvider(ctx context.Context) (*corev1.PodList, error) {
	result := &corev1.PodList{}

	err := f.KubeClient.CoreV1().
		RESTClient().
		Get().
		Resource("nodes").
		Name(f.NodeName).
		SubResource("proxy").
		Suffix("runningpods/").
		Do(ctx).
		Into(result)

	return result, err
}

// GetRunningPodsFromKubernetes gets the running pods from the provider of the virtual kubelet
func (f *Framework) GetRunningPodsFromKubernetes(ctx context.Context) (*corev1.PodList, error) {
	result := &corev1.PodList{}

	err := f.KubeClient.CoreV1().
		RESTClient().
		Get().
		Resource("nodes").
		Name(f.NodeName).
		SubResource("proxy").
		Suffix("pods").
		Do(ctx).
		Into(result)

	return result, err
}

// stripParentTestName strips out the parent's test name from the input (in the form of 'TestParent/TestChild').
// Some test cases use their name as the pod name for testing purpose, and sometimes it might exceed 63
// characters (Kubernetes's limit for pod name). This function ensures that we strip out the parent's
// test name to decrease the length of the pod name
func stripParentTestName(name string) string {
	parts := strings.Split(name, "/")
	if len(parts) == 1 {
		return parts[0]
	}
	return parts[len(parts)-1]
}
