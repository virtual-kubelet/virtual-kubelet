package manager

import (
	"testing"

	"github.com/google/uuid"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

var (
	fakeClient kubernetes.Interface
)

func init() {
	fakeClient = fake.NewSimpleClientset()
}

func TestResourceManager(t *testing.T) {
	pm := NewResourceManager(fakeClient)
	pod1Name := "Pod1"
	pod1Namespace := "Pod1Namespace"
	pod1 := makePod(pod1Namespace, pod1Name)
	pm.AddPod(pod1)

	pods := pm.GetPods()
	if len(pods) != 1 {
		t.Errorf("Got %d, expected 1 pod", len(pods))
	}
	gotPod1 := pm.GetPod(pod1Namespace, pod1Name)
	if gotPod1.Name != pod1.Name {
		t.Errorf("Got %s, wanted %s", gotPod1.Name, pod1.Name)
	}
}

func TestResourceManagerDeletePod(t *testing.T) {
	pm := NewResourceManager(fakeClient)
	pod1Name := "Pod1"
	pod1Namespace := "Pod1Namespace"
	pod1 := makePod(pod1Namespace, pod1Name)
	pm.AddPod(pod1)
	pods := pm.GetPods()
	if len(pods) != 1 {
		t.Errorf("Got %d, expected 1 pod", len(pods))
	}
	pm.DeletePod(pod1)

	pods = pm.GetPods()
	if len(pods) != 0 {
		t.Errorf("Got %d, expected 0 pods", len(pods))
	}
}
func makePod(namespace, name string) *v1.Pod {
	pod := &v1.Pod{}
	pod.Name = name
	pod.Namespace = namespace
	pod.UID = types.UID(uuid.New().String())
	return pod
}

func TestResourceManagerUpdatePod(t *testing.T) {
	pm := NewResourceManager(fakeClient)
	pod1Name := "Pod1"
	pod1Namespace := "Pod1Namespace"
	pod1 := makePod(pod1Namespace, pod1Name)
	pm.AddPod(pod1)

	pods := pm.GetPods()
	if len(pods) != 1 {
		t.Errorf("Got %d, expected 1 pod", len(pods))
	}
	gotPod1 := pm.GetPod(pod1Namespace, pod1Name)
	if gotPod1.Name != pod1.Name {
		t.Errorf("Got %s, wanted %s", gotPod1.Name, pod1.Name)
	}

	if gotPod1.Namespace != pod1.Namespace {
		t.Errorf("Got %s, wanted %s", gotPod1.Namespace, pod1.Namespace)
	}
	pod1.Namespace = "POD2NAMESPACE"
	pm.UpdatePod(pod1)

	gotPod1 = pm.GetPod(pod1Namespace, pod1Name)
	if gotPod1.Name != pod1.Name {
		t.Errorf("Got %s, wanted %s", gotPod1.Name, pod1.Name)
	}

	if gotPod1.Namespace != pod1.Namespace {
		t.Errorf("Got %s, wanted %s", gotPod1.Namespace, pod1.Namespace)
	}
}
