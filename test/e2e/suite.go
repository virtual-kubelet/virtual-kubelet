package e2e

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/virtual-kubelet/virtual-kubelet/internal/test/e2e/framework"
	"github.com/virtual-kubelet/virtual-kubelet/internal/test/suite"
)

const defaultWatchTimeout = 2 * time.Minute

// f is a testing framework that is accessible across the e2e package
var f *framework.Framework

// EndToEndTestSuite holds the setup, teardown, and shouldSkipTest functions for a specific provider
type EndToEndTestSuite struct {
	setup          suite.SetUpFunc
	teardown       suite.TeardownFunc
	shouldSkipTest suite.ShouldSkipTestFunc
}

// EndToEndTestSuiteConfig is the config passed to initialize the testing framework and test suite.
type EndToEndTestSuiteConfig struct {
	// Kubeconfig is the path to the kubeconfig file to use when running the test suite outside a Kubernetes cluster.
	Kubeconfig string
	// Namespace is the name of the Kubernetes namespace to use for running the test suite (i.e. where to create pods).
	Namespace string
	// NodeName is the name of the virtual-kubelet node to test.
	NodeName string
	// WatchTimeout is the duration for which the framework watch a particular condition to be satisfied (e.g. watches a pod  becoming ready)
	WatchTimeout time.Duration
	// Setup is a function that sets up provider-specific resource in the test suite
	Setup suite.SetUpFunc
	// Teardown is a function that tears down provider-specific resources from the test suite
	Teardown suite.TeardownFunc
	// ShouldSkipTest is a function that determines whether the test suite should skip certain tests
	ShouldSkipTest suite.ShouldSkipTestFunc
}

// Setup runs the setup function from the provider and other
// procedures before running the test suite
func (ts *EndToEndTestSuite) Setup(t *testing.T) {
	if err := ts.setup(); err != nil {
		t.Fatal(err)
	}

	// Wait for the virtual kubelet node resource to become fully ready
	if err := f.WaitUntilNodeCondition(func(ev watch.Event) (bool, error) {
		n := ev.Object.(*corev1.Node)
		if n.Name != f.NodeName {
			return false, nil
		}

		for _, c := range n.Status.Conditions {
			if c.Type != "Ready" {
				continue
			}
			t.Log(c.Status)
			return c.Status == corev1.ConditionTrue, nil
		}

		return false, nil
	}); err != nil {
		t.Fatal(err)
	}
}

// Teardown runs the teardown function from the provider and other
// procedures after running the test suite
func (ts *EndToEndTestSuite) Teardown() {
	if err := ts.teardown(); err != nil {
		panic(err)
	}
}

// ShouldSkipTest returns true if a provider wants to skip running a particular test
func (ts *EndToEndTestSuite) ShouldSkipTest(testName string) bool {
	return ts.shouldSkipTest(testName)
}

// Run runs tests registered in the test suite
func (ts *EndToEndTestSuite) Run(t *testing.T) {
	suite.Run(t, ts)
}

// NewEndToEndTestSuite returns a new EndToEndTestSuite given a test suite configuration,
// setup, and teardown functions from provider
func NewEndToEndTestSuite(cfg EndToEndTestSuiteConfig) *EndToEndTestSuite {
	if cfg.Namespace == "" {
		panic("Empty namespace")
	} else if cfg.NodeName == "" {
		panic("Empty node name")
	}

	if cfg.WatchTimeout == time.Duration(0) {
		cfg.WatchTimeout = defaultWatchTimeout
	}

	f = framework.NewTestingFramework(cfg.Kubeconfig, cfg.Namespace, cfg.NodeName, cfg.WatchTimeout)

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
