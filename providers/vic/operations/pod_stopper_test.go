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
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/vmware/vic/lib/apiservers/portlayer/client"
)

func TestNewPodStopper(t *testing.T) {
	_, ip, _, _ := createMocks(t)
	client := client.Default

	// Positive Cases
	s, err := NewPodStopper(client, ip)
	assert.NotNil(t, s, "Expected non-nil creating a pod Stopper but received nil")

	// Negative Cases
	s, err = NewPodStopper(nil, ip)
	assert.Nil(t, s, "Expected nil")
	assert.Equal(t, err, PodStopperPortlayerClientError)

	s, err = NewPodStopper(client, nil)
	assert.Nil(t, s, "Expected nil")
	assert.Equal(t, err, PodStopperIsolationProxyError)
}

func TestStopPod(t *testing.T) {
	client := client.Default
	_, ip, _, op := createMocks(t)

	// Start with arguments
	s, err := NewPodStopper(client, ip)
	assert.NotNil(t, s, "Expected non-nil creating a pod Stopper but received nil")
	assert.Nil(t, err, "Expected nil")

	// Set up the mocks for this test
	ip.On("Handle", op, podID, podName).Return(podHandle, nil)
	ip.On("UnbindScope", op, podHandle, podName).Return(podHandle, fakeEP, nil)
	ip.On("SetState", op, podHandle, podName, "STOPPED").Return(podHandle, nil)
	ip.On("CommitHandle", op, podHandle, podID, int32(-1)).Return(nil)

	// Positive case
	err = s.Stop(op, podID, podName)
	assert.Nil(t, err, "Expected nil")
}

func TestStopPodErrorHandle(t *testing.T) {
	client := client.Default
	_, ip, _, op := createMocks(t)

	// Start with arguments
	s, err := NewPodStopper(client, ip)
	assert.NotNil(t, s, "Expected non-nil creating a pod Stopper but received nil")
	assert.Nil(t, err, "Expected nil")

	// Set up the mocks for this test
	ip.On("UnbindScope", op, podHandle, podName).Return(podHandle, fakeEP, nil)
	ip.On("SetState", op, podHandle, podName, "STOPPED").Return(podHandle, nil)
	ip.On("CommitHandle", op, podHandle, podID, int32(-1)).Return(nil)

	// Failed Handle
	fakeErr := fakeError("invalid handle")
	ip.On("Handle", op, podID, podName).Return("", fakeErr)

	err = s.Stop(op, podID, podName)
	assert.Equal(t, err, fakeErr, "Expected invalid handle error")
}

func TestStopPodErrorUnbindScope(t *testing.T) {
	client := client.Default
	_, ip, _, op := createMocks(t)

	// Start with arguments
	s, err := NewPodStopper(client, ip)
	assert.NotNil(t, s, "Expected non-nil creating a pod Stopper but received nil")
	assert.Nil(t, err, "Expected nil")

	// Set up the mocks for this test
	ip.On("Handle", op, podID, podName).Return(podHandle, nil)
	ip.On("SetState", op, podHandle, podName, "STOPPED").Return(podHandle, nil)
	ip.On("CommitHandle", op, podHandle, podID, int32(-1)).Return(nil)

	// Failed UnbindScope
	fakeErr := fakeError("failed UnbindScope")
	ip.On("UnbindScope", op, podHandle, podName).Return("", nil, fakeErr)

	err = s.Stop(op, podID, podName)
	assert.Equal(t, err, fakeErr, "Expected failed UnbindScope error")
}

func TestStopPodErrorSetState(t *testing.T) {
	client := client.Default
	_, ip, _, op := createMocks(t)

	// Start with arguments
	s, err := NewPodStopper(client, ip)
	assert.NotNil(t, s, "Expected non-nil creating a pod Stopper but received nil")
	assert.Nil(t, err, "Expected nil")

	// Set up the mocks for this test
	ip.On("Handle", op, podID, podName).Return(podHandle, nil)
	ip.On("UnbindScope", op, podHandle, podName).Return(podHandle, fakeEP, nil)
	ip.On("CommitHandle", op, podHandle, podID, int32(-1)).Return(nil)

	// Failed SetState
	fakeErr := fakeError("failed SetState")
	ip.On("SetState", op, podHandle, podName, "STOPPED").Return("", fakeErr)
	err = s.Stop(op, podID, podName)
	assert.Equal(t, err, fakeErr, "Expected failed SetState error")
}

func TestStopPodErrorCommit(t *testing.T) {
	client := client.Default
	_, ip, _, op := createMocks(t)

	// Start with arguments
	s, err := NewPodStopper(client, ip)
	assert.NotNil(t, s, "Expected non-nil creating a pod Stopper but received nil")
	assert.Nil(t, err, "Expected nil")

	// Set up the mocks for this test
	ip.On("Handle", op, podID, podName).Return(podHandle, nil)
	ip.On("UnbindScope", op, podHandle, podName).Return(podHandle, fakeEP, nil)
	ip.On("SetState", op, podHandle, podName, "STOPPED").Return(podHandle, nil)

	// Failed Commit
	fakeErr := fakeError("failed Commit")
	ip.On("CommitHandle", op, podHandle, podID, int32(-1)).Return(fakeErr)
	err = s.Stop(op, podID, podName)
	assert.Equal(t, err, fakeErr ,"Expected failed Commit error")
}