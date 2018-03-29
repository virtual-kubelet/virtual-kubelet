/**
* Copyright (c) Microsoft.  All rights reserved.
 */

package azure

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/virtual-kubelet/virtual-kubelet/manager"
	azure "github.com/virtual-kubelet/virtual-kubelet/providers/azure/client"
	"github.com/virtual-kubelet/virtual-kubelet/providers/azure/client/aci"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/kubernetes/fake"
)

const (
	fakeSubscription  = "a88d9e8f-3cb3-456f-8f10-27395c1e122a"
	fakeResourceGroup = "vk-rg"
	fakeClientID      = "f14193ad-4c4c-4876-a18a-c0badb3bbd40"
	fakeClientSecret  = "VGhpcyBpcyBhIHNlY3JldAo="
	fakeTenantID      = "8cb81aca-83fe-4c6f-b667-4ec09c45a8bf"
)

// Tests create pod without resource spec
func TestCreatePodWithoutResourceSpec(t *testing.T) {
	_, aciServerMocker, provider, err := prepareMocks()

	if err != nil {
		t.Fatal("Unable to prepare the mocks", err)
	}

	podName := "pod-" + uuid.New().String()
	podNamespace := "ns-" + uuid.New().String()

	aciServerMocker.OnCreate = func(subscription, resourceGroup, containerGroup string, cg *aci.ContainerGroup) (int, interface{}) {
		assert.Equal(t, fakeSubscription, subscription, "Subscription doesn't match")
		assert.Equal(t, fakeResourceGroup, resourceGroup, "Resource group doesn't match")
		assert.NotNil(t, cg, "Container group is nil")
		assert.Equal(t, podNamespace + "-" + podName, containerGroup, "Container group name is not expected")
		assert.NotNil(t, cg.ContainerGroupProperties, "Container group properties should not be nil")
		assert.NotNil(t, cg.ContainerGroupProperties.Containers, "Containers should not be nil")
		assert.Equal(t, 1, len(cg.ContainerGroupProperties.Containers), "1 Container is expected")
		assert.Equal(t, "nginx", cg.ContainerGroupProperties.Containers[0].Name, "Container nginx is expected")
		assert.NotNil(t, cg.ContainerGroupProperties.Containers[0].Resources, "Container resources should not be nil")
		assert.NotNil(t, cg.ContainerGroupProperties.Containers[0].Resources.Requests, "Container resource requests should not be nil")
		assert.Equal(t, 1.0, cg.ContainerGroupProperties.Containers[0].Resources.Requests.CPU, "Request CPU is not expected")
		assert.Equal(t, 1.5, cg.ContainerGroupProperties.Containers[0].Resources.Requests.MemoryInGB, "Request Memory is not expected")
		assert.Equal(t, 1.0, cg.ContainerGroupProperties.Containers[0].Resources.Limits.CPU, "Limit CPU is not expected")
		assert.Equal(t, 1.5, cg.ContainerGroupProperties.Containers[0].Resources.Limits.MemoryInGB, "Limit Memory is not expected")

		return http.StatusOK, cg
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

	if err := provider.CreatePod(pod); err != nil {
		t.Fatal("Failed to create pod", err)
	}
}

// Tests create pod without resource spec
func TestCreatePodWithResourceSpec(t *testing.T) {
	_, aciServerMocker, provider, err := prepareMocks()

	if err != nil {
		t.Fatal("Unable to prepare the mocks", err)
	}

	podName := "pod-" + uuid.New().String()
	podNamespace := "ns-" + uuid.New().String()

	aciServerMocker.OnCreate = func(subscription, resourceGroup, containerGroup string, cg *aci.ContainerGroup) (int, interface{}) {
		assert.Equal(t, fakeSubscription, subscription, "Subscription doesn't match")
		assert.Equal(t, fakeResourceGroup, resourceGroup, "Resource group doesn't match")
		assert.NotNil(t, cg, "Container group is nil")
		assert.Equal(t, podNamespace + "-" + podName, containerGroup, "Container group name is not expected")
		assert.NotNil(t, cg.ContainerGroupProperties, "Container group properties should not be nil")
		assert.NotNil(t, cg.ContainerGroupProperties.Containers, "Containers should not be nil")
		assert.Equal(t, 1, len(cg.ContainerGroupProperties.Containers), "1 Container is expected")
		assert.Equal(t, "nginx", cg.ContainerGroupProperties.Containers[0].Name, "Container nginx is expected")
		assert.NotNil(t, cg.ContainerGroupProperties.Containers[0].Resources, "Container resources should not be nil")
		assert.NotNil(t, cg.ContainerGroupProperties.Containers[0].Resources.Requests, "Container resource requests should not be nil")
		assert.Equal(t, 1.98, cg.ContainerGroupProperties.Containers[0].Resources.Requests.CPU, "Request CPU is not expected")
		assert.Equal(t, 3.4, cg.ContainerGroupProperties.Containers[0].Resources.Requests.MemoryInGB, "Request Memory is not expected")
		assert.Equal(t, 3.99, cg.ContainerGroupProperties.Containers[0].Resources.Limits.CPU, "Limit CPU is not expected")
		assert.Equal(t, 8.0, cg.ContainerGroupProperties.Containers[0].Resources.Limits.MemoryInGB, "Limit Memory is not expected")

		return http.StatusOK, cg
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
					Resources: v1.ResourceRequirements{
						Requests: v1.ResourceList{
							"cpu": resource.MustParse("1.98"),
							"memory": resource.MustParse("3.4G"),
						},
						Limits: v1.ResourceList{
							"cpu": resource.MustParse("3999m"),
							"memory": resource.MustParse("8010M"),
						},
					},
				},
			},
		},
	}

	if err := provider.CreatePod(pod); err != nil {
		t.Fatal("Failed to create pod", err)
	}
}

func prepareMocks() (*AADMock, *ACIMock, *ACIProvider, error) {
	aadServerMocker := NewAADMock()
	aciServerMocker := NewACIMock()

	auth := azure.NewAuthentication(
		azure.PublicCloud.Name,
		fakeClientID,
		fakeClientSecret,
		fakeSubscription,
		fakeTenantID)

	auth.ActiveDirectoryEndpoint = aadServerMocker.GetServerURL()
	auth.ResourceManagerEndpoint = aciServerMocker.GetServerURL()

	file, err := ioutil.TempFile("", "auth.json")
	if err != nil {
		return nil, nil, nil, err
	}

	defer os.Remove(file.Name())

	b := new(bytes.Buffer)
	json.NewEncoder(b).Encode(auth)

	if _, err := file.Write(b.Bytes()); err != nil {
		return nil, nil, nil, err
	}

	os.Setenv("AZURE_AUTH_LOCATION", file.Name())
	os.Setenv("ACI_RESOURCE_GROUP", fakeResourceGroup)

	clientset := fake.NewSimpleClientset()
	rm := manager.NewResourceManager(clientset)

	provider, err := NewACIProvider("example.toml", rm, "vk", "Linux", "0.0.0.0", 10250)
	if err != nil {
		return nil, nil, nil, err
	}

	return aadServerMocker, aciServerMocker, provider, nil
}

