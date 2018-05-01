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
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/vmware/vic/lib/apiservers/portlayer/client"
	"github.com/vmware/vic/pkg/trace"

	"github.com/virtual-kubelet/virtual-kubelet/providers/vic/proxy/mocks"
)

func TestNewPodStarter(t *testing.T) {
	var s PodStarter
	var err error

	client := client.Default
	ip := &mocks.IsolationProxy{}

	// Positive Cases
	s, err = NewPodStarter(client, ip)
	assert.NotNil(t, s, "Expected non-nil creating a pod starter but received nil")

	// Negative Cases
	s, err = NewPodStarter(nil, ip)
	assert.Nil(t, s, "Expected nil")
	assert.Equal(t, err, PodStarterPortlayerClientError)

	s, err = NewPodStarter(client, nil)
	assert.Nil(t, s, "Expected nil")
	assert.Equal(t, err, PodStarterIsolationProxyError)
}

func TestStartPod_BadArgs(t *testing.T) {
	client := client.Default
	ip := &mocks.IsolationProxy{}
	op := trace.NewOperation(context.Background(), "")

	// Start with arguments
	s, err := NewPodStarter(client, ip)
	assert.NotNil(t, s, "Expected non-nil creating a pod starter but received nil")

	// Negative Cases
	err = s.Start(op, "", podName)
	assert.Equal(t, err, PodStarterInvalidPodIDError)

	err = s.Start(op, podID, "")
	assert.Equal(t, err, PodStarterInvalidPodNameError)
}

//NOTE: The rest of PodStarter tests were handled in PocCreator's tests so there's no need for further tests.