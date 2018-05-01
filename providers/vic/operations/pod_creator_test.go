// Copyright 2018 VMware, Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package operations

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	vicpod "github.com/virtual-kubelet/virtual-kubelet/providers/vic/pod"

	"github.com/vmware/vic/lib/apiservers/portlayer/client"
	"github.com/vmware/vic/lib/metadata"
	"github.com/vmware/vic/pkg/trace"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/virtual-kubelet/virtual-kubelet/providers/vic/cache"
	"github.com/virtual-kubelet/virtual-kubelet/providers/vic/proxy"
	proxymocks "github.com/virtual-kubelet/virtual-kubelet/providers/vic/proxy/mocks"
)

var (
	pod              v1.Pod
	imgConfig        metadata.ImageConfig
	busyboxIsoConfig proxy.IsolationContainerConfig
	alpineIsoConfig  proxy.IsolationContainerConfig
	vicPod           vicpod.VicPod
)

const (
	podID     = "123"
	podName   = "busybox-sleep"
	podHandle = "fakehandle"

	fakeEP = "fake-endpoint"
)

func createMocks(t *testing.T) (*proxymocks.ImageStore, *proxymocks.IsolationProxy, cache.PodCache, trace.Operation) {
	store := &proxymocks.ImageStore{}
	ip := &proxymocks.IsolationProxy{}
	cache := cache.NewVicPodCache()
	op := trace.NewOperation(context.Background(), "tests")

	return store, ip, cache, op
}

func TestNewPodCreator(t *testing.T) {
	var c PodCreator
	var err error

	store, proxy, cache, _ := createMocks(t)
	client := client.Default
	persona := "1.2.3.4"
	portlayer := "1.2.3.4"

	// Positive cases
	c, err = NewPodCreator(client, store, proxy, cache, persona, portlayer)
	assert.NotNil(t, c, "Expected not-nil creating a pod creator but received nil")

	// Negative cases
	c, err = NewPodCreator(nil, store, proxy, cache, persona, portlayer)
	assert.Nil(t, c, "Expected nil")
	assert.Equal(t, err, PodCreatorPortlayerClientError)

	c, err = NewPodCreator(client, nil, proxy, cache, persona, portlayer)
	assert.Nil(t, c, "Expected nil")
	assert.Equal(t, err, PodCreatorImageStoreError)

	c, err = NewPodCreator(client, store, nil, cache, persona, portlayer)
	assert.Nil(t, c, "Expected nil")
	assert.Equal(t, err, PodCreatorIsolationProxyError)

	c, err = NewPodCreator(client, store, proxy, nil, persona, portlayer)
	assert.Nil(t, c, "Expected nil")
	assert.Equal(t, err, PodCreatorPodCacheError)

	c, err = NewPodCreator(client, store, proxy, cache, "", portlayer)
	assert.Nil(t, c, "Expected nil")
	assert.Equal(t, err, PodCreatorPersonaAddrError)

	c, err = NewPodCreator(client, store, proxy, cache, persona, "")
	assert.Nil(t, c, "Expected nil")
	assert.Equal(t, err, PodCreatorPortlayerAddrError)
}

func TestCreatePod_NilPod(t *testing.T) {
	store, ip, cache, op := createMocks(t)
	client := client.Default
	persona := "1.2.3.4"
	portlayer := "1.2.3.4"

	// Create nil pod
	c, err := NewPodCreator(client, store, ip, cache, persona, portlayer)
	assert.NotNil(t, c, "Expected not-nil creating a pod creator but received nil")

	err = c.CreatePod(op, nil, true)
	assert.NotNil(t, err, "Expected error from createPod but received '%s'", err)
}

func TestCreatePod_Success(t *testing.T) {
	store, ip, cache, op := createMocks(t)
	client := client.Default
	persona := "1.2.3.4"
	portlayer := "1.2.3.4"

	// Setup mocks
	ip.On("CreateHandle", op).Return(podID, podHandle, nil)
	ip.On("AddImageToHandle", op, podHandle, "busybox-container", "", "", "").Return(podHandle, nil)
	ip.On("AddImageToHandle", op, podHandle, "alpine-container", "", "", "").Return(podHandle, nil)
	ip.On("CreateHandleTask", op, podHandle, podID, "", busyboxIsoConfig).Return(podHandle, nil)
	ip.On("CreateHandleTask", op, podHandle, "Container-1-task", "", alpineIsoConfig).Return(podHandle, nil)
	ip.On("AddHandleToScope", op, podHandle, busyboxIsoConfig).Return(podHandle, nil)
	ip.On("AddHandleToScope", op, podHandle, alpineIsoConfig).Return(podHandle, nil)
	ip.On("AddInteractionToHandle", op, podHandle).Return(podHandle, nil)
	ip.On("AddLoggingToHandle", op, podHandle).Return(podHandle, nil)
	ip.On("CommitHandle", op, podHandle, podID, int32(-1)).Return(nil)
	ip.On("Handle", op, podID, podName).Return(podHandle, nil)
	ip.On("BindScope", op, podHandle, podName).Return(podHandle, fakeEP, nil)
	ip.On("SetState", op, podHandle, podName, "RUNNING").Return(podHandle, nil)

	store.On("Get", op, "busybox", "", true).Return(&metadata.ImageConfig{}, nil)
	store.On("Get", op, "alpine", "", true).Return(&metadata.ImageConfig{}, nil)

	// The test
	c, err := NewPodCreator(client, store, ip, cache, persona, portlayer)
	assert.NotNil(t, c, "Expected not-nil creating a pod creator but received nil")

	err = c.CreatePod(op, &pod, true)
	assert.Nil(t, err, "Expected error from createPod but received '%s'", err)
}

func TestCreatePod_ImageStoreError(t *testing.T) {
	store, ip, cache, op := createMocks(t)
	client := client.Default
	persona := "1.2.3.4"
	portlayer := "1.2.3.4"

	// Setup mocks
	ip.On("CreateHandle", op).Return(podID, podHandle, nil)
	ip.On("AddImageToHandle", op, podHandle, "busybox-container", "", "", "").Return(podHandle, nil)
	ip.On("AddImageToHandle", op, podHandle, "alpine-container", "", "", "").Return(podHandle, nil)
	ip.On("CreateHandleTask", op, podHandle, podID, "", busyboxIsoConfig).Return(podHandle, nil)
	ip.On("CreateHandleTask", op, podHandle, "Container-1-task", "", alpineIsoConfig).Return(podHandle, nil)
	ip.On("AddHandleToScope", op, podHandle, busyboxIsoConfig).Return(podHandle, nil)
	ip.On("AddHandleToScope", op, podHandle, alpineIsoConfig).Return(podHandle, nil)
	ip.On("AddInteractionToHandle", op, podHandle).Return(podHandle, nil)
	ip.On("AddLoggingToHandle", op, podHandle).Return(podHandle, nil)
	ip.On("CommitHandle", op, podHandle, podID, int32(-1)).Return(nil)
	ip.On("Handle", op, podID, podName).Return(podHandle, nil)
	ip.On("BindScope", op, podHandle, podName).Return(podHandle, fakeEP, nil)
	ip.On("SetState", op, podHandle, podName, "RUNNING").Return(podHandle, nil)

	fakeErr := fmt.Errorf("Error getting pod containers")
	store.On("Get", op, "alpine", "", true).Return(nil, fakeErr)
	store.On("Get", op, "busybox", "", true).Return(nil, fakeErr)

	// The test
	c, err := NewPodCreator(client, store, ip, cache, persona, portlayer)
	assert.NotNil(t, c, "Expected not-nil creating a pod creator but received nil")

	err = c.CreatePod(op, &pod, true)
	assert.NotNil(t, err, "Expected nil error from createPod")
}

func TestCreatePod_CreateHandleError(t *testing.T) {
	store, ip, cache, op := createMocks(t)
	client := client.Default
	persona := "1.2.3.4"
	portlayer := "1.2.3.4"

	// Setup mocks
	fakeErr := fmt.Errorf("fake create handle error")
	ip.On("CreateHandle", op).Return(podID, podHandle, fakeErr)
	ip.On("AddImageToHandle", op, podHandle, "busybox-container", "", "", "").Return(podHandle, nil)
	ip.On("AddImageToHandle", op, podHandle, "alpine-container", "", "", "").Return(podHandle, nil)
	ip.On("CreateHandleTask", op, podHandle, podID, "", busyboxIsoConfig).Return(podHandle, nil)
	ip.On("CreateHandleTask", op, podHandle, "Container-1-task", "", alpineIsoConfig).Return(podHandle, nil)
	ip.On("AddHandleToScope", op, podHandle, busyboxIsoConfig).Return(podHandle, nil)
	ip.On("AddHandleToScope", op, podHandle, alpineIsoConfig).Return(podHandle, nil)
	ip.On("AddInteractionToHandle", op, podHandle).Return(podHandle, nil)
	ip.On("AddLoggingToHandle", op, podHandle).Return(podHandle, nil)
	ip.On("CommitHandle", op, podHandle, podID, int32(-1)).Return(nil)
	ip.On("Handle", op, podID, podName).Return(podHandle, nil)
	ip.On("BindScope", op, podHandle, podName).Return(podHandle, fakeEP, nil)
	ip.On("SetState", op, podHandle, podName, "RUNNING").Return(podHandle, nil)

	store.On("Get", op, "busybox", "", true).Return(&metadata.ImageConfig{}, nil)
	store.On("Get", op, "alpine", "", true).Return(&metadata.ImageConfig{}, nil)

	// The test
	c, err := NewPodCreator(client, store, ip, cache, persona, portlayer)
	assert.NotNil(t, c, "Expected not-nil creating a pod creator but received nil")

	err = c.CreatePod(op, &pod, true)
	assert.NotNil(t, err, "Expected nil error from createPod")
	assert.Equal(t, err.Error(), fakeErr.Error())
}

func TestCreatePod_AddImageError(t *testing.T) {
	store, ip, cache, op := createMocks(t)
	client := client.Default
	persona := "1.2.3.4"
	portlayer := "1.2.3.4"

	// Setup mocks
	fakeErr := fmt.Errorf("fake add image error")
	ip.On("CreateHandle", op).Return(podID, podHandle, nil)
	ip.On("AddImageToHandle", op, podHandle, "busybox-container", "", "", "").Return(podHandle, fakeErr)
	ip.On("AddImageToHandle", op, podHandle, "alpine-container", "", "", "").Return(podHandle, nil)
	ip.On("CreateHandleTask", op, podHandle, podID, "", busyboxIsoConfig).Return(podHandle, nil)
	ip.On("CreateHandleTask", op, podHandle, "Container-1-task", "", alpineIsoConfig).Return(podHandle, nil)
	ip.On("AddHandleToScope", op, podHandle, busyboxIsoConfig).Return(podHandle, nil)
	ip.On("AddHandleToScope", op, podHandle, alpineIsoConfig).Return(podHandle, nil)
	ip.On("AddInteractionToHandle", op, podHandle).Return(podHandle, nil)
	ip.On("AddLoggingToHandle", op, podHandle).Return(podHandle, nil)
	ip.On("CommitHandle", op, podHandle, podID, int32(-1)).Return(nil)
	ip.On("Handle", op, podID, podName).Return(podHandle, nil)
	ip.On("BindScope", op, podHandle, podName).Return(podHandle, fakeEP, nil)
	ip.On("SetState", op, podHandle, podName, "RUNNING").Return(podHandle, nil)

	store.On("Get", op, "busybox", "", true).Return(&metadata.ImageConfig{}, nil)
	store.On("Get", op, "alpine", "", true).Return(&metadata.ImageConfig{}, nil)

	// The test
	c, err := NewPodCreator(client, store, ip, cache, persona, portlayer)
	assert.NotNil(t, c, "Expected not-nil creating a pod creator but received nil")

	err = c.CreatePod(op, &pod, true)
	assert.NotNil(t, err, "Expected nil error from createPod")
	assert.Equal(t, err.Error(), fakeErr.Error())
}

func TestCreatePod_CreateHandleTaskError(t *testing.T) {
	store, ip, cache, op := createMocks(t)
	client := client.Default
	persona := "1.2.3.4"
	portlayer := "1.2.3.4"

	// Setup mocks
	fakeErr := fmt.Errorf("fake create handle task error")
	ip.On("CreateHandle", op).Return(podID, podHandle, nil)
	ip.On("AddImageToHandle", op, podHandle, "busybox-container", "", "", "").Return(podHandle, nil)
	ip.On("AddImageToHandle", op, podHandle, "alpine-container", "", "", "").Return(podHandle, nil)
	ip.On("CreateHandleTask", op, podHandle, podID, "", busyboxIsoConfig).Return(podHandle, fakeErr)
	ip.On("CreateHandleTask", op, podHandle, "Container-1-task", "", alpineIsoConfig).Return(podHandle, fakeErr)
	ip.On("AddHandleToScope", op, podHandle, busyboxIsoConfig).Return(podHandle, nil)
	ip.On("AddHandleToScope", op, podHandle, alpineIsoConfig).Return(podHandle, nil)
	ip.On("AddInteractionToHandle", op, podHandle).Return(podHandle, nil)
	ip.On("AddLoggingToHandle", op, podHandle).Return(podHandle, nil)
	ip.On("CommitHandle", op, podHandle, podID, int32(-1)).Return(nil)
	ip.On("Handle", op, podID, podName).Return(podHandle, nil)
	ip.On("BindScope", op, podHandle, podName).Return(podHandle, fakeEP, nil)
	ip.On("SetState", op, podHandle, podName, "RUNNING").Return(podHandle, nil)

	store.On("Get", op, "busybox", "", true).Return(&metadata.ImageConfig{}, nil)
	store.On("Get", op, "alpine", "", true).Return(&metadata.ImageConfig{}, nil)

	// The test
	c, err := NewPodCreator(client, store, ip, cache, persona, portlayer)
	assert.NotNil(t, c, "Expected not-nil creating a pod creator but received nil")

	err = c.CreatePod(op, &pod, true)
	assert.NotNil(t, err, "Expected nil error from createPod")
	assert.Equal(t, err.Error(), fakeErr.Error())
}

func TestCreatePod_AddHandleToScopeError(t *testing.T) {
	store, ip, cache, op := createMocks(t)
	client := client.Default
	persona := "1.2.3.4"
	portlayer := "1.2.3.4"

	// Setup mocks
	fakeErr := fmt.Errorf("fake add handle to scope error")
	ip.On("CreateHandle", op).Return(podID, podHandle, nil)
	ip.On("AddImageToHandle", op, podHandle, "busybox-container", "", "", "").Return(podHandle, nil)
	ip.On("AddImageToHandle", op, podHandle, "alpine-container", "", "", "").Return(podHandle, nil)
	ip.On("CreateHandleTask", op, podHandle, podID, "", busyboxIsoConfig).Return(podHandle, nil)
	ip.On("CreateHandleTask", op, podHandle, "Container-1-task", "", alpineIsoConfig).Return(podHandle, nil)
	ip.On("AddHandleToScope", op, podHandle, busyboxIsoConfig).Return(podHandle, fakeErr)
	ip.On("AddHandleToScope", op, podHandle, alpineIsoConfig).Return(podHandle, fakeErr)
	ip.On("AddInteractionToHandle", op, podHandle).Return(podHandle, nil)
	ip.On("AddLoggingToHandle", op, podHandle).Return(podHandle, nil)
	ip.On("CommitHandle", op, podHandle, podID, int32(-1)).Return(nil)
	ip.On("Handle", op, podID, podName).Return(podHandle, nil)
	ip.On("BindScope", op, podHandle, podName).Return(podHandle, fakeEP, nil)
	ip.On("SetState", op, podHandle, podName, "RUNNING").Return(podHandle, nil)

	store.On("Get", op, "busybox", "", true).Return(&metadata.ImageConfig{}, nil)
	store.On("Get", op, "alpine", "", true).Return(&metadata.ImageConfig{}, nil)

	// The test
	c, err := NewPodCreator(client, store, ip, cache, persona, portlayer)
	assert.NotNil(t, c, "Expected not-nil creating a pod creator but received nil")

	err = c.CreatePod(op, &pod, true)
	assert.NotNil(t, err, "Expected nil error from createPod")
	assert.Equal(t, err.Error(), fakeErr.Error())
}

func TestCreatePod_AddInteractionError(t *testing.T) {
	store, ip, cache, op := createMocks(t)
	client := client.Default
	persona := "1.2.3.4"
	portlayer := "1.2.3.4"

	// Setup mocks
	fakeErr := fmt.Errorf("fake add interaction error")
	ip.On("CreateHandle", op).Return(podID, podHandle, nil)
	ip.On("AddImageToHandle", op, podHandle, "busybox-container", "", "", "").Return(podHandle, nil)
	ip.On("AddImageToHandle", op, podHandle, "alpine-container", "", "", "").Return(podHandle, nil)
	ip.On("CreateHandleTask", op, podHandle, podID, "", busyboxIsoConfig).Return(podHandle, nil)
	ip.On("CreateHandleTask", op, podHandle, "Container-1-task", "", alpineIsoConfig).Return(podHandle, nil)
	ip.On("AddHandleToScope", op, podHandle, busyboxIsoConfig).Return(podHandle, nil)
	ip.On("AddHandleToScope", op, podHandle, alpineIsoConfig).Return(podHandle, nil)
	ip.On("AddInteractionToHandle", op, podHandle).Return(podHandle, fakeErr)
	ip.On("AddLoggingToHandle", op, podHandle).Return(podHandle, nil)
	ip.On("CommitHandle", op, podHandle, podID, int32(-1)).Return(nil)
	ip.On("Handle", op, podID, podName).Return(podHandle, nil)
	ip.On("BindScope", op, podHandle, podName).Return(podHandle, fakeEP, nil)
	ip.On("SetState", op, podHandle, podName, "RUNNING").Return(podHandle, nil)

	store.On("Get", op, "busybox", "", true).Return(&metadata.ImageConfig{}, nil)
	store.On("Get", op, "alpine", "", true).Return(&metadata.ImageConfig{}, nil)

	// The test
	c, err := NewPodCreator(client, store, ip, cache, persona, portlayer)
	assert.NotNil(t, c, "Expected not-nil creating a pod creator but received nil")

	err = c.CreatePod(op, &pod, true)
	assert.NotNil(t, err, "Expected nil error from createPod")
	assert.Equal(t, err.Error(), fakeErr.Error())
}

func TestCreatePod_AddLoggingError(t *testing.T) {
	store, ip, cache, op := createMocks(t)
	client := client.Default
	persona := "1.2.3.4"
	portlayer := "1.2.3.4"

	// Setup mocks
	fakeErr := fmt.Errorf("fake add logging error")
	ip.On("CreateHandle", op).Return(podID, podHandle, nil)
	ip.On("AddImageToHandle", op, podHandle, "busybox-container", "", "", "").Return(podHandle, nil)
	ip.On("AddImageToHandle", op, podHandle, "alpine-container", "", "", "").Return(podHandle, nil)
	ip.On("CreateHandleTask", op, podHandle, podID, "", busyboxIsoConfig).Return(podHandle, nil)
	ip.On("CreateHandleTask", op, podHandle, "Container-1-task", "", alpineIsoConfig).Return(podHandle, nil)
	ip.On("AddHandleToScope", op, podHandle, busyboxIsoConfig).Return(podHandle, nil)
	ip.On("AddHandleToScope", op, podHandle, alpineIsoConfig).Return(podHandle, nil)
	ip.On("AddInteractionToHandle", op, podHandle).Return(podHandle, nil)
	ip.On("AddLoggingToHandle", op, podHandle).Return(podHandle, fakeErr)
	ip.On("CommitHandle", op, podHandle, podID, int32(-1)).Return(nil)
	ip.On("Handle", op, podID, podName).Return(podHandle, nil)
	ip.On("BindScope", op, podHandle, podName).Return(podHandle, fakeEP, nil)
	ip.On("SetState", op, podHandle, podName, "RUNNING").Return(podHandle, nil)

	store.On("Get", op, "busybox", "", true).Return(&metadata.ImageConfig{}, nil)
	store.On("Get", op, "alpine", "", true).Return(&metadata.ImageConfig{}, nil)

	// The test
	c, err := NewPodCreator(client, store, ip, cache, persona, portlayer)
	assert.NotNil(t, c, "Expected not-nil creating a pod creator but received nil")

	err = c.CreatePod(op, &pod, true)
	assert.NotNil(t, err, "Expected nil error from createPod")
	assert.Equal(t, err.Error(), fakeErr.Error())
}

func TestCreatePod_CommitError(t *testing.T) {
	store, ip, cache, op := createMocks(t)
	client := client.Default
	persona := "1.2.3.4"
	portlayer := "1.2.3.4"

	// Setup mocks
	fakeErr := fmt.Errorf("fake commit error")
	ip.On("CreateHandle", op).Return(podID, podHandle, nil)
	ip.On("AddImageToHandle", op, podHandle, "busybox-container", "", "", "").Return(podHandle, nil)
	ip.On("AddImageToHandle", op, podHandle, "alpine-container", "", "", "").Return(podHandle, nil)
	ip.On("CreateHandleTask", op, podHandle, podID, "", busyboxIsoConfig).Return(podHandle, nil)
	ip.On("CreateHandleTask", op, podHandle, "Container-1-task", "", alpineIsoConfig).Return(podHandle, nil)
	ip.On("AddHandleToScope", op, podHandle, busyboxIsoConfig).Return(podHandle, nil)
	ip.On("AddHandleToScope", op, podHandle, alpineIsoConfig).Return(podHandle, nil)
	ip.On("AddInteractionToHandle", op, podHandle).Return(podHandle, nil)
	ip.On("AddLoggingToHandle", op, podHandle).Return(podHandle, nil)
	ip.On("CommitHandle", op, podHandle, podID, int32(-1)).Return(fakeErr)
	ip.On("Handle", op, podID, podName).Return(podHandle, nil)
	ip.On("BindScope", op, podHandle, podName).Return(podHandle, fakeEP, nil)
	ip.On("SetState", op, podHandle, podName, "RUNNING").Return(podHandle, nil)

	store.On("Get", op, "busybox", "", true).Return(&metadata.ImageConfig{}, nil)
	store.On("Get", op, "alpine", "", true).Return(&metadata.ImageConfig{}, nil)

	// The test
	c, err := NewPodCreator(client, store, ip, cache, persona, portlayer)
	assert.NotNil(t, c, "Expected not-nil creating a pod creator but received nil")

	err = c.CreatePod(op, &pod, true)
	assert.NotNil(t, err, "Expected nil error from createPod")
	assert.Equal(t, err.Error(), fakeErr.Error())
}

func TestCreatePod_HandleError(t *testing.T) {
	store, ip, cache, op := createMocks(t)
	client := client.Default
	persona := "1.2.3.4"
	portlayer := "1.2.3.4"

	// Setup mocks
	fakeErr := fmt.Errorf("fake handle error")
	ip.On("CreateHandle", op).Return(podID, podHandle, nil)
	ip.On("AddImageToHandle", op, podHandle, "busybox-container", "", "", "").Return(podHandle, nil)
	ip.On("AddImageToHandle", op, podHandle, "alpine-container", "", "", "").Return(podHandle, nil)
	ip.On("CreateHandleTask", op, podHandle, podID, "", busyboxIsoConfig).Return(podHandle, nil)
	ip.On("CreateHandleTask", op, podHandle, "Container-1-task", "", alpineIsoConfig).Return(podHandle, nil)
	ip.On("AddHandleToScope", op, podHandle, busyboxIsoConfig).Return(podHandle, nil)
	ip.On("AddHandleToScope", op, podHandle, alpineIsoConfig).Return(podHandle, nil)
	ip.On("AddInteractionToHandle", op, podHandle).Return(podHandle, nil)
	ip.On("AddLoggingToHandle", op, podHandle).Return(podHandle, nil)
	ip.On("CommitHandle", op, podHandle, podID, int32(-1)).Return(nil)
	ip.On("Handle", op, podID, podName).Return(podHandle, fakeErr)
	ip.On("BindScope", op, podHandle, podName).Return(podHandle, fakeEP, nil)
	ip.On("SetState", op, podHandle, podName, "RUNNING").Return(podHandle, nil)

	store.On("Get", op, "busybox", "", true).Return(&metadata.ImageConfig{}, nil)
	store.On("Get", op, "alpine", "", true).Return(&metadata.ImageConfig{}, nil)

	// The test
	c, err := NewPodCreator(client, store, ip, cache, persona, portlayer)
	assert.NotNil(t, c, "Expected not-nil creating a pod creator but received nil")

	err = c.CreatePod(op, &pod, true)
	assert.NotNil(t, err, "Expected nil error from createPod")
	assert.Equal(t, err.Error(), fakeErr.Error())
}

func TestCreatePod_BindScopeError(t *testing.T) {
	store, ip, cache, op := createMocks(t)
	client := client.Default
	persona := "1.2.3.4"
	portlayer := "1.2.3.4"

	// Setup mocks
	fakeErr := fmt.Errorf("fake bind scope error")
	ip.On("CreateHandle", op).Return(podID, podHandle, nil)
	ip.On("AddImageToHandle", op, podHandle, "busybox-container", "", "", "").Return(podHandle, nil)
	ip.On("AddImageToHandle", op, podHandle, "alpine-container", "", "", "").Return(podHandle, nil)
	ip.On("CreateHandleTask", op, podHandle, podID, "", busyboxIsoConfig).Return(podHandle, nil)
	ip.On("CreateHandleTask", op, podHandle, "Container-1-task", "", alpineIsoConfig).Return(podHandle, nil)
	ip.On("AddHandleToScope", op, podHandle, busyboxIsoConfig).Return(podHandle, nil)
	ip.On("AddHandleToScope", op, podHandle, alpineIsoConfig).Return(podHandle, nil)
	ip.On("AddInteractionToHandle", op, podHandle).Return(podHandle, nil)
	ip.On("AddLoggingToHandle", op, podHandle).Return(podHandle, nil)
	ip.On("CommitHandle", op, podHandle, podID, int32(-1)).Return(nil)
	ip.On("Handle", op, podID, podName).Return(podHandle, nil)
	ip.On("BindScope", op, podHandle, podName).Return(podHandle, fakeEP, fakeErr)
	ip.On("UnbindScope", op, podHandle, podName).Return(podHandle, fakeEP, nil)
	ip.On("SetState", op, podHandle, podName, "RUNNING").Return(podHandle, nil)

	store.On("Get", op, "busybox", "", true).Return(&metadata.ImageConfig{}, nil)
	store.On("Get", op, "alpine", "", true).Return(&metadata.ImageConfig{}, nil)

	// The test
	c, err := NewPodCreator(client, store, ip, cache, persona, portlayer)
	assert.NotNil(t, c, "Expected not-nil creating a pod creator but received nil")

	err = c.CreatePod(op, &pod, true)
	assert.NotNil(t, err, "Expected nil error from createPod")
	assert.Equal(t, err.Error(), fakeErr.Error())
}

func TestCreatePod_SetStateError(t *testing.T) {
	store, ip, cache, op := createMocks(t)
	client := client.Default
	persona := "1.2.3.4"
	portlayer := "1.2.3.4"

	// Setup mocks
	fakeErr := fmt.Errorf("fake set state error")
	ip.On("CreateHandle", op).Return(podID, podHandle, nil)
	ip.On("AddImageToHandle", op, podHandle, "busybox-container", "", "", "").Return(podHandle, nil)
	ip.On("AddImageToHandle", op, podHandle, "alpine-container", "", "", "").Return(podHandle, nil)
	ip.On("CreateHandleTask", op, podHandle, podID, "", busyboxIsoConfig).Return(podHandle, nil)
	ip.On("CreateHandleTask", op, podHandle, "Container-1-task", "", alpineIsoConfig).Return(podHandle, nil)
	ip.On("AddHandleToScope", op, podHandle, busyboxIsoConfig).Return(podHandle, nil)
	ip.On("AddHandleToScope", op, podHandle, alpineIsoConfig).Return(podHandle, nil)
	ip.On("AddInteractionToHandle", op, podHandle).Return(podHandle, nil)
	ip.On("AddLoggingToHandle", op, podHandle).Return(podHandle, nil)
	ip.On("CommitHandle", op, podHandle, podID, int32(-1)).Return(nil)
	ip.On("Handle", op, podID, podName).Return(podHandle, nil)
	ip.On("BindScope", op, podHandle, podName).Return(podHandle, fakeEP, nil)
	ip.On("UnbindScope", op, podHandle, podName).Return(podHandle, fakeEP, nil)
	ip.On("SetState", op, podHandle, podName, "RUNNING").Return(podHandle, fakeErr)

	store.On("Get", op, "busybox", "", true).Return(&metadata.ImageConfig{}, nil)
	store.On("Get", op, "alpine", "", true).Return(&metadata.ImageConfig{}, nil)

	// The test
	c, err := NewPodCreator(client, store, ip, cache, persona, portlayer)
	assert.NotNil(t, c, "Expected not-nil creating a pod creator but received nil")

	err = c.CreatePod(op, &pod, true)
	assert.NotNil(t, err, "Expected nil error from createPod")
	assert.Equal(t, err.Error(), fakeErr.Error())
}

func init() {
	pod = v1.Pod{
		//TypeMeta: v1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:                       "busybox-sleep",
			GenerateName:               "",
			Namespace:                  "default",
			SelfLink:                   "/api/v1/namespaces/default/pods/busybox-sleep",
			UID:                        "b1fc6e1b-499b-11e8-946c-000c29479092",
			ResourceVersion:            "10338145",
			Generation:                 0,
			DeletionTimestamp:          nil,
			DeletionGracePeriodSeconds: nil,
			Labels:          map[string]string{},
			Annotations:     map[string]string{},
			OwnerReferences: nil,
			Initializers:    nil,
			Finalizers:      nil,
			ClusterName:     "",
		},
		Spec: v1.PodSpec{
			Volumes: []v1.Volume{
				{
					Name: "default-token-9q9lr",
					VolumeSource: v1.VolumeSource{
						HostPath:             nil,
						EmptyDir:             nil,
						GCEPersistentDisk:    nil,
						AWSElasticBlockStore: nil,
						GitRepo:              nil,
						Secret: &v1.SecretVolumeSource{
							SecretName: "default-token-9q9lr",
							Items:      nil,
							Optional:   nil,
						},
						NFS:                   nil,
						ISCSI:                 nil,
						Glusterfs:             nil,
						PersistentVolumeClaim: nil,
						RBD:                  nil,
						FlexVolume:           nil,
						Cinder:               nil,
						CephFS:               nil,
						Flocker:              nil,
						DownwardAPI:          nil,
						FC:                   nil,
						AzureFile:            nil,
						ConfigMap:            nil,
						VsphereVolume:        nil,
						Quobyte:              nil,
						AzureDisk:            nil,
						PhotonPersistentDisk: nil,
						Projected:            nil,
						PortworxVolume:       nil,
						ScaleIO:              nil,
						StorageOS:            nil,
					},
				},
			},
			InitContainers: nil,
			Containers: []v1.Container{
				{
					Name:       "busybox-container",
					Image:      "busybox",
					Command:    []string{"/bin/sleep"},
					Args:       []string{"2m"},
					WorkingDir: "",
					Ports:      nil,
					EnvFrom:    nil,
					Env:        nil,
					Resources:  v1.ResourceRequirements{},
					VolumeMounts: []v1.VolumeMount{
						{
							Name:             "default-token-9q9lr",
							ReadOnly:         true,
							MountPath:        "/var/run/secrets/kubernetes.io/serviceaccount",
							SubPath:          "",
							MountPropagation: nil,
						},
					},
					LivenessProbe:            nil,
					ReadinessProbe:           nil,
					Lifecycle:                nil,
					TerminationMessagePath:   "/dev/termination-log",
					TerminationMessagePolicy: "File",
					ImagePullPolicy:          "IfNotPresent",
					SecurityContext:          nil,
					Stdin:                    false,
					StdinOnce:                false,
					TTY:                      false,
				},
				{
					Name:       "alpine-container",
					Image:      "alpine",
					Command:    nil,
					Args:       nil,
					WorkingDir: "",
					Ports:      nil,
					EnvFrom:    nil,
					Env:        nil,
					Resources:  v1.ResourceRequirements{},
					VolumeMounts: []v1.VolumeMount{
						{
							Name:             "default-token-9q9lr",
							ReadOnly:         true,
							MountPath:        "/var/run/secrets/kubernetes.io/serviceaccount",
							SubPath:          "",
							MountPropagation: nil,
						},
					},
					LivenessProbe:            nil,
					ReadinessProbe:           nil,
					Lifecycle:                nil,
					TerminationMessagePath:   "/dev/termination-log",
					TerminationMessagePolicy: "File",
					ImagePullPolicy:          "IfNotPresent",
					SecurityContext:          nil,
					Stdin:                    false,
					StdinOnce:                false,
					TTY:                      false,
				},
			},
			RestartPolicy:                 "Always",
			TerminationGracePeriodSeconds: new(int64),
			ActiveDeadlineSeconds:         nil,
			DNSPolicy:                     "ClusterFirst",
			NodeSelector:                  map[string]string{"affinity": "vmware"},
			ServiceAccountName:            "default",
			DeprecatedServiceAccount:      "default",
			AutomountServiceAccountToken:  nil,
			NodeName:                      "vic-kubelet",
			HostNetwork:                   false,
			HostPID:                       false,
			HostIPC:                       false,
			SecurityContext:               &v1.PodSecurityContext{},
			ImagePullSecrets:              nil,
			Hostname:                      "",
			Subdomain:                     "",
			Affinity:                      nil,
			SchedulerName:                 "default-scheduler",
			Tolerations: []v1.Toleration{
				{
					Key:               "node.kubernetes.io/not-ready",
					Operator:          "Exists",
					Value:             "",
					Effect:            "NoExecute",
					TolerationSeconds: new(int64),
				},
				{
					Key:               "node.kubernetes.io/unreachable",
					Operator:          "Exists",
					Value:             "",
					Effect:            "NoExecute",
					TolerationSeconds: new(int64),
				},
			},
			HostAliases:       nil,
			PriorityClassName: "",
			Priority:          nil,
		},
	}

	busyboxIsoConfig = proxy.IsolationContainerConfig{
		ID:         "",
		ImageID:    "",
		LayerID:    "",
		ImageName:  "busybox",
		Name:       "busybox-container",
		Namespace:  "",
		Cmd:        []string{"/bin/sleep", "2m"},
		Path:       "",
		Entrypoint: nil,
		Env:        nil,
		WorkingDir: "",
		User:       "",
		StopSignal: "",
		Attach:     false,
		StdinOnce:  false,
		OpenStdin:  false,
		Tty:        false,
		CPUCount:   2,
		Memory:     2048,
		PortMap:    map[string]proxy.PortBinding{},
	}

	alpineIsoConfig = proxy.IsolationContainerConfig{
		ID:         "",
		ImageID:    "",
		LayerID:    "",
		ImageName:  "alpine",
		Name:       "alpine-container",
		Namespace:  "",
		Cmd:        nil,
		Path:       "",
		Entrypoint: nil,
		Env:        nil,
		WorkingDir: "",
		User:       "",
		StopSignal: "",
		Attach:     false,
		StdinOnce:  false,
		OpenStdin:  false,
		Tty:        false,
		CPUCount:   2,
		Memory:     2048,
		PortMap:    map[string]proxy.PortBinding{},
	}
}
