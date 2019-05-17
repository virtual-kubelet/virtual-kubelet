// +build e2e

package e2e

import (
	"flag"
	"os"
	"testing"

	v1 "k8s.io/api/core/v1"

	"github.com/virtual-kubelet/virtual-kubelet/test/e2e/framework"
)

const (
	defaultNamespace = v1.NamespaceDefault
	defaultNodeName  = "vkubelet-mock-0"
)

var (
	// f is the testing framework used for running the test suite.
	f *framework.Framework

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

func TestMain(m *testing.M) {
	// Set sane defaults in case no values (or empty ones) have been provided.
	setDefaults()
	// Create a new instance of the test framework targeting the specified node.
	f = framework.NewTestingFramework(kubeconfig, namespace, nodeName)
	// Wait for the virtual-kubelet pod to be ready.
	if _, err := f.WaitUntilPodReady(namespace, nodeName); err != nil {
		panic(err)
	}
	// Run the test suite.
	os.Exit(m.Run())
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
