package e2e

import (
	"github.com/virtual-kubelet/virtual-kubelet/test/e2e/framework"
)

var f *framework.Framework

// EndToEndTestSuite holds the setup and teardown functions for a specific provider
type EndToEndTestSuite struct {
	// setupProvider is a function that setup
	setupProvider func() error
	// teardownProvider is a function
	teardownProvider func() error
}

// EndToEndTestSuiteConfig is the config passed to initialize the testing framework.
type EndToEndTestSuiteConfig struct {
	// Kubeconfig is the path to the kubeconfig file to use when running the test suite outside a Kubernetes cluster.
	Kubeconfig string
	// Namespace is the name of the Kubernetes namespace to use for running the test suite (i.e. where to create pods).
	Namespace string
	// NodeName is the name of the virtual-kubelet node to test.
	NodeName string
}

// Setup runs the setup function from the provider and other
// procedures before running the test suite
func (ts *EndToEndTestSuite) Setup() {
	if err := ts.setupProvider(); err != nil {
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
	if err := ts.teardownProvider(); err != nil {
		panic("Error in Teardown()")
	}
}

// NewEndToEndTestSuite returns a new EndToEndTestSuite given a test suite configuration,
// setup, and teardown functions from provider
func NewEndToEndTestSuite(cfg EndToEndTestSuiteConfig, setupProvider, teardownProvider func() error) *EndToEndTestSuite {
	if cfg.Namespace == "" {
		panic("Empty namespace")
	} else if cfg.NodeName == "" {
		panic("Empty node name")
	}

	// f is accessible across the e2e package
	f = framework.NewTestingFramework(cfg.Kubeconfig, cfg.Namespace, cfg.NodeName)

	return &EndToEndTestSuite{
		setupProvider:    setupProvider,
		teardownProvider: teardownProvider,
	}
}
