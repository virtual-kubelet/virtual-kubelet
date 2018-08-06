package sfmesh

import (
	"errors"
	"os"
	"testing"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func setEnvVars() {
	os.Setenv("AZURE_SUBSCRIPTION_ID", "fake")
	os.Setenv("AZURE_TENANT_ID", "fake")
	os.Setenv("AZURE_CLIENT_ID", "fake")
	os.Setenv("AZURE_CLIENT_SECRET", "fake")
	os.Setenv("REGION", "fake")
	os.Setenv("RESOURCE_GROUP", "fake")
}

func Test_podToMeshApp(t *testing.T) {
	setEnvVars()

	pod := &v1.Pod{}
	pod.ObjectMeta = metav1.ObjectMeta{
		Name: "test-pod",
	}
	pod.Spec = v1.PodSpec{
		Containers: []v1.Container{
			{
				Name:  "testcontainer",
				Image: "nginx",
				Ports: []v1.ContainerPort{
					{
						Name:          "http",
						ContainerPort: 80,
					},
				},
			},
		},
	}

	provider, err := NewSFMeshProvider(nil, "testnode", "Linux", "6.6.6.6", 80)
	if err != nil {
		t.Error(err.Error())
	}

	_, err = provider.getMeshApplication(pod)
	if err != nil {
		t.Error(err.Error())
	}
}

func Test_meshStateToPodCondition(t *testing.T) {
	setEnvVars()

	meshStateSucceeded := "Succeeded"

	provider, err := NewSFMeshProvider(nil, "testnode", "Linux", "6.6.6.6", 80)
	if err != nil {
		t.Error(err.Error())
	}

	phase := provider.appStateToPodPhase(meshStateSucceeded)
	if phase != v1.PodRunning {
		t.Error(errors.New("PodRunning phase expected"))
	}
}
