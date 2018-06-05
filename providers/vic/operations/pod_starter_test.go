package operations

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/virtual-kubelet/virtual-kubelet/providers/vic/proxy/mocks"
	"github.com/vmware/vic/lib/apiservers/portlayer/client"
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

//NOTE: The rest of PodStarter tests were handled in PodCreator's tests so there's no need for further tests.
