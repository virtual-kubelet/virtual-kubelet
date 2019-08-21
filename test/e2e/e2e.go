package e2e

import (
	"github.com/chewong/virtual-kubelet/test/e2e/framework"
)

var f *framework.Framework

// TestingSuite is
type TestingSuite struct {
	// setupProvider is a function that setup
	setupProvider func() error
	// teardownProvider is a function
	teardownProvider func() error
}

// TestingSuiteConfig is
type TestingSuiteConfig struct {
	// kubeconfig is the path to the kubeconfig file to use when running the test suite outside a Kubernetes cluster.
	Kubeconfig string
	// namespace is the name of the Kubernetes namespace to use for running the test suite (i.e. where to create pods).
	Namespace string
	// nodeName is the name of the virtual-kubelet node to test.
	NodeName string
}

// Setup is
func (ts *TestingSuite) Setup() {
	if err := ts.setupProvider(); err != nil {
		panic("Error in Setup()")
	}

	if _, err := f.WaitUntilPodReady(f.Namespace, f.NodeName); err != nil {
		panic(err)
	}
}

// Teardown is
func (ts *TestingSuite) Teardown() {
	if err := ts.teardownProvider(); err != nil {
		panic("Error in Teardown()")
	}
}

// NewTestingSuite is
func NewTestingSuite(cfg TestingSuiteConfig, setupProvider, teardownProvider func() error) *TestingSuite {
	if cfg.Kubeconfig == "" {
		panic("Empty kubeconfig")
	} else if cfg.Namespace == "" {
		panic("Empty namespace")
	} else if cfg.NodeName == "" {
		panic("Empty nodeName")
	}

	// f is accessible across the e2e package
	f = framework.NewTestingFramework(cfg.Kubeconfig, cfg.Namespace, cfg.NodeName)

	return &TestingSuite{
		setupProvider:    setupProvider,
		teardownProvider: teardownProvider,
	}
}
