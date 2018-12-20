/**
* Copyright (c) Microsoft.  All rights reserved.
 */

package azure

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/virtual-kubelet/virtual-kubelet/manager"
	"github.com/virtual-kubelet/virtual-kubelet/providers/azure/client"
	"github.com/virtual-kubelet/virtual-kubelet/providers/azure/client/aci"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	fakeSubscription  = "a88d9e8f-3cb3-456f-8f10-27395c1e122a"
	fakeResourceGroup = "vk-rg"
	fakeClientID      = "f14193ad-4c4c-4876-a18a-c0badb3bbd40"
	fakeClientSecret  = "VGhpcyBpcyBhIHNlY3JldAo="
	fakeTenantID      = "8cb81aca-83fe-4c6f-b667-4ec09c45a8bf"
	fakeNodeName      = "vk"
)

// Test make registry credential
func TestMakeRegistryCredential(t *testing.T) {
	server := "server-" + uuid.New().String()
	username := "user-" + uuid.New().String()
	password := "pass-" + uuid.New().String()
	auth := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", username, password)))

	tt := []struct {
		name        string
		authConfig  AuthConfig
		shouldFail  bool
		failMessage string
	}{
		{
			"Valid username and password",
			AuthConfig{Username: username, Password: password},
			false,
			"",
		},
		{
			"Username and password in auth",
			AuthConfig{Auth: auth},
			false,
			"",
		},
		{
			"No Username",
			AuthConfig{},
			true,
			"no username present in auth config for server",
		},
		{
			"Invalid Auth",
			AuthConfig{Auth: "123"},
			true,
			"error decoding the auth for server",
		},
		{
			"Malformed Auth",
			AuthConfig{Auth: base64.StdEncoding.EncodeToString([]byte("123"))},
			true,
			"malformed auth for server",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			cred, err := makeRegistryCredential(server, tc.authConfig)

			if tc.shouldFail {
				assert.NotNil(t, err, "convertion should fail")
				assert.True(t, strings.Contains(err.Error(), tc.failMessage), "failed message is not expected")
				return
			}

			assert.Nil(t, err, "convertion should not fail")
			assert.NotNil(t, cred, "credential should not be nil")
			assert.Equal(t, server, cred.Server, "server doesn't match")
			assert.Equal(t, username, cred.Username, "username doesn't match")
			assert.Equal(t, password, cred.Password, "password doesn't match")
		})
	}
}

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
		assert.Equal(t, podNamespace+"-"+podName, containerGroup, "Container group name is not expected")
		assert.NotNil(t, cg.ContainerGroupProperties, "Container group properties should not be nil")
		assert.NotNil(t, cg.ContainerGroupProperties.Containers, "Containers should not be nil")
		assert.Equal(t, 1, len(cg.ContainerGroupProperties.Containers), "1 Container is expected")
		assert.Equal(t, "nginx", cg.ContainerGroupProperties.Containers[0].Name, "Container nginx is expected")
		assert.NotNil(t, cg.ContainerGroupProperties.Containers[0].Resources, "Container resources should not be nil")
		assert.NotNil(t, cg.ContainerGroupProperties.Containers[0].Resources.Requests, "Container resource requests should not be nil")
		assert.Equal(t, 1.0, cg.ContainerGroupProperties.Containers[0].Resources.Requests.CPU, "Request CPU is not expected")
		assert.Equal(t, 1.5, cg.ContainerGroupProperties.Containers[0].Resources.Requests.MemoryInGB, "Request Memory is not expected")
		assert.Nil(t, cg.ContainerGroupProperties.Containers[0].Resources.Limits, "Limits should be nil")

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

	if err := provider.CreatePod(context.Background(), pod); err != nil {
		t.Fatal("Failed to create pod", err)
	}
}

// Tests create pod with resource request only
func TestCreatePodWithResourceRequestOnly(t *testing.T) {
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
		assert.Equal(t, podNamespace+"-"+podName, containerGroup, "Container group name is not expected")
		assert.NotNil(t, cg.ContainerGroupProperties, "Container group properties should not be nil")
		assert.NotNil(t, cg.ContainerGroupProperties.Containers, "Containers should not be nil")
		assert.Equal(t, 1, len(cg.ContainerGroupProperties.Containers), "1 Container is expected")
		assert.Equal(t, "nginx", cg.ContainerGroupProperties.Containers[0].Name, "Container nginx is expected")
		assert.NotNil(t, cg.ContainerGroupProperties.Containers[0].Resources, "Container resources should not be nil")
		assert.NotNil(t, cg.ContainerGroupProperties.Containers[0].Resources.Requests, "Container resource requests should not be nil")
		assert.Equal(t, 1.98, cg.ContainerGroupProperties.Containers[0].Resources.Requests.CPU, "Request CPU is not expected")
		assert.Equal(t, 3.4, cg.ContainerGroupProperties.Containers[0].Resources.Requests.MemoryInGB, "Request Memory is not expected")
		assert.Nil(t, cg.ContainerGroupProperties.Containers[0].Resources.Limits, "Limits should be nil")

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
							"cpu":    resource.MustParse("1.981"),
							"memory": resource.MustParse("3.49G"),
						},
					},
				},
			},
		},
	}

	if err := provider.CreatePod(context.Background(), pod); err != nil {
		t.Fatal("Failed to create pod", err)
	}
}

// Tests create pod with both resource request and limit.
func TestCreatePodWithResourceRequestAndLimit(t *testing.T) {
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
		assert.Equal(t, podNamespace+"-"+podName, containerGroup, "Container group name is not expected")
		assert.NotNil(t, cg.ContainerGroupProperties, "Container group properties should not be nil")
		assert.NotNil(t, cg.ContainerGroupProperties.Containers, "Containers should not be nil")
		assert.Equal(t, 1, len(cg.ContainerGroupProperties.Containers), "1 Container is expected")
		assert.Equal(t, "nginx", cg.ContainerGroupProperties.Containers[0].Name, "Container nginx is expected")
		assert.NotNil(t, cg.ContainerGroupProperties.Containers[0].Resources, "Container resources should not be nil")
		assert.NotNil(t, cg.ContainerGroupProperties.Containers[0].Resources.Requests, "Container resource requests should not be nil")
		assert.Equal(t, 1.98, cg.ContainerGroupProperties.Containers[0].Resources.Requests.CPU, "Request CPU is not expected")
		assert.Equal(t, 3.4, cg.ContainerGroupProperties.Containers[0].Resources.Requests.MemoryInGB, "Request Memory is not expected")
		assert.Equal(t, 3.999, cg.ContainerGroupProperties.Containers[0].Resources.Limits.CPU, "Limit CPU is not expected")
		assert.Equal(t, 8.01, cg.ContainerGroupProperties.Containers[0].Resources.Limits.MemoryInGB, "Limit Memory is not expected")

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
							"cpu":    resource.MustParse("1.981"),
							"memory": resource.MustParse("3.49G"),
						},
						Limits: v1.ResourceList{
							"cpu":    resource.MustParse("3999m"),
							"memory": resource.MustParse("8010M"),
						},
					},
				},
			},
		},
	}

	if err := provider.CreatePod(context.Background(), pod); err != nil {
		t.Fatal("Failed to create pod", err)
	}
}

// Tests get pods with empty list.
func TestGetPodsWithEmptyList(t *testing.T) {
	_, aciServerMocker, provider, err := prepareMocks()

	if err != nil {
		t.Fatal("Unable to prepare the mocks", err)
	}

	aciServerMocker.OnGetContainerGroups = func(subscription, resourceGroup string) (int, interface{}) {
		assert.Equal(t, fakeSubscription, subscription, "Subscription doesn't match")
		assert.Equal(t, fakeResourceGroup, resourceGroup, "Resource group doesn't match")

		return http.StatusOK, aci.ContainerGroupListResult{
			Value: []aci.ContainerGroup{},
		}
	}

	pods, err := provider.GetPods(context.Background())
	if err != nil {
		t.Fatal("Failed to get pods", err)
	}

	assert.NotNil(t, pods, "Response pods should not be nil")
	assert.Equal(t, 0, len(pods), "No pod should be returned")
}

// Tests get pods without requests limit.
func TestGetPodsWithoutResourceRequestsLimits(t *testing.T) {
	_, aciServerMocker, provider, err := prepareMocks()

	if err != nil {
		t.Fatal("Unable to prepare the mocks", err)
	}

	aciServerMocker.OnGetContainerGroups = func(subscription, resourceGroup string) (int, interface{}) {
		assert.Equal(t, fakeSubscription, subscription, "Subscription doesn't match")
		assert.Equal(t, fakeResourceGroup, resourceGroup, "Resource group doesn't match")

		var cg = aci.ContainerGroup{
			Name: "default-nginx",
			Tags: map[string]string{
				"NodeName": fakeNodeName,
			},
			ContainerGroupProperties: aci.ContainerGroupProperties{
				ProvisioningState: "Creating",
				Containers: []aci.Container{
					aci.Container{
						Name: "nginx",
						ContainerProperties: aci.ContainerProperties{
							Image:   "nginx",
							Command: []string{"nginx", "-g", "daemon off;"},
							Ports: []aci.ContainerPort{
								{
									Protocol: aci.ContainerNetworkProtocolTCP,
									Port:     80,
								},
							},
							Resources: aci.ResourceRequirements{
								Requests: &aci.ResourceRequests{
									CPU:        0.99,
									MemoryInGB: 1.5,
								},
							},
						},
					},
				},
			},
		}

		return http.StatusOK, aci.ContainerGroupListResult{
			Value: []aci.ContainerGroup{cg},
		}
	}

	pods, err := provider.GetPods(context.Background())
	if err != nil {
		t.Fatal("Failed to get pods", err)
	}

	assert.NotNil(t, pods, "Response pods should not be nil")
	assert.Equal(t, 1, len(pods), "No pod should be returned")

	pod := pods[0]

	assert.NotNil(t, pod, "Response pod should not be nil")
	assert.NotNil(t, pod.Spec.Containers, "Containers should not be nil")
	assert.NotNil(t, pod.Spec.Containers[0].Resources, "Containers[0].Resources should not be nil")
	assert.Nil(t, pod.Spec.Containers[0].Resources.Limits, "Containers[0].Resources.Limits should be nil")
	assert.NotNil(t, pod.Spec.Containers[0].Resources.Requests, "Containers[0].Resources.Requests should be nil")
	assert.Equal(
		t,
		ptrQuantity(resource.MustParse("0.99")).Value(),
		pod.Spec.Containers[0].Resources.Requests.Cpu().Value(),
		"Containers[0].Resources.Requests.CPU doesn't match")
	assert.Equal(
		t,
		ptrQuantity(resource.MustParse("1.5G")).Value(),
		pod.Spec.Containers[0].Resources.Requests.Memory().Value(),
		"Containers[0].Resources.Requests.Memory doesn't match")
}

// Tests get pod without requests limit.
func TestGetPodWithoutResourceRequestsLimits(t *testing.T) {
	_, aciServerMocker, provider, err := prepareMocks()

	if err != nil {
		t.Fatal("Unable to prepare the mocks", err)
	}

	podName := "pod-" + uuid.New().String()
	podNamespace := "ns-" + uuid.New().String()

	aciServerMocker.OnGetContainerGroup = func(subscription, resourceGroup, containerGroup string) (int, interface{}) {
		assert.Equal(t, fakeSubscription, subscription, "Subscription doesn't match")
		assert.Equal(t, fakeResourceGroup, resourceGroup, "Resource group doesn't match")
		assert.Equal(t, podNamespace+"-"+podName, containerGroup, "Container group name is not expected")

		return http.StatusOK, aci.ContainerGroup{
			Tags: map[string]string{
				"NodeName": fakeNodeName,
			},
			ContainerGroupProperties: aci.ContainerGroupProperties{
				ProvisioningState: "Creating",
				Containers: []aci.Container{
					aci.Container{
						Name: "nginx",
						ContainerProperties: aci.ContainerProperties{
							Image:   "nginx",
							Command: []string{"nginx", "-g", "daemon off;"},
							Ports: []aci.ContainerPort{
								{
									Protocol: aci.ContainerNetworkProtocolTCP,
									Port:     80,
								},
							},
							Resources: aci.ResourceRequirements{
								Requests: &aci.ResourceRequests{
									CPU:        0.99,
									MemoryInGB: 1.5,
								},
							},
						},
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
	assert.NotNil(t, pod.Spec.Containers[0].Resources, "Containers[0].Resources should not be nil")
	assert.Nil(t, pod.Spec.Containers[0].Resources.Limits, "Containers[0].Resources.Limits should be nil")
	assert.NotNil(t, pod.Spec.Containers[0].Resources.Requests, "Containers[0].Resources.Requests should be nil")
	assert.Equal(
		t,
		ptrQuantity(resource.MustParse("0.99")).Value(),
		pod.Spec.Containers[0].Resources.Requests.Cpu().Value(),
		"Containers[0].Resources.Requests.CPU doesn't match")
	assert.Equal(
		t,
		ptrQuantity(resource.MustParse("1.5G")).Value(),
		pod.Spec.Containers[0].Resources.Requests.Memory().Value(),
		"Containers[0].Resources.Requests.Memory doesn't match")
}

func TestGetPodWithContainerID(t *testing.T) {
	_, aciServerMocker, provider, err := prepareMocks()

	if err != nil {
		t.Fatal("Unable to prepare the mocks", err)
	}

	podName := "pod-" + uuid.New().String()
	podNamespace := "ns-" + uuid.New().String()
	containerName := "c-" + uuid.New().String()
	containerImage := "ci-" + uuid.New().String()

	cgID := fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.ContainerInstance/containerGroups/%s-%s", fakeSubscription, fakeResourceGroup, podNamespace, podName)

	aciServerMocker.OnGetContainerGroup = func(subscription, resourceGroup, containerGroup string) (int, interface{}) {
		assert.Equal(t, fakeSubscription, subscription, "Subscription doesn't match")
		assert.Equal(t, fakeResourceGroup, resourceGroup, "Resource group doesn't match")
		assert.Equal(t, podNamespace+"-"+podName, containerGroup, "Container group name is not expected")

		return http.StatusOK, aci.ContainerGroup{
			ID: cgID,
			Tags: map[string]string{
				"NodeName": fakeNodeName,
			},
			ContainerGroupProperties: aci.ContainerGroupProperties{
				ProvisioningState: "Creating",
				Containers: []aci.Container{
					aci.Container{
						Name: containerName,
						ContainerProperties: aci.ContainerProperties{
							Image:   containerImage,
							Command: []string{"nginx", "-g", "daemon off;"},
							Ports: []aci.ContainerPort{
								{
									Protocol: aci.ContainerNetworkProtocolTCP,
									Port:     80,
								},
							},
							Resources: aci.ResourceRequirements{
								Requests: &aci.ResourceRequests{
									CPU:        0.99,
									MemoryInGB: 1.5,
								},
							},
						},
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
	assert.Equal(t, 1, len(pod.Status.ContainerStatuses), "1 container status is expected")
	assert.Equal(t, containerName, pod.Status.ContainerStatuses[0].Name, "Container name in the container status doesn't match")
	assert.Equal(t, containerImage, pod.Status.ContainerStatuses[0].Image, "Container image in the container status doesn't match")
	assert.Equal(t, getContainerID(cgID, containerName), pod.Status.ContainerStatuses[0].ContainerID, "Container ID in the container status is not expected")
}

func TestPodToACISecretEnvVar(t *testing.T) {

	testKey := "testVar"
	testVal := "testVal"

	e := v1.EnvVar{
		Name:  testKey,
		Value: testVal,
		ValueFrom: &v1.EnvVarSource{
			SecretKeyRef: &v1.SecretKeySelector{},
		},
	}
	aciEnvVar := getACIEnvVar(e)

	if aciEnvVar.Value != "" {
		t.Fatalf("ACI Env Variable Value should be empty for a secret")
	}

	if aciEnvVar.Name != testKey {
		t.Fatalf("ACI Env Variable Name does not match expected Name")
	}

	if aciEnvVar.SecureValue != testVal {
		t.Fatalf("ACI Env Variable Secure Value does not match expected value")
	}
}

func TestPodToACIEnvVar(t *testing.T) {

	testKey := "testVar"
	testVal := "testVal"

	e := v1.EnvVar{
		Name:      testKey,
		Value:     testVal,
		ValueFrom: &v1.EnvVarSource{},
	}
	aciEnvVar := getACIEnvVar(e)

	if aciEnvVar.SecureValue != "" {
		t.Fatalf("ACI Env Variable Secure Value should be empty for non-secret variables")
	}

	if aciEnvVar.Name != testKey {
		t.Fatalf("ACI Env Variable Name does not match expected Name")
	}

	if aciEnvVar.Value != testVal {
		t.Fatalf("ACI Env Variable Value does not match expected value")
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

	rm, err := manager.NewResourceManager(nil, nil, nil)
	if err != nil {
		return nil, nil, nil, err
	}

	provider, err := NewACIProvider("example.toml", rm, fakeNodeName, "Linux", "0.0.0.0", 10250)
	if err != nil {
		return nil, nil, nil, err
	}

	return aadServerMocker, aciServerMocker, provider, nil
}

func ptrQuantity(q resource.Quantity) *resource.Quantity {
	return &q
}

func TestCreatePodWithLivenessProbe(t *testing.T) {
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
		assert.Equal(t, podNamespace+"-"+podName, containerGroup, "Container group name is not expected")
		assert.NotNil(t, cg.ContainerGroupProperties, "Container group properties should not be nil")
		assert.NotNil(t, cg.ContainerGroupProperties.Containers, "Containers should not be nil")
		assert.Equal(t, 1, len(cg.ContainerGroupProperties.Containers), "1 Container is expected")
		assert.Equal(t, "nginx", cg.ContainerGroupProperties.Containers[0].Name, "Container nginx is expected")
		assert.NotNil(t, cg.Containers[0].LivenessProbe, "Liveness probe expected")
		assert.Equal(t, int32(10), cg.Containers[0].LivenessProbe.InitialDelaySeconds, "Initial Probe Delay doesn't match")
		assert.Equal(t, int32(5), cg.Containers[0].LivenessProbe.Period, "Probe Period doesn't match")
		assert.Equal(t, int32(60), cg.Containers[0].LivenessProbe.TimeoutSeconds, "Probe Timeout doesn't match")
		assert.Equal(t, int32(3), cg.Containers[0].LivenessProbe.SuccessThreshold, "Probe Success Threshold doesn't match")
		assert.Equal(t, int32(5), cg.Containers[0].LivenessProbe.FailureThreshold, "Probe Failure Threshold doesn't match")
		assert.NotNil(t, cg.Containers[0].LivenessProbe.HTTPGet, "Expected an HTTP Get Probe")

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
					LivenessProbe: &v1.Probe{
						Handler: v1.Handler{
							HTTPGet: &v1.HTTPGetAction{
								Port: intstr.FromString("8080"),
								Path: "/",
							},
						},
						InitialDelaySeconds: 10,
						PeriodSeconds:       5,
						TimeoutSeconds:      60,
						SuccessThreshold:    3,
						FailureThreshold:    5,
					},
				},
			},
		},
	}

	if err := provider.CreatePod(context.Background(), pod); err != nil {
		t.Fatal("Failed to create pod", err)
	}
}

func TestCreatePodWithReadinessProbe(t *testing.T) {
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
		assert.Equal(t, podNamespace+"-"+podName, containerGroup, "Container group name is not expected")
		assert.NotNil(t, cg.ContainerGroupProperties, "Container group properties should not be nil")
		assert.NotNil(t, cg.ContainerGroupProperties.Containers, "Containers should not be nil")
		assert.Equal(t, 1, len(cg.ContainerGroupProperties.Containers), "1 Container is expected")
		assert.Equal(t, "nginx", cg.ContainerGroupProperties.Containers[0].Name, "Container nginx is expected")
		assert.NotNil(t, cg.Containers[0].ReadinessProbe, "Readiness probe expected")
		assert.Equal(t, int32(10), cg.Containers[0].ReadinessProbe.InitialDelaySeconds, "Initial Probe Delay doesn't match")
		assert.Equal(t, int32(5), cg.Containers[0].ReadinessProbe.Period, "Probe Period doesn't match")
		assert.Equal(t, int32(60), cg.Containers[0].ReadinessProbe.TimeoutSeconds, "Probe Timeout doesn't match")
		assert.Equal(t, int32(3), cg.Containers[0].ReadinessProbe.SuccessThreshold, "Probe Success Threshold doesn't match")
		assert.Equal(t, int32(5), cg.Containers[0].ReadinessProbe.FailureThreshold, "Probe Failure Threshold doesn't match")
		assert.NotNil(t, cg.Containers[0].ReadinessProbe.HTTPGet, "Expected an HTTP Get Probe")

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
					ReadinessProbe: &v1.Probe{
						Handler: v1.Handler{
							HTTPGet: &v1.HTTPGetAction{
								Port: intstr.FromString("8080"),
								Path: "/",
							},
						},
						InitialDelaySeconds: 10,
						PeriodSeconds:       5,
						TimeoutSeconds:      60,
						SuccessThreshold:    3,
						FailureThreshold:    5,
					},
				},
			},
		},
	}

	if err := provider.CreatePod(context.Background(), pod); err != nil {
		t.Fatal("Failed to create pod", err)
	}
}
