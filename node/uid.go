package node

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

type podUIDKey struct {
	Namespace string
	Name      string
	UID       types.UID
}

func newPodUIDKey(pod *corev1.Pod) podUIDKey {
	return podUIDKey{
		Namespace: pod.Namespace,
		Name:      pod.Name,
		UID:       pod.UID,
	}
}

func objectName(namespace, name string) string {
	if len(namespace) > 0 {
		return namespace + "/" + name
	}
	return name
}

func (k *podUIDKey) ObjectName() string {
	return objectName(k.Namespace, k.Name)
}

func (k *podUIDKey) String() string {
	return k.ObjectName() + "/" + string(k.UID)
}

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
