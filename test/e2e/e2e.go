// +build e2e

package e2e

import (
	"github.com/virtual-kubelet/virtual-kubelet/test/e2e/framework"
)

// f is a testing framework that is accessible across the e2e package
var f *framework.Framework

// SetUpFunc sets up provider-specific resource in the test suite
type SetUpFunc func() error

// TeardownFunc tears down provider-specific resources from the test suite
type TeardownFunc func() error

// ShouldSkipTestFunc is a function to determine whether the test suite should skip a particular test
type ShouldSkipTestFunc func(string) bool

// EndToEndTestSuite holds the setup, teardown, and shouldSkipTest functions for a specific provider
type EndToEndTestSuite struct {
	setup          SetUpFunc
	teardown       TeardownFunc
	shouldSkipTest ShouldSkipTestFunc
}

// EndToEndTestSuiteConfig is the config passed to initialize the testing framework and test suite.
type EndToEndTestSuiteConfig struct {
	Kubeconfig     string
	Namespace      string
	NodeName       string
	Setup          SetUpFunc
	Teardown       TeardownFunc
	ShouldSkipTest ShouldSkipTestFunc
}

// Setup runs the setup function from the provider and other
// procedures before running the test suite
func (ts *EndToEndTestSuite) Setup() {
	if err := ts.setup(); err != nil {
		panic("Error in Setup()")
	}

	// Wait for the virtual kubelet (deployed as a pod) to become fully ready
	if _, err := f.WaitUntilPodReady(f.Namespace, f.NodeName); err != nil {
		panic(err)
	}
}

// Teardown runs the teardown function from the provider and other
// procedures after running the test suite
func (ts *EndToEndTestSuite) Teardown() {
	if err := ts.teardown(); err != nil {
		panic("Error in Teardown()")
	}
}

// ShouldSkipTest returns true if a provider wants to skip running a particular test
func (ts *EndToEndTestSuite) ShouldSkipTest(testName string) bool {
	return ts.shouldSkipTest(testName)
}

// NewEndToEndTestSuite returns a new EndToEndTestSuite given a test suite configuration,
// setup, and teardown functions from provider
func NewEndToEndTestSuite(cfg EndToEndTestSuiteConfig) *EndToEndTestSuite {
	if cfg.Namespace == "" {
		panic("Empty namespace")
	} else if cfg.NodeName == "" {
		panic("Empty node name")
	}

	f = framework.NewTestingFramework(cfg.Kubeconfig, cfg.Namespace, cfg.NodeName)

	emptyFunc := func() error { return nil }
	if cfg.Setup == nil {
		cfg.Setup = emptyFunc
	}
	if cfg.Teardown == nil {
		cfg.Teardown = emptyFunc
	}
	if cfg.ShouldSkipTest == nil {
		// This will not skip any test in the test suite
		cfg.ShouldSkipTest = func(_ string) bool { return false }
	}

	return &EndToEndTestSuite{
		setup:          cfg.Setup,
		teardown:       cfg.Teardown,
		shouldSkipTest: cfg.ShouldSkipTest,
	}
}
