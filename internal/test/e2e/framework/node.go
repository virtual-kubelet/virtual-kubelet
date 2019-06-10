package framework

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	watchapi "k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/watch"
)

// WaitUntilNodeCondition establishes a watch on the vk node.
// Then, it waits for the specified condition function to be verified.
func (f *Framework) WaitUntilNodeCondition(fn watch.ConditionFunc) error {
	// Create a field selector that matches the specified Pod resource.
	fs := fields.OneTermEqualSelector("metadata.name", f.NodeName).String()
	// Create a ListWatch so we can receive events for the matched Pod resource.
	lw := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			options.FieldSelector = fs
			return f.KubeClient.CoreV1().Nodes().List(options)
		},
		WatchFunc: func(options metav1.ListOptions) (watchapi.Interface, error) {
			options.FieldSelector = fs
			return f.KubeClient.CoreV1().Nodes().Watch(options)
		},
	}

	// Watch for updates to the Pod resource until fn is satisfied, or until the timeout is reached.
	ctx, cancel := context.WithTimeout(context.Background(), defaultWatchTimeout)
	defer cancel()
	last, err := watch.UntilWithSync(ctx, lw, &corev1.Node{}, nil, fn)
	if err != nil {
		return err
	}
	if last == nil {
		return fmt.Errorf("no events received for node %q", f.NodeName)
	}
	return nil
}

// DeleteNode deletes the vk node used by the framework
func (f *Framework) DeleteNode() error {
	var gracePeriod int64
	propagation := metav1.DeletePropagationBackground
	opts := metav1.DeleteOptions{
		PropagationPolicy:  &propagation,
		GracePeriodSeconds: &gracePeriod,
	}
	return f.KubeClient.CoreV1().Nodes().Delete(f.NodeName, &opts)
}

// GetNode gets the vk nodeused by the framework
func (f *Framework) GetNode() (*corev1.Node, error) {
	return f.KubeClient.CoreV1().Nodes().Get(f.NodeName, metav1.GetOptions{})
}
