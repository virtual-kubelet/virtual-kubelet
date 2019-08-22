package e2e

import (
	"github.com/chewong/virtual-kubelet/test/e2e/framework"
)

var f *framework.Framework

// TestingSuite holds the setup and teardown functions for a specific provider
type TestingSuite struct {
	// setupProvider is a function that setup
	setupProvider func() error
	// teardownProvider is a function
	teardownProvider func() error
}

// TestingSuiteConfig is the config passed to initialize the testing framework.
type TestingSuiteConfig struct {
	// Kubeconfig is the path to the kubeconfig file to use when running the test suite outside a Kubernetes cluster.
	Kubeconfig string
	// Namespace is the name of the Kubernetes namespace to use for running the test suite (i.e. where to create pods).
	Namespace string
	// NodeName is the name of the virtual-kubelet node to test.
	NodeName string
}

// Setup runs the setup function from the provider and other
// procedures before running the testing suite
func (ts *TestingSuite) Setup() {
	if err := ts.setupProvider(); err != nil {
		panic("Error in Setup()")
	}

	// Wait for the virtual kubelet (deployed as a pod) to become fully ready
	if _, err := f.WaitUntilPodReady(f.Namespace, f.NodeName); err != nil {
		panic(err)
	}
}

// Teardown runs the teardown function from the provider and other
// procedures after running the testing suite
func (ts *TestingSuite) Teardown() {
	if err := ts.teardownProvider(); err != nil {
		panic("Error in Teardown()")
	}
}

// NewTestingSuite returns a new TestingSuite given a testing suite configuration,
// setup, and teardown functions from provider
func NewTestingSuite(cfg TestingSuiteConfig, setupProvider, teardownProvider func() error) *TestingSuite {
	if cfg.Namespace == "" {
		panic("Empty namespace")
	} else if cfg.NodeName == "" {
		panic("Empty node name")
	}

	// f is accessible across the e2e package
	f = framework.NewTestingFramework(cfg.Kubeconfig, cfg.Namespace, cfg.NodeName)

	return &TestingSuite{
		setupProvider:    setupProvider,
		teardownProvider: teardownProvider,
	}
}
