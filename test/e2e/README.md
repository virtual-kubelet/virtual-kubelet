# Importable End-To-End Test Suite

Virtual Kubelet (VK) provides an importable end-to-end (E2E) test suite containing a set of common integration tests. As a provider, you can import the test suite and use it to validate your VK implementation.

## Prerequisite

To run the E2E test suite, three things are required:

- A local Kubernetes cluster (we have tested with [Docker for Mac](https://docs.docker.com/docker-for-mac/install/) and [Minikube](https://kubernetes.io/docs/tasks/tools/install-minikube/));
- Your _kubeconfig_ default context points to the local Kubernetes cluster;
- [skaffold](https://skaffold.dev/docs/getting-started/#installing-skaffold)

> The test suite is based on [VK 1.0](https://github.com/virtual-kubelet/virtual-kubelet/releases/tag/v1.0.0). If your VK implementation is based on legacy VK library (< v1.0.0), you will have to upgrade it to VK 1.0 using [virtual-kubelet/node-cli](https://github.com/virtual-kubelet/node-cli).

### Skaffold Folder

Before running the E2E test suite, you will need to copy the [`./hack`](../../hack) folder containing Skaffold-related files such as Dockerfile, manifests, and certificates to your VK project root. Skaffold essentially helps package your virtual kubelet into a container based on the given [`Dockerfile`](../../hack/skaffold/virtual-kubelet/Dockerfile) and deploy it as a pod (see [`pod.yml`](../../hack/skaffold/virtual-kubelet/pod.yml)) to your Kubernetes test cluster. In summary, you will likely need to modify the VK name in those files, customize the VK configuration file, and the API server certificates (`<vk-name>-crt.pem` and `<vk-name>-key.pem`) before running the test suite.

### Makefile.e2e

Also, you will need to copy [`Makefile.e2e`](../../Makefile.e2e) to your VK project root. It contains necessary `make` commands to run the E2E test suite. Do not forget to add `include Makefile.e2e` in your `Makefile`.

### File Structure

A minimal VK provider should now have a file structure similar to the one below:

```console
.
├── Makefile
├── Makefile.e2e
├── README.md
├── cmd
│   └── virtual-kubelet
│       └── main.go
├── go.mod
├── go.sum
├── hack
│   └── skaffold
│       └── virtual-kubelet
│           ├── Dockerfile
│           ├── base.yml
│           ├── pod.yml
│           ├── skaffold.yml
│           ├── vkubelet-provider-0-cfg.json
│           ├── vkubelet-provider-0-crt.pem
│           └── vkubelet-provider-0-key.pem
├── test
│   └── e2e
│       └── main_test.go # import and run the E2E test suite here
├── provider.go # provider-specific VK implementation
├── provider_test.go # unit test
```

## Importing the Test Suite

The test suite can be easily imported in your test files (e.g. `./test/e2e/main_test.go`) with the following import statement:
```go
import (
	vke2e "github.com/virtual-kubelet/virtual-kubelet/test/e2e"
)
```

### Test Suite Customization

The test suite allows providers to customize the test suite using `EndToEndTestSuiteConfig`:

```go
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
```

> `Setup()` is invoked before running the E2E test suite, and `Teardown()` is invoked after all the E2E tests are finished.

You will need an `EndToEndTestSuiteConfig` to create an `EndToEndTestSuite` using `NewEndToEndTestSuite`. After that, invoke `Run` from `EndToEndTestSuite` to start the test suite. The code snippet below is a minimal example of how to import and run the test suite in your test file.

```go
package e2e

import (
	"flag"
	"fmt"
	"testing"
	"time"

	vke2e "github.com/virtual-kubelet/virtual-kubelet/test/e2e"
)

var (
	kubeconfig string
	namespace  string
	nodeName   string
)

var defaultNamespace = "default"
var defaultNodeName = "default-node"

// Read the following variables from command-line flags
func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "path to the kubeconfig file to use when running the test suite outside a kubernetes cluster")
	flag.StringVar(&namespace, "namespace", defaultNamespace, "the name of the kubernetes namespace to use for running the test suite (i.e. where to create pods)")
	flag.StringVar(&nodeName, "node-name", defaultNodeName, "the name of the virtual-kubelet node to test")
	flag.Parse()
}

func setup() error {
	fmt.Println("Setting up end-to-end test suite...")
	return nil
}

func teardown() error {
	fmt.Println("Tearing down end-to-end test suite...")
	return nil
}

func shouldSkipTest(testName string) bool {
	// Skip the test 'TestGetStatsSummary'
	return testName == "TestGetStatsSummary"
}

// TestEndToEnd runs the e2e tests against a previously configured cluster
func TestEndToEnd(t *testing.T) {
	config := vke2e.EndToEndTestSuiteConfig{
		Kubeconfig:     kubeconfig,
		Namespace:      namespace,
		NodeName:       nodeName,
		Setup:          setup,
		Teardown:       teardown,
		ShouldSkipTest: shouldSkipTest,
		WatchTimeout:   5 * time.Minute,
	}
	ts := vke2e.NewEndToEndTestSuite(config)
	ts.Run(t)
}
```

## Running the Test Suite

Since our CI uses Minikube, we describe below how to run E2E on top of it.

To create a Minikube cluster, run the following command after [installing Minikube](https://github.com/kubernetes/minikube#installation):

```bash
minikube start
```

To run the E2E test suite, you can run the following command:

```bash
make e2e
```

You can see from the console output whether the tests in the test suite pass or not.

```console
...
=== RUN   TestEndToEnd
Setting up end-to-end test suite for mock provider...
    suite.go:62: True
=== RUN   TestEndToEnd/TestCreatePodWithMandatoryInexistentConfigMap
=== RUN   TestEndToEnd/TestCreatePodWithMandatoryInexistentSecrets
=== RUN   TestEndToEnd/TestCreatePodWithOptionalInexistentConfigMap
=== RUN   TestEndToEnd/TestCreatePodWithOptionalInexistentSecrets
=== RUN   TestEndToEnd/TestGetPods
    basic.go:40: Created pod: nginx-testgetpods-g9s42
    basic.go:46: Pod nginx-testgetpods-g9s42 ready
=== RUN   TestEndToEnd/TestGetStatsSummary
=== RUN   TestEndToEnd/TestNodeCreateAfterDelete
=== RUN   TestEndToEnd/TestPodLifecycleForceDelete
    basic.go:208: Created pod: nginx-testpodlifecycleforcedelete-wrjgk
    basic.go:214: Pod nginx-testpodlifecycleforcedelete-wrjgk ready
    basic.go:247: Force deleted pod:  nginx-testpodlifecycleforcedelete-wrjgk
    basic.go:264: Pod ended as phase: Running
=== RUN   TestEndToEnd/TestPodLifecycleGracefulDelete
    basic.go:135: Created pod: nginx-testpodlifecyclegracefuldelete-tp49x
    basic.go:141: Pod nginx-testpodlifecyclegracefuldelete-tp49x ready
    basic.go:168: Deleted pod: nginx-testpodlifecyclegracefuldelete-tp49x
Tearing down end-to-end test suite for mock provider...
--- PASS: TestEndToEnd (11.75s)
    --- PASS: TestEndToEnd/TestCreatePodWithMandatoryInexistentConfigMap (0.04s)
    --- PASS: TestEndToEnd/TestCreatePodWithMandatoryInexistentSecrets (0.03s)
    --- PASS: TestEndToEnd/TestCreatePodWithOptionalInexistentConfigMap (0.73s)
    --- PASS: TestEndToEnd/TestCreatePodWithOptionalInexistentSecrets (1.00s)
    --- PASS: TestEndToEnd/TestGetPods (0.80s)
    --- PASS: TestEndToEnd/TestGetStatsSummary (0.80s)
    --- PASS: TestEndToEnd/TestNodeCreateAfterDelete (5.25s)
    --- PASS: TestEndToEnd/TestPodLifecycleForceDelete (2.05s)
    --- PASS: TestEndToEnd/TestPodLifecycleGracefulDelete (1.05s)
PASS
ok      github.com/virtual-kubelet/virtual-kubelet/internal/test/e2e    12.298s
?       github.com/virtual-kubelet/virtual-kubelet/internal/test/e2e/framework  [no test files]
...
```
