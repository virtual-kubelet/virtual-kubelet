package operations

import (
	"testing"

	"github.com/vmware/vic/lib/apiservers/portlayer/client"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
	v1 "k8s.io/api/core/v1"
)

func TestNewPodStatus(t *testing.T) {
	_, ip, _, _ := createMocks(t)
	client := client.Default

	// Positive Cases
	s, err := NewPodStatus(client, ip)
	assert.Check(t, s != nil, "Expected non-nil creating a pod Status but received nil")

	// Negative Cases
	s, err = NewPodStatus(nil, ip)
	assert.Check(t, is.Nil(s), "Expected nil")
	assert.Check(t, is.DeepEqual(err, PodStatusPortlayerClientError))

	s, err = NewPodStatus(client, nil)
	assert.Check(t, is.Nil(s), "Expected nil")
	assert.Check(t, is.DeepEqual(err, PodStatusIsolationProxyError))
}

func TestStatusPodStarting(t *testing.T) {
	client := client.Default
	_, ip, _, op := createMocks(t)

	s, err := NewPodStatus(client, ip)
	assert.Check(t, s != nil, "Expected non-nil creating a pod Status but received nil")
	assert.Check(t, err, "Expected nil")

	HostAddress := "1.2.3.4"
	EndpointAddresses := []string{
		"5.6.7.8/24",
	}

	// Set up the mocks for this test
	ip.On("State", op, podID, podName).Return(stateStarting, nil)
	ip.On("EpAddresses", op, podID, podName).Return(EndpointAddresses, nil)

	// Positive case
	status, err := s.GetStatus(op, podID, podName, HostAddress)
	assert.Check(t, err, "Expected nil")
	assert.Check(t, is.Equal(status.Phase, v1.PodPending), "Expected Phase Pending")
	verifyConditions(t, status.Conditions, v1.ConditionTrue, v1.ConditionFalse, v1.ConditionFalse)
	assert.Check(t, is.Equal(status.HostIP, "1.2.3.4"), "Expected Host IP Address")
	assert.Check(t, is.Equal(status.PodIP, "5.6.7.8"), "Expected Pod IP Address")
}

func TestStatusPodRunning(t *testing.T) {
	client := client.Default
	_, ip, _, op := createMocks(t)

	s, err := NewPodStatus(client, ip)
	assert.Check(t, s != nil, "Expected non-nil creating a pod Status but received nil")
	assert.Check(t, err, "Expected nil")

	HostAddress := "1.2.3.4"
	EndpointAddresses := []string{
		"5.6.7.8/24",
	}

	// Set up the mocks for this test
	ip.On("State", op, podID, podName).Return(stateRunning, nil)
	ip.On("EpAddresses", op, podID, podName).Return(EndpointAddresses, nil)

	// Pod Running case
	status, err := s.GetStatus(op, podID, podName, HostAddress)
	assert.Check(t, err, "Expected nil")
	assert.Check(t, is.Equal(status.Phase, v1.PodRunning), "Expected Phase PodRunning")
	verifyConditions(t, status.Conditions, v1.ConditionTrue, v1.ConditionTrue, v1.ConditionTrue)
	assert.Check(t, is.Equal(status.HostIP, "1.2.3.4"), "Expected Host IP Address")
	assert.Check(t, is.Equal(status.PodIP, "5.6.7.8"), "Expected Pod IP Address")
}

func TestStatusPodStopping(t *testing.T) {
	client := client.Default
	_, ip, _, op := createMocks(t)

	s, err := NewPodStatus(client, ip)
	assert.Check(t, s != nil, "Expected non-nil creating a pod Status but received nil")
	assert.Check(t, err, "Expected nil")

	HostAddress := "1.2.3.4"
	EndpointAddresses := []string{
		"5.6.7.8/24",
	}

	// Set up the mocks for this test
	ip.On("State", op, podID, podName).Return(stateStopping, nil)
	ip.On("EpAddresses", op, podID, podName).Return(EndpointAddresses, nil)

	// Pod error case
	status, err := s.GetStatus(op, podID, podName, HostAddress)

	assert.Check(t, is.Equal(status.Phase, v1.PodRunning), "Expected Phase PodFailed")
	verifyConditions(t, status.Conditions, v1.ConditionTrue, v1.ConditionTrue, v1.ConditionFalse)
	assert.Check(t, is.Equal(status.HostIP, "1.2.3.4"), "Expected Host IP Address")
	assert.Check(t, is.Equal(status.PodIP, "5.6.7.8"), "Expected Pod IP Address")
}

func TestStatusPodStopped(t *testing.T) {
	client := client.Default
	_, ip, _, op := createMocks(t)

	s, err := NewPodStatus(client, ip)
	assert.Check(t, s != nil, "Expected non-nil creating a pod Status but received nil")
	assert.Check(t, err, "Expected nil")

	HostAddress := "1.2.3.4"
	EndpointAddresses := []string{
		"5.6.7.8/24",
	}

	// Set up the mocks for this test
	ip.On("State", op, podID, podName).Return(stateStopped, nil)
	ip.On("EpAddresses", op, podID, podName).Return(EndpointAddresses, nil)

	// Pod error case
	status, err := s.GetStatus(op, podID, podName, HostAddress)

	assert.Check(t, is.Equal(status.Phase, v1.PodSucceeded), "Expected Phase PodFailed")
	verifyConditions(t, status.Conditions, v1.ConditionTrue, v1.ConditionTrue, v1.ConditionFalse)
	assert.Check(t, is.Equal(status.HostIP, "1.2.3.4"), "Expected Host IP Address")
	assert.Check(t, is.Equal(status.PodIP, "5.6.7.8"), "Expected Pod IP Address")
}

func TestStatusPodRemoving(t *testing.T) {
	client := client.Default
	_, ip, _, op := createMocks(t)

	s, err := NewPodStatus(client, ip)
	assert.Check(t, s != nil, "Expected non-nil creating a pod Status but received nil")
	assert.Check(t, err, "Expected nil")

	HostAddress := "1.2.3.4"
	EndpointAddresses := []string{
		"5.6.7.8/24",
	}

	// Set up the mocks for this test
	ip.On("State", op, podID, podName).Return(stateRemoving, nil)
	ip.On("EpAddresses", op, podID, podName).Return(EndpointAddresses, nil)

	// Pod error case
	status, err := s.GetStatus(op, podID, podName, HostAddress)

	assert.Check(t, is.Equal(status.Phase, v1.PodSucceeded), "Expected Phase PodFailed")
	verifyConditions(t, status.Conditions, v1.ConditionTrue, v1.ConditionTrue, v1.ConditionFalse)
	assert.Check(t, is.Equal(status.HostIP, "1.2.3.4"), "Expected Host IP Address")
	assert.Check(t, is.Equal(status.PodIP, "5.6.7.8"), "Expected Pod IP Address")
}

func TestStatusPodRemoved(t *testing.T) {
	client := client.Default
	_, ip, _, op := createMocks(t)

	s, err := NewPodStatus(client, ip)
	assert.Check(t, s != nil, "Expected non-nil creating a pod Status but received nil")
	assert.Check(t, err, "Expected nil")

	HostAddress := "1.2.3.4"
	EndpointAddresses := []string{
		"5.6.7.8/24",
	}

	// Set up the mocks for this test
	ip.On("State", op, podID, podName).Return(stateRemoved, nil)
	ip.On("EpAddresses", op, podID, podName).Return(EndpointAddresses, nil)

	// Pod error case
	status, err := s.GetStatus(op, podID, podName, HostAddress)

	assert.Check(t, is.Equal(status.Phase, v1.PodSucceeded), "Expected Phase PodFailed")
	verifyConditions(t, status.Conditions, v1.ConditionTrue, v1.ConditionTrue, v1.ConditionFalse)
	assert.Check(t, is.Equal(status.HostIP, "1.2.3.4"), "Expected Host IP Address")
	assert.Check(t, is.Equal(status.PodIP, "5.6.7.8"), "Expected Pod IP Address")
}

func TestStatusError(t *testing.T) {
	client := client.Default
	_, ip, _, op := createMocks(t)

	// Start with arguments
	s, err := NewPodStatus(client, ip)
	assert.Check(t, s != nil, "Expected non-nil creating a pod Status but received nil")
	assert.Check(t, err, "Expected nil")

	HostAddress := "0.0.0.0"

	// Set up the mocks for this test
	fakeErr := fakeError("invalid Pod")
	ip.On("State", op, podID, podName).Return("", fakeErr)
	ip.On("EpAddresses", op, podID, podName).Return(nil, fakeErr)

	// Error case
	status, err := s.GetStatus(op, podID, podName, HostAddress)
	assert.Check(t, err, "Expected nil")
	assert.Check(t, is.Equal(status.Phase, v1.PodUnknown), "Expected Phase PodUnknown")
	verifyConditions(t, status.Conditions, v1.ConditionUnknown, v1.ConditionUnknown, v1.ConditionUnknown)
	assert.Check(t, is.Equal(status.HostIP, "0.0.0.0"), "Expected Host IP Address")
	assert.Check(t, is.Equal(status.PodIP, "0.0.0.0"), "Expected Pod IP Address")
}

func verifyConditions(t *testing.T, conditions []v1.PodCondition, scheduled v1.ConditionStatus, initialized v1.ConditionStatus, ready v1.ConditionStatus) {
	for _, condition := range conditions {
		switch condition.Type {
		case v1.PodScheduled:
			assert.Check(t, is.Equal(condition.Status, scheduled), "Condition Pod Scheduled")
			break
		case v1.PodInitialized:
			assert.Check(t, is.Equal(condition.Status, initialized), "Condition Pod Initialized")
			break
		case v1.PodReady:
			assert.Check(t, is.Equal(condition.Status, ready), "Condition Pod Ready")
			break
		}
	}
}
