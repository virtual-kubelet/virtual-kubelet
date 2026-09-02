package nodeutil

import (
	"errors"
	"testing"

	"github.com/virtual-kubelet/virtual-kubelet/node"
	"gotest.tools/assert"
)

// TestWithPodControllerConfigOverrides verifies that overrides registered via the option are
// appended to the NodeConfig and, when applied the way NewNode applies them, run in registration
// order and mutate the PodControllerConfig.
func TestWithPodControllerConfigOverrides(t *testing.T) {
	var order []string

	var cfg NodeConfig

	// Registering across multiple calls should accumulate, not replace.
	assert.NilError(t, WithPodControllerConfigOverrides(
		func(c *node.PodControllerConfig) error {
			order = append(order, "first")
			c.SkipDownwardAPIResolution = true
			return nil
		},
	)(&cfg))
	assert.NilError(t, WithPodControllerConfigOverrides(
		func(c *node.PodControllerConfig) error {
			order = append(order, "second")
			return nil
		},
	)(&cfg))
	assert.Equal(t, len(cfg.PodControllerConfigOpts), 2)

	// Apply them the same way NewNode does.
	pcc := node.PodControllerConfig{}
	for _, o := range cfg.PodControllerConfigOpts {
		assert.NilError(t, o(&pcc))
	}

	assert.Equal(t, len(order), 2)
	assert.Equal(t, order[0], "first")
	assert.Equal(t, order[1], "second")
	assert.Equal(t, pcc.SkipDownwardAPIResolution, true)
}

// TestWithPodControllerConfigOverridesError verifies an override's error is surfaced when applied.
func TestWithPodControllerConfigOverridesError(t *testing.T) {
	boom := errors.New("boom")

	var cfg NodeConfig
	assert.NilError(t, WithPodControllerConfigOverrides(
		func(*node.PodControllerConfig) error { return boom },
	)(&cfg))

	err := cfg.PodControllerConfigOpts[0](&node.PodControllerConfig{})
	assert.Equal(t, err, boom)
}
