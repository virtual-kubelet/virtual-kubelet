package huawei

import (
	"context"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
)

const (
	fakeAppKey    = "Whj8f5RAHsvQahveqCdo"
	fakeAppSecret = "ymW5JgrdwrIvRS76YxyIqHNXe9s5ocIhaWWvPUhx"
	fakeRegion    = "southchina"
	fakeService   = "default"
	fakeProject   = "vk-project"
	fakeNodeName  = "vk"
)

// TestCreateProject test create project.
func TestCreateProject(t *testing.T) {
	cciServerMocker, provider, err := prepareMocks()

	if err != nil {
		t.Fatal("Unable to prepare the mocks", err)
	}

	cciServerMocker.OnCreateProject = func(ns *v1.Namespace) (int, interface{}) {
		assert.NotNil(t, ns, "Project is nil")
		assert.Equal(t, fakeProject, ns.Name, "pod.Annotations[\"virtual-kubelet-podname\"] is not expected")
		return http.StatusOK, &v1.Namespace{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Namespace",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: fakeProject,
			},
		}
	}

	if err := provider.createProject(); err != nil {
		t.Fatal("Failed to create project", err)
	}
}

// TestCreatePod test create pod.
func TestCreatePod(t *testing.T) {
	cciServerMocker, provider, err := prepareMocks()

	if err != nil {
		t.Fatal("Unable to prepare the mocks", err)
	}
	podName := "pod-" + string(uuid.NewUUID())
	podNamespace := "ns-" + string(uuid.NewUUID())

	cciServerMocker.OnCreatePod = func(pod *v1.Pod) (int, interface{}) {
		assert.NotNil(t, pod, "Pod is nil")
		assert.NotNil(t, pod.Annotations, "pod.Annotations is expected")
		assert.Equal(t, podName, pod.Annotations[podAnnotationPodNameKey], "pod.Annotations[\"virtual-kubelet-podname\"] is not expected")
		assert.Equal(t, podNamespace, pod.Annotations[podAnnotationNamespaceKey], "pod.Annotations[\"virtual-kubelet-namespace\"] is not expected")
		assert.Equal(t, 1, len(pod.Spec.Containers), "1 Container is expected")
		assert.Equal(t, "nginx", pod.Spec.Containers[0].Name, "Container nginx is expected")
		return http.StatusOK, pod
	}

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: podNamespace,
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				v1.Container{
					Name: "nginx",
				},
			},
		},
	}

	if err := provider.CreatePod(context.Background(), pod); err != nil {
		t.Fatal("Failed to create pod", err)
	}
}

// Tests get pod.
func TestGetPod(t *testing.T) {
	cciServerMocker, provider, err := prepareMocks()

	if err != nil {
		t.Fatal("Unable to prepare the mocks", err)
	}

	podName := "pod-" + string(uuid.NewUUID())
	podNamespace := "ns-" + string(uuid.NewUUID())

	cciServerMocker.OnGetPod = func(namespace, name string) (int, interface{}) {
		annotations := map[string]string{
			podAnnotationPodNameKey:     "podname",
			podAnnotationNamespaceKey:   "podnamespaces",
			podAnnotationUIDkey:         "poduid",
			podAnnotationClusterNameKey: "podclustername",
			podAnnotationNodeName:       "podnodename",
		}

		return http.StatusOK, &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:        podName,
				Namespace:   podNamespace,
				Annotations: annotations,
			},
			Spec: v1.PodSpec{
				Containers: []v1.Container{
					v1.Container{
						Name: "nginx",
					},
				},
			},
		}
	}
	pod, err := provider.GetPod(context.Background(), podNamespace, podName)
	if err != nil {
		t.Fatal("Failed to get pod", err)
	}

	assert.NotNil(t, pod, "Response pod should not be nil")
	assert.NotNil(t, pod.Spec.Containers, "Containers should not be nil")
	assert.Equal(t, pod.Name, "podname", "Pod name is not expected")
	assert.Equal(t, pod.Namespace, "podnamespaces", "Pod namespace is not expected")
	assert.Nil(t, pod.Annotations, "Pod Annotations should be nil")
	assert.Equal(t, string(pod.UID), "poduid", "Pod UID is not expected")
	assert.Equal(t, pod.ClusterName, "podclustername", "Pod clustername is not expected")
	assert.Equal(t, pod.Spec.NodeName, "podnodename", "Pod node name is not expected")
}

// Tests get pod.
func TestGetPods(t *testing.T) {
	cciServerMocker, provider, err := prepareMocks()

	if err != nil {
		t.Fatal("Unable to prepare the mocks", err)
	}

	podName := "pod-" + string(uuid.NewUUID())
	podNamespace := "ns-" + string(uuid.NewUUID())

	cciServerMocker.OnGetPods = func() (int, interface{}) {
		annotations := map[string]string{
			podAnnotationPodNameKey:     "podname",
			podAnnotationNamespaceKey:   "podnamespaces",
			podAnnotationUIDkey:         "poduid",
			podAnnotationClusterNameKey: "podclustername",
			podAnnotationNodeName:       "podnodename",
		}

		pod := v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:        podName,
				Namespace:   podNamespace,
				Annotations: annotations,
			},
			Spec: v1.PodSpec{
				Containers: []v1.Container{
					v1.Container{
						Name: "nginx",
					},
				},
			},
		}
		return http.StatusOK, []v1.Pod{pod}
	}
	pods, err := provider.GetPods(context.Background())
	if err != nil {
		t.Fatal("Failed to get pods", err)
	}

	pod := pods[0]
	assert.NotNil(t, pod, "Response pod should not be nil")
	assert.NotNil(t, pod.Spec.Containers, "Containers should not be nil")
	assert.Equal(t, pod.Name, "podname", "Pod name is not expected")
	assert.Equal(t, pod.Namespace, "podnamespaces", "Pod namespace is not expected")
	assert.Nil(t, pod.Annotations, "Pod Annotations should be nil")
	assert.Equal(t, string(pod.UID), "poduid", "Pod UID is not expected")
	assert.Equal(t, pod.ClusterName, "podclustername", "Pod clustername is not expected")
	assert.Equal(t, pod.Spec.NodeName, "podnodename", "Pod node name is not expected")
}

func prepareMocks() (*CCIMock, *CCIProvider, error) {
	cciServerMocker := NewCCIMock()

	os.Setenv("CCI_APP_KEP", fakeAppKey)
	os.Setenv("CCI_APP_SECRET", fakeAppSecret)

	defaultApiEndpoint = cciServerMocker.GetServerURL()
	provider, err := NewCCIProvider("cci.toml", nil, fakeNodeName, "Linux", "0.0.0.0", 10250)
	if err != nil {
		return nil, nil, err
	}

	provider.project = fakeProject
	provider.client.Signer = &fakeSigner{
		AppKey:    fakeAppKey,
		AppSecret: fakeAppSecret,
		Region:    fakeRegion,
		Service:   fakeService,
	}

	return cciServerMocker, provider, nil
}
