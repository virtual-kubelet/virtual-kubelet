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

func setup() error {
	// Implement provider-specific setup function
	return nil
}

func teardown() error {
	// Implement provider-specific teardown function
	return nil
}

// TestEndToEnd creates and runs the end-to-end test suite for virtual kubelet
func TestEndToEnd(t *testing.T) {
	setDefaults()

	config := EndToEndTestSuiteConfig{
		Kubeconfig: kubeconfig,
		Namespace:  namespace,
		NodeName:   nodeName,
	}
	ts := NewEndToEndTestSuite(config, setup, teardown)

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
