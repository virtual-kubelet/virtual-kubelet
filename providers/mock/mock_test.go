package mock

import (
	"testing"

	"github.com/virtual-kubelet/virtual-kubelet/vkubelet"
	"github.com/virtual-kubelet/virtual-kubelet/providers"
	"gotest.tools/assert"
)

// We can guarantee the right interfaces are implemented inside of by putting casts in place. We must do the verification
// that a given type *does not* implement a given interface in this test.
func TestMockLegacyInterface(t *testing.T) {
	var mlp providers.Provider = &MockLegacyProvider{}
	_, ok := mlp.(vkubelet.PodNotifier)
	assert.Assert(t, !ok)
}
