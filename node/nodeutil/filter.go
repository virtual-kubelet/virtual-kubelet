package nodeutil

import (
	"context"

	"github.com/virtual-kubelet/virtual-kubelet/node"
	v1 "k8s.io/api/core/v1"
)

// FilterPodsForNodeName creates an event filter function that filters pod events such that pod.Sepc.NodeName matches the provided name
// Use the return value of this as the PodEventFilterFunc in PodControllerConfig
func FilterPodsForNodeName(name string) node.PodEventFilterFunc {
	return func(_ context.Context, p *v1.Pod) bool {
		return p.Spec.NodeName == name
	}
}

// PodFilters turns a list of pod filters into a single filter.
// When run, each item in the list is itterated in order until the first `true` result.
// If nothing returns true, the filter is false.
func PodFilters(filters ...node.PodEventFilterFunc) node.PodEventFilterFunc {
	return func(ctx context.Context, p *v1.Pod) bool {
		for _, f := range filters {
			if f(ctx, p) {
				return true
			}
		}
		return false
	}
}
