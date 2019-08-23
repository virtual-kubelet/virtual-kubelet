// +build e2e

package e2e

import (
	"flag"
	"testing"

	"github.com/virtual-kubelet/virtual-kubelet/test/suite"
	v1 "k8s.io/api/core/v1"
)

const (
	defaultNamespace = v1.NamespaceDefault
	defaultNodeName  = "vkubelet-mock-0"
)

var (
	// kubeconfig is the path to the kubeconfig file to use when running the test suite outside a Kubernetes cluster.
	kubeconfig string
	// namespace is the name of the Kubernetes namespace to use for running the test suite (i.e. where to create pods).
	namespace string
	// nodeName is the name of the virtual-kubelet node to test.
	nodeName string
)

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "path to the kubeconfig file to use when running the test suite outside a kubernetes cluster")
	flag.StringVar(&namespace, "namespace", defaultNamespace, "the name of the kubernetes namespace to use for running the test suite (i.e. where to create pods)")
	flag.StringVar(&nodeName, "node-name", defaultNodeName, "the name of the virtual-kubelet node to test")
	flag.Parse()
}

// Provider-specific setup function
func setup() error {
	return nil
}

// Provider-specific teardown function
func teardown() error {
	return nil
}

//  Provider-specific shouldSkipTest function
func shouldSkipTest(testName string) bool {
	return false
}

// TestEndToEnd creates and runs the end-to-end test suite for virtual kubelet
func TestEndToEnd(t *testing.T) {
	setDefaults()

	config := EndToEndTestSuiteConfig{
		Kubeconfig:     kubeconfig,
		Namespace:      namespace,
		NodeName:       nodeName,
		Setup:          setup,
		Teardown:       teardown,
		ShouldSkipTest: shouldSkipTest,
	}
	ts := NewEndToEndTestSuite(config)

	suite.Run(t, ts)
}

// setDefaults sets sane defaults in case no values (or empty ones) have been provided.
func setDefaults() {
	if namespace == "" {
		namespace = defaultNamespace
	}
	if nodeName == "" {
		nodeName = defaultNodeName
	}
}
