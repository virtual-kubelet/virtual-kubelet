package node

import (
	"time"

	corev1 "k8s.io/api/core/v1"
)

var (
	_                     PodDeletionPolicy = (*defaultDeletionPolicy)(nil)
	DefaultDeletionPolicy                   = defaultDeletionPolicy{}
)

type PodDeletionPolicy interface {
	// ShouldDelete should return false if the pod if we should not delete the pod from API Server with a grace
	// period of 0
	ShouldDelete(*corev1.Pod) bool
	// ShouldDelayDelete indicates that we should delay deletion with a grace period of 0 by this amount. It will be
	// called again after the delay expires. A return value of 0, or a negative value indicates immediate deletion.
	ShouldDelayDelete(*corev1.Pod) time.Duration
}

type defaultDeletionPolicy struct {
}

func (d defaultDeletionPolicy) ShouldDelete(*corev1.Pod) bool {
	return true
}

func (d defaultDeletionPolicy) ShouldDelayDelete(*corev1.Pod) time.Duration {
	return time.Duration(0)
}
