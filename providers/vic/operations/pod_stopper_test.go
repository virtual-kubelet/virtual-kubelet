package operations

import (
	"testing"

	"github.com/vmware/vic/lib/apiservers/portlayer/client"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
)

func TestNewPodStopper(t *testing.T) {
	_, ip, _, _ := createMocks(t)
	client := client.Default

	// Positive Cases
	s, err := NewPodStopper(client, ip)
	assert.Check(t, s != nil, "Expected non-nil creating a pod Stopper but received nil")

	// Negative Cases
	s, err = NewPodStopper(nil, ip)
	assert.Check(t, is.Nil(s), "Expected nil")
	assert.Check(t, is.DeepEqual(err, PodStopperPortlayerClientError))

	s, err = NewPodStopper(client, nil)
	assert.Check(t, is.Nil(s), "Expected nil")
	assert.Check(t, is.DeepEqual(err, PodStopperIsolationProxyError))
}

func TestStopPod(t *testing.T) {
	client := client.Default
	_, ip, _, op := createMocks(t)

	// Start with arguments
	s, err := NewPodStopper(client, ip)
	assert.Check(t, s != nil, "Expected non-nil creating a pod Stopper but received nil")
	assert.Check(t, err, "Expected nil")

	// Set up the mocks for this test
	ip.On("Handle", op, podID, podName).Return(podHandle, nil)
	ip.On("UnbindScope", op, podHandle, podName).Return(podHandle, fakeEP, nil)
	ip.On("SetState", op, podHandle, podName, "STOPPED").Return(podHandle, nil)
	ip.On("CommitHandle", op, podHandle, podID, int32(-1)).Return(nil)

	// Positive case
	err = s.Stop(op, podID, podName)
	assert.Check(t, err, "Expected nil")
}

func TestStopPodErrorHandle(t *testing.T) {
	client := client.Default
	_, ip, _, op := createMocks(t)

	// Start with arguments
	s, err := NewPodStopper(client, ip)
	assert.Check(t, s != nil, "Expected non-nil creating a pod Stopper but received nil")
	assert.Check(t, err, "Expected nil")

	// Set up the mocks for this test
	ip.On("UnbindScope", op, podHandle, podName).Return(podHandle, fakeEP, nil)
	ip.On("SetState", op, podHandle, podName, "STOPPED").Return(podHandle, nil)
	ip.On("CommitHandle", op, podHandle, podID, int32(-1)).Return(nil)

	// Failed Handle
	fakeErr := fakeError("invalid handle")
	ip.On("Handle", op, podID, podName).Return("", fakeErr)

	err = s.Stop(op, podID, podName)
	assert.Check(t, is.DeepEqual(err, fakeErr), "Expected invalid handle error")
}

func TestStopPodErrorUnbindScope(t *testing.T) {
	client := client.Default
	_, ip, _, op := createMocks(t)

	// Start with arguments
	s, err := NewPodStopper(client, ip)
	assert.Check(t, s != nil, "Expected non-nil creating a pod Stopper but received nil")
	assert.Check(t, err, "Expected nil")

	// Set up the mocks for this test
	ip.On("Handle", op, podID, podName).Return(podHandle, nil)
	ip.On("SetState", op, podHandle, podName, "STOPPED").Return(podHandle, nil)
	ip.On("CommitHandle", op, podHandle, podID, int32(-1)).Return(nil)

	// Failed UnbindScope
	fakeErr := fakeError("failed UnbindScope")
	ip.On("UnbindScope", op, podHandle, podName).Return("", nil, fakeErr)

	err = s.Stop(op, podID, podName)
	assert.Check(t, is.DeepEqual(err, fakeErr), "Expected failed UnbindScope error")
}

func TestStopPodErrorSetState(t *testing.T) {
	client := client.Default
	_, ip, _, op := createMocks(t)

	// Start with arguments
	s, err := NewPodStopper(client, ip)
	assert.Check(t, s != nil, "Expected non-nil creating a pod Stopper but received nil")
	assert.Check(t, err, "Expected nil")

	// Set up the mocks for this test
	ip.On("Handle", op, podID, podName).Return(podHandle, nil)
	ip.On("UnbindScope", op, podHandle, podName).Return(podHandle, fakeEP, nil)
	ip.On("CommitHandle", op, podHandle, podID, int32(-1)).Return(nil)

	// Failed SetState
	fakeErr := fakeError("failed SetState")
	ip.On("SetState", op, podHandle, podName, "STOPPED").Return("", fakeErr)
	err = s.Stop(op, podID, podName)
	assert.Check(t, is.DeepEqual(err, fakeErr), "Expected failed SetState error")
}

func TestStopPodErrorCommit(t *testing.T) {
	client := client.Default
	_, ip, _, op := createMocks(t)

	// Start with arguments
	s, err := NewPodStopper(client, ip)
	assert.Check(t, s != nil, "Expected non-nil creating a pod Stopper but received nil")
	assert.Check(t, err, "Expected nil")

	// Set up the mocks for this test
	ip.On("Handle", op, podID, podName).Return(podHandle, nil)
	ip.On("UnbindScope", op, podHandle, podName).Return(podHandle, fakeEP, nil)
	ip.On("SetState", op, podHandle, podName, "STOPPED").Return(podHandle, nil)

	// Failed Commit
	fakeErr := fakeError("failed Commit")
	ip.On("CommitHandle", op, podHandle, podID, int32(-1)).Return(fakeErr)
	err = s.Stop(op, podID, podName)
	assert.Check(t, is.DeepEqual(err, fakeErr), "Expected failed Commit error")
}
