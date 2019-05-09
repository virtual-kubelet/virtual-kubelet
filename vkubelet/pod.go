package vkubelet

import (
	"context"
	"fmt"

	"github.com/virtual-kubelet/virtual-kubelet/log"
	"github.com/virtual-kubelet/virtual-kubelet/trace"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
)

func addPodAttributes(ctx context.Context, span trace.Span, pod *corev1.Pod) context.Context {
	return span.WithFields(ctx, log.Fields{
		"uid":       string(pod.GetUID()),
		"namespace": pod.GetNamespace(),
		"name":      pod.GetName(),
		"phase":     string(pod.Status.Phase),
		"reason":    pod.Status.Reason,
	})
}

// loggablePodName returns the "namespace/name" key for the specified pod.
// If the key cannot be computed, "(unknown)" is returned.
// This method is meant to be used for logging purposes only.
func loggablePodName(pod *corev1.Pod) string {
	k, err := cache.MetaNamespaceKeyFunc(pod)
	if err != nil {
		return "(unknown)"
	}
	return k
}

// loggablePodNameFromCoordinates returns the "namespace/name" key for the pod identified by the specified namespace and name (coordinates).
func loggablePodNameFromCoordinates(namespace, name string) string {
	return fmt.Sprintf("%s/%s", namespace, name)
}
