package node

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

type uidProviderWrapper struct {
	PodLifecycleHandler
}

var _ PodUIDLifecycleHandler = (*uidProviderWrapper)(nil)

func (p *uidProviderWrapper) GetPodByUID(ctx context.Context, namespace, name string, uid types.UID) (*corev1.Pod, error) {
	return p.GetPod(ctx, namespace, name)
}

func (p *uidProviderWrapper) GetPodStatusByUID(ctx context.Context, namespace, name string, uid types.UID) (*corev1.PodStatus, error) {
	return p.GetPodStatus(ctx, namespace, name)
}
