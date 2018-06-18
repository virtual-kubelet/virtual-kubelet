package operations

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vmware/vic/lib/apiservers/portlayer/client"
	"github.com/vmware/vic/lib/metadata"
)

func init() {
	initPod()
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
