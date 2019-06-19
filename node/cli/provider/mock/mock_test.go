package mock

// We can guarantee the right interfaces are implemented inside of by putting casts in place. We must do the verification
// that a given type *does not* implement a given interface in this test.
// Cannot implement this due to:  https://github.com/virtual-kubelet/virtual-kubelet/issues/632
/*
func TestMockLegacyInterface(t *testing.T) {
	var mlp providers.Provider = &MockLegacyProvider{}
	_, ok := mlp.(node.PodNotifier)
	assert.Assert(t, !ok)
}
*/
