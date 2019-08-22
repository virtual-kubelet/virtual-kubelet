package node

import v1 "k8s.io/api/core/v1"

// Is this pod deletable?
func deletable(pod *v1.Pod) bool {
	return pod.DeletionGracePeriodSeconds != nil
}
