# Importable End-To-End Test Suite

Virtual Kubelet (VK) provides an importable end-to-end (e2e) test suite containing a set of common integration tests. As a provider, you can import the test suite and use it to validate your VK implementation.

## Prerequisite

To run the e2e test suite, three things are required:

- A local Kubernetes cluster (we have tested with [Docker for Mac](https://docs.docker.com/docker-for-mac/install/) and [Minikube](https://kubernetes.io/docs/tasks/tools/install-minikube/));
- Your _kubeconfig_ default context points to the local Kubernetes cluster;
- [skaffold](https://skaffold.dev/docs/getting-started/#installing-skaffold);

Note that the test suite is based on [VK 1.0](https://github.com/virtual-kubelet/virtual-kubelet/releases/tag/v1.0.0). If your VK implementation is based on legacy VK library (< v1.0.0), you will have to upgrade it to VK 1.0 using [virtual-kubelet/node-cli](https://github.com/virtual-kubelet/node-cli).

### Skaffold Folder

Before running the e2e test suite, you will need to copy the [`./hack`](../../hack) folder containing Skaffold-related files such as Dockerfile, manifests, and certificates to your VK project root. Skaffold essentially helps package your virtual kubelet into a container based on the given [`Dockerfile`](../../hack/skaffold/virtual-kubelet/Dockerfile) and deploy it as a pod (see [`pod.yml`](../../hack/skaffold/virtual-kubelet/pod.yml)) to your Kubernetes test cluster. In summary, you will need to modify the VK name in those files, the VK configuration file, and your API server certificates (`<vk-name>-crt.pem` and `<vk-name>-key.pem`) to suit your particular provider.

### Makefile.e2e

Also, you will need to copy [`Makefile.e2e`](../../Makefile.e2e) to your VK project root. It contains necessary `make` commands to run the e2e test suite. Do not forget to add `include Makefile.e2e` in your `Makefile`.

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
│       └── main_test.go # import and run the e2e test suite here
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

The test suite allows providers to specify custom logic for setting up and tearing down the test suite. `Setup()` is invoked before running the e2e test suite, and `Teardown()` is invoked after all the e2e tests are finished. The test suite also allows providers to skip certain tests of their choices.

The following interfaces describe the method signatures of `Setup()`, `Teardown()`, and `ShouldSkipTest()`

```go
// TestSuite contains methods that defines the lifecycle of a test suite
type TestSuite interface {
	Setup()
	Teardown()
}

// TestSkipper allows providers to skip certain tests
type TestSkipper interface {
	ShouldSkipTest(testName string) bool
}
```

The customizations above can be specified in `EndToEndTestSuiteConfig`. In summary, you will need to initialize a `EndToEndTestSuiteConfig` struct to specify various parameters. After that, you will need it to create a `EndToEndTestSuite` using `NewEndToEndTestSuite`. Finally, invoke `Run` from `EndToEndTestSuite` to start the test suite. The code snippet below is a minimal example of how to import and run the test suite in your test file.

```go
package e2e

import (
	"time"

	vke2e "github.com/virtual-kubelet/virtual-kubelet/test/e2e"
)

var (
	kubeconfig string
	namespace string
	nodeName string
)

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

func TestEndToEnd(t *testing.T) {
	config := vke2e.EndToEndTestSuiteConfig{
		Kubeconfig:     kubeconfig,
		Namespace:      namespace,
		NodeName:       nodeName,
		Setup:          setup,
		Teardown:       teardown,
		ShouldSkipTest: shouldSkipTest,
		WaitTimeout:    5 * time.Minute,
	}
	ts := vke2e.NewEndToEndTestSuite(config)
	ts.Run(t)
}
```

## Running the Test Suite

Since our CI uses Minikube, we describe below how to run e2e on top of it.

To create a Minikube cluster, run the following command after [installing Minikube](https://github.com/kubernetes/minikube#installation):

```bash
minikube start
```

To run the e2e test suite, you can run the following command:

```bash
make e2e
```

You can see from the console output whether the tests in the test suite pass or not.

```console
...
=== RUN   TestEndToEnd
=== RUN   TestEndToEnd/TestCreatePodWithMandatoryInexistentConfigMap
=== RUN   TestEndToEnd/TestCreatePodWithMandatoryInexistentSecrets
=== RUN   TestEndToEnd/TestCreatePodWithOptionalInexistentConfigMap
=== RUN   TestEndToEnd/TestCreatePodWithOptionalInexistentSecrets
=== RUN   TestEndToEnd/TestGetStatsSummary
=== RUN   TestEndToEnd/TestNodeCreateAfterDelete
=== RUN   TestEndToEnd/TestPodLifecycleForceDelete
=== RUN   TestEndToEnd/TestPodLifecycleGracefulDelete
--- PASS: TestEndToEnd (21.93s)
    --- PASS: TestEndToEnd/TestCreatePodWithMandatoryInexistentConfigMap (0.03s)
    --- PASS: TestEndToEnd/TestCreatePodWithMandatoryInexistentSecrets (0.03s)
    --- PASS: TestEndToEnd/TestCreatePodWithOptionalInexistentConfigMap (0.55s)
    --- PASS: TestEndToEnd/TestCreatePodWithOptionalInexistentSecrets (0.99s)
    --- PASS: TestEndToEnd/TestGetStatsSummary (0.80s)
    --- PASS: TestEndToEnd/TestNodeCreateAfterDelete (9.63s)
    --- PASS: TestEndToEnd/TestPodLifecycleForceDelete (2.05s)
        basic.go:158: Created pod: nginx-testpodlifecycleforcedelete-jz84g
        basic.go:164: Pod nginx-testpodlifecycleforcedelete-jz84g ready
        basic.go:197: Force deleted pod:  nginx-testpodlifecycleforcedelete-jz84g
        basic.go:214: Pod ended as phase: Running
    --- PASS: TestEndToEnd/TestPodLifecycleGracefulDelete (1.04s)
        basic.go:87: Created pod: nginx-testpodlifecyclegracefuldelete-r84v7
        basic.go:93: Pod nginx-testpodlifecyclegracefuldelete-r84v7 ready
        basic.go:120: Deleted pod: nginx-testpodlifecyclegracefuldelete-r84v7
PASS
...
```
