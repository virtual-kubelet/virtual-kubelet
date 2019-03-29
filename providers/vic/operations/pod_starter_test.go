package operations

import (
	"testing"

	"github.com/virtual-kubelet/virtual-kubelet/providers/vic/proxy/mocks"
	"github.com/vmware/vic/lib/apiservers/portlayer/client"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
)

func TestNewPodStarter(t *testing.T) {
	var s PodStarter
	var err error

	client := client.Default
	ip := &mocks.IsolationProxy{}

	// Positive Cases
	s, err = NewPodStarter(client, ip)
	assert.Check(t, s != nil, "Expected non-nil creating a pod starter but received nil")

	// Negative Cases
	s, err = NewPodStarter(nil, ip)
	assert.Check(t, is.Nil(s), "Expected nil")
	assert.Check(t, is.DeepEqual(err, PodStarterPortlayerClientError))

	s, err = NewPodStarter(client, nil)
	assert.Check(t, is.Nil(s), "Expected nil")
	assert.Check(t, is.DeepEqual(err, PodStarterIsolationProxyError))
}

//NOTE: The rest of PodStarter tests were handled in PodCreator's tests so there's no need for further tests.
