package manager

import (
	"log"
	"testing"

	"github.com/google/uuid"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
)

var (
	fakeClient *kubernetes.Clientset
)

func init() {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	// if you want to change the loading rules (which files in which order), you can do so here

	configOverrides := &clientcmd.ConfigOverrides{}
	// if you want to change override values or bind them to flags, there are methods to help you

	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
	config, err := kubeConfig.ClientConfig()
	if err != nil {
		log.Fatalf("unable to create client config: %s\n", err.Error())
	}
	fakeClient, err = kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("unable to create new clientset: %s\n", err.Error())
	}
}

func TestResourceManager(t *testing.T) {
	pm := NewResourceManager(fakeClient)
	pod1Name := "Pod1"
	pod1 := makePod(pod1Name)
	pm.AddPod(pod1)

	pods := pm.GetPods()
	if len(pods) != 1 {
		t.Errorf("Got %d, expected 1 pod", len(pods))
	}
	gotPod1 := pm.GetPod(pod1Name)
	if gotPod1.Name != pod1.Name {
		t.Errorf("Got %s, wanted %s", gotPod1.Name, pod1.Name)
	}
}

func TestResourceManagerDeletePod(t *testing.T) {
	pm := NewResourceManager(fakeClient)
	pod1Name := "Pod1"
	pod1 := makePod(pod1Name)
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
func makePod(name string) *v1.Pod {
	pod := &v1.Pod{}
	pod.Name = name
	pod.UID = types.UID(uuid.New().String())
	return pod
}

func TestResourceManagerUpdatePod(t *testing.T) {
	pm := NewResourceManager(fakeClient)
	pod1Name := "Pod1"
	pod1 := makePod(pod1Name)
	pm.AddPod(pod1)

	pods := pm.GetPods()
	if len(pods) != 1 {
		t.Errorf("Got %d, expected 1 pod", len(pods))
	}
	gotPod1 := pm.GetPod(pod1Name)
	if gotPod1.Name != pod1.Name {
		t.Errorf("Got %s, wanted %s", gotPod1.Name, pod1.Name)
	}

	if gotPod1.Namespace != "" {
		t.Errorf("Got %s, wanted %s", gotPod1.Namespace, "<empty namespace>")
	}
	pod1.Namespace = "POD1NAMESPACE"
	pm.UpdatePod(pod1)

	gotPod1 = pm.GetPod(pod1Name)
	if gotPod1.Name != pod1.Name {
		t.Errorf("Got %s, wanted %s", gotPod1.Name, pod1.Name)
	}

	if gotPod1.Namespace != pod1.Namespace {
		t.Errorf("Got %s, wanted %s", gotPod1.Namespace, pod1.Namespace)
	}
}
