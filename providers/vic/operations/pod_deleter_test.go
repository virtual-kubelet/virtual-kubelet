package operations

import (
	"testing"

	"github.com/vmware/vic/lib/apiservers/portlayer/client"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
)

func init() {
	initPod()
}

func TestNewPodDeleter(t *testing.T) {
	_, ip, cache, _ := createMocks(t)
	client := client.Default
	persona := "1.2.3.4"
	portlayer := "1.2.3.4"

	// Positive Cases
	d, err := NewPodDeleter(client, ip, cache, persona, portlayer)
	assert.Check(t, d != nil, "Expected non-nil creating a pod Deleter but received nil")

	// Negative Cases
	d, err = NewPodDeleter(nil, ip, cache, persona, portlayer)
	assert.Check(t, is.Nil(d), "Expected nil")
	assert.Check(t, is.DeepEqual(err, PodDeleterPortlayerClientError))

	d, err = NewPodDeleter(client, nil, cache, persona, portlayer)
	assert.Check(t, is.Nil(d), "Expected nil")
	assert.Check(t, is.DeepEqual(err, PodDeleterIsolationProxyError))

	d, err = NewPodDeleter(client, ip, nil, persona, portlayer)
	assert.Check(t, is.Nil(d), "Expected nil")
	assert.Check(t, is.DeepEqual(err, PodDeleterPodCacheError))
}

func TestDeletePod(t *testing.T) {
	client := client.Default
	_, ip, cache, op := createMocks(t)

	persona := "1.2.3.4"
	portlayer := "1.2.3.4"

	d, err := NewPodDeleter(client, ip, cache, persona, portlayer)
	assert.Check(t, d != nil, "Expected non-nil creating a pod Deleter but received nil")
	assert.Check(t, err, "Expected nil")

	// Set up the mocks for this test
	ip.On("Handle", op, podID, podName).Return(podHandle, nil)
	ip.On("UnbindScope", op, podHandle, podName).Return(podHandle, fakeEP, nil)
	ip.On("SetState", op, podHandle, podName, "STOPPED").Return(podHandle, nil)
	ip.On("CommitHandle", op, podHandle, podID, int32(-1)).Return(nil)
	ip.On("Remove", op, podID, true).Return(nil)

	// Add vicPod to the cache
	cache.Add(op, "", pod.Name, &vicPod)

	// Positive case
	err = d.DeletePod(op, &pod)
	assert.Check(t, err, "Expected nil")
}

func TestDeletePodErrorHandle(t *testing.T) {
	client := client.Default
	_, ip, cache, op := createMocks(t)

	persona := "1.2.3.4"
	portlayer := "1.2.3.4"

	d, err := NewPodDeleter(client, ip, cache, persona, portlayer)
	assert.Check(t, d != nil, "Expected non-nil creating a pod Deleter but received nil")
	assert.Check(t, err, "Expected nil")

	// Set up the mocks for this test
	ip.On("UnbindScope", op, podHandle, podName).Return(podHandle, fakeEP, nil)
	ip.On("SetState", op, podHandle, podName, "STOPPED").Return(podHandle, nil)
	ip.On("CommitHandle", op, podHandle, podID, int32(-1)).Return(nil)
	ip.On("Remove", op, podID, true).Return(nil)

	// Add vicPod to the cache
	cache.Add(op, "", pod.Name, &vicPod)

	// Failed Handle
	fakeErr := fakeError("invalid handle")
	ip.On("Handle", op, podID, podName).Return("", fakeErr)

	err = d.DeletePod(op, &pod)
	assert.Check(t, is.Error(err, fakeErr.Error()), "Expected invalid handle error")
}

func TestDeletePodErrorUnbindScope(t *testing.T) {
	client := client.Default
	_, ip, cache, op := createMocks(t)

	persona := "1.2.3.4"
	portlayer := "1.2.3.4"

	d, err := NewPodDeleter(client, ip, cache, persona, portlayer)
	assert.Check(t, d != nil, "Expected non-nil creating a pod Deleter but received nil")
	assert.Check(t, err, "Expected nil")

	// Set up the mocks for this test
	ip.On("Handle", op, podID, podName).Return(podHandle, nil)
	ip.On("SetState", op, podHandle, podName, "STOPPED").Return(podHandle, nil)
	ip.On("CommitHandle", op, podHandle, podID, int32(-1)).Return(nil)
	ip.On("Remove", op, podID, true).Return(nil)

	// Add vicPod to the cache
	cache.Add(op, "", pod.Name, &vicPod)
	// Failed UnbindScope
	fakeErr := fakeError("failed UnbindScope")
	ip.On("UnbindScope", op, podHandle, podName).Return("", nil, fakeErr)

	err = d.DeletePod(op, &pod)
	assert.Check(t, is.Error(err, fakeErr.Error()), "Expected failed UnbindScope error")
}

func TestDeletePodErrorSetState(t *testing.T) {
	client := client.Default
	_, ip, cache, op := createMocks(t)

	persona := "1.2.3.4"
	portlayer := "1.2.3.4"

	d, err := NewPodDeleter(client, ip, cache, persona, portlayer)
	assert.Check(t, d != nil, "Expected non-nil creating a pod Deleter but received nil")
	assert.Check(t, err, "Expected nil")

	// Set up the mocks for this test
	ip.On("Handle", op, podID, podName).Return(podHandle, nil)
	ip.On("UnbindScope", op, podHandle, podName).Return(podHandle, fakeEP, nil)
	ip.On("CommitHandle", op, podHandle, podID, int32(-1)).Return(nil)
	ip.On("Remove", op, podID, true).Return(nil)

	// Add vicPod to the cache
	cache.Add(op, "", pod.Name, &vicPod)

	// Failed SetState
	fakeErr := fakeError("failed SetState")
	ip.On("SetState", op, podHandle, podName, "STOPPED").Return("", fakeErr)
	err = d.DeletePod(op, &pod)
	assert.Check(t, is.Error(err, fakeErr.Error()), "Expected failed SetState error")
}

func TestDeletePodErrorCommitHandle(t *testing.T) {
	client := client.Default
	_, ip, cache, op := createMocks(t)

	persona := "1.2.3.4"
	portlayer := "1.2.3.4"

	d, err := NewPodDeleter(client, ip, cache, persona, portlayer)
	assert.Check(t, d != nil, "Expected non-nil creating a pod Deleter but received nil")
	assert.Check(t, err, "Expected nil")

	// Set up the mocks for this test
	ip.On("Handle", op, podID, podName).Return(podHandle, nil)
	ip.On("UnbindScope", op, podHandle, podName).Return(podHandle, fakeEP, nil)
	ip.On("SetState", op, podHandle, podName, "STOPPED").Return(podHandle, nil)
	ip.On("Remove", op, podID, true).Return(nil)

	// Add vicPod to the cache
	cache.Add(op, "", pod.Name, &vicPod)
	// Failed Commit
	fakeErr := fakeError("failed Commit")
	ip.On("CommitHandle", op, podHandle, podID, int32(-1)).Return(fakeErr)
	err = d.DeletePod(op, &pod)
	assert.Check(t, is.Error(err, fakeErr.Error()), "Expected failed Commit error")
}

func TestDeletePodErrorRemove(t *testing.T) {
	client := client.Default
	_, ip, cache, op := createMocks(t)

	persona := "1.2.3.4"
	portlayer := "1.2.3.4"

	d, err := NewPodDeleter(client, ip, cache, persona, portlayer)
	assert.Check(t, d != nil, "Expected non-nil creating a pod Deleter but received nil")
	assert.Check(t, err, "Expected nil")

	// Set up the mocks for this test
	ip.On("Handle", op, podID, podName).Return(podHandle, nil)
	ip.On("UnbindScope", op, podHandle, podName).Return(podHandle, fakeEP, nil)
	ip.On("SetState", op, podHandle, podName, "STOPPED").Return(podHandle, nil)
	ip.On("CommitHandle", op, podHandle, podID, int32(-1)).Return(nil)

	// Add vicPod to the cache
	cache.Add(op, "", pod.Name, &vicPod)

	// Failed Remove
	fakeErr := fakeError("failed Remove")
	ip.On("Remove", op, podID, true).Return(fakeErr)
	err = d.DeletePod(op, &pod)
	assert.Check(t, is.Error(err, fakeErr.Error()), "Expected failed Remove error")
}

func TestDeletePodErrorBadArgs(t *testing.T) {
	client := client.Default
	_, ip, cache, op := createMocks(t)
	persona := "1.2.3.4"
	portlayer := "1.2.3.4"

	d, err := NewPodDeleter(client, ip, cache, persona, portlayer)
	assert.Check(t, d != nil, "Expected non-nil creating a pod Deleter but received nil")

	// Negative Cases
	err = d.DeletePod(op, nil)
	assert.Check(t, is.DeepEqual(err, PodDeleterInvalidPodSpecError))
}
