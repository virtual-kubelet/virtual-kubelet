//go:build e2e
// +build e2e

package e2e

import (
	"flag"
	"fmt"
	"testing"

	vke2e "github.com/virtual-kubelet/virtual-kubelet/test/e2e"

	v1 "k8s.io/api/core/v1"
)

const (
	defaultNamespace = v1.NamespaceDefault
	defaultNodeName  = "vkubelet-mock-0"
)

var (
	kubeconfig string
	namespace  string
	nodeName   string
)

// go1.13 compatibility cf. https://github.com/golang/go/issues/31859
var _ = func() bool {
	testing.Init()
	return true
}()

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "path to the kubeconfig file to use when running the test suite outside a kubernetes cluster")
	flag.StringVar(&namespace, "namespace", defaultNamespace, "the name of the kubernetes namespace to use for running the test suite (i.e. where to create pods)")
	flag.StringVar(&nodeName, "node-name", defaultNodeName, "the name of the virtual-kubelet node to test")
	flag.Parse()
}

// Provider-specific setup function
func setup() error {
	fmt.Println("Setting up end-to-end test suite for mock provider...")
	return nil
}

// Provider-specific teardown function
func teardown() error {
	fmt.Println("Tearing down end-to-end test suite for mock provider...")
	return nil
}

// Provider-specific shouldSkipTest function
func shouldSkipTest(testName string) bool {
	return false
}

// TestEndToEnd creates and runs the end-to-end test suite for virtual kubelet
func TestEndToEnd(t *testing.T) {
	setDefaults()
	config := vke2e.EndToEndTestSuiteConfig{
		Kubeconfig:     kubeconfig,
		Namespace:      namespace,
		NodeName:       nodeName,
		Setup:          setup,
		Teardown:       teardown,
		ShouldSkipTest: shouldSkipTest,
	}
	ts := vke2e.NewEndToEndTestSuite(config)
	ts.Run(t)
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
