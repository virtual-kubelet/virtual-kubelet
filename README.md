# Virtual Kubelet

Virtual Kubelet is an open source [Kubernetes kubelet](https://kubernetes.io/docs/reference/generated/kubelet/)
implementation that masquerades as a kubelet for the purposes of connecting Kubernetes to other APIs.
This allows the nodes to be backed by other services like ACI, AWS Fargate, [IoT Edge](https://github.com/Azure/iot-edge-virtual-kubelet-provider) etc. The primary scenario for VK is enabling the extension of the Kubernetes API into serverless container platforms like ACI and Fargate, though we are open to others. However, it should be noted that VK is explicitly not intended to be an alternative to Kubernetes federation.
 
Virtual Kubelet features a pluggable architecture and direct use of Kubernetes primitives, making it much easier to build on.

We invite the Kubernetes ecosystem to join us in empowering developers to build
upon our base. Join our slack channel named, virtual-kubelet, within the [Kubernetes slack group](https://kubernetes.slack.com/).

Please note this software is experimental and should not be used for anything
resembling a production workload.

The best description is "Kubernetes API on top, programmable back."

#### Table of Contents

* [How It Works](#how-it-works)
* [Usage](#usage)
* [Providers](#providers)
    + [Alibaba Cloud ECI Provider](#alibaba-cloud-eci-provider)
    + [Azure Container Instances Provider](#azure-container-instances-provider)
	+ [Azure Batch GPU Provider](./providers/azurebatch/README.md)
    + [AWS Fargate Provider](#aws-fargate-provider)
	+ [HashiCorp Nomad](#hashicorp-nomad-provider)
    + [OpenStack Zun](#openstack-zun-provider)
    + [Adding a New Provider via the Provider Interface](#adding-a-new-provider-via-the-provider-interface)
* [Testing](#testing)
    + [Unit tests](#unit-tests)
    + [End-to-end tests](#end-to-end-tests)
    + [Testing the Azure Provider Client](#testing-the-azure-provider-client)
* [Known quirks and workarounds](#known-quirks-and-workarounds)
* [Contributing](#contributing)

## How It Works

The diagram below illustrates how Virtual-Kubelet works.

![diagram](website/static/img/diagram.svg)

## Usage

Deploy a Kubernetes cluster and make sure it's reachable.

### Outside the Kubernetes cluster

Run the binary with your chosen provider:

```bash
./bin/virtual-kubelet --provider <your provider>
```

Now that the virtual-kubelet is deployed run `kubectl get nodes` and you should see
a `virtual-kubelet` node.

### Inside the Kubernetes cluster (Minikube or Docker for Desktop)

It is possible to run the Virtual Kubelet as a Kubernetes pod inside a Minikube or Docker for Desktop cluster.
As of this writing, automation of this deployment is supported only for the mock provider, and is primarily intended at testing.
In order to deploy the Virtual Kubelet, you need to [install `skaffold`](https://github.com/GoogleContainerTools/skaffold#installation).
You also need to make sure that your current context is either `minikube` or `docker-for-desktop`.

In order to deploy the Virtual Kubelet, run the following command after the prerequisites have been met:

```console
$ make skaffold
```

By default, this will run `skaffold` in [_development_ mode](https://github.com/GoogleContainerTools/skaffold#skaffold-dev).
This will make `skaffold` watch `hack/skaffold/virtual-kubelet/Dockerfile` and its dependencies for changes and re-deploy the Virtual Kubelet when said changes happen.
It will also make `skaffold` stream logs from the Virtual Kubelet pod.

As an alternative, and if you are not concerned about continuous deployment and log streaming, you can run the following command instead:

```console
$ make skaffold MODE=run
```

This will build and deploy the Virtual Kubelet, and return.

## Current Features

- create, delete and update pods
- container logs, exec, and metrics 
- get pod, pods and pod status
- capacity 
- node addresses, node capacity, node daemon endpoints
- operating system
- bring your own virtual network 


## Command-Line Usage

```bash
virtual-kubelet implements the Kubelet interface with a pluggable
backend implementation allowing users to create kubernetes nodes without running the kubelet.
This allows users to schedule kubernetes workloads on nodes that aren't running Kubernetes.

Usage:
  virtual-kubelet [flags]
  virtual-kubelet [command]

Available Commands:
  help        Help about any command
  version     Show the version of the program

Flags:
  -h, --help                     help for virtual-kubelet
      --kubeconfig string        config file (default is $HOME/.kube/config)
      --namespace string         kubernetes namespace (default is 'all')
      --nodename string          kubernetes node name (default "virtual-kubelet")
      --os string                Operating System (Linux/Windows) (default "Linux")
      --provider string          cloud provider
      --provider-config string   cloud provider configuration file
      --taint string             apply taint to node, making scheduling explicit

Use "virtual-kubelet [command] --help" for more information about a command.
```

## Providers

This project features a pluggable provider interface developers can implement
that defines the actions of a typical kubelet.

This enables on-demand and nearly instantaneous container compute, orchestrated
by Kubernetes, without having VM infrastructure to manage and while still
leveraging the portable Kubernetes API.

Each provider may have its own configuration file, and required environmental variables.

Providers must provide the following functionality to be considered a supported integration with Virtual Kubelet.
1. Provides the back-end plumbing necessary to support the lifecycle management of pods, containers and supporting resources in the context of Kubernetes.
2. Conforms to the current API provided by Virtual Kubelet.
3. Does not have access to the Kubernetes API Server and has a well-defined callback mechanism for getting data like secrets or configmaps.


### Alibaba Cloud ECI Provider

Alibaba Cloud ECI(Elastic Container Instance) is a service that allow you run containers without having to manage servers or clusters.

You can find more details in the [Alibaba Cloud ECI provider documentation](./providers/alibabacloud/README.md).

#### Configuration File

The alibaba ECI provider will read configuration file specified by the `--provider-config` flag.

The example configure file is `providers/alibabacloud/eci.toml`.

### Azure Container Instances Provider

The Azure Container Instances Provider allows you to utilize both
typical pods on VMs and Azure Container instances simultaneously in the
same Kubernetes cluster.

You can find detailed instructions on how to set it up and how to test it in the [Azure Container Instances Provider documentation](./providers/azure/README.md).

#### Configuration File

The Azure connector can use a configuration file specified by the `--provider-config` flag.
The config file is in TOML format, and an example lives in `providers/azure/example.toml`.

#### More Details

See the [ACI Readme](providers/azure/README.md)

### AWS Fargate Provider

[AWS Fargate](https://aws.amazon.com/fargate/) is a technology that allows you to run containers
without having to manage servers or clusters.

The AWS Fargate provider allows you to deploy pods to [AWS Fargate](https://aws.amazon.com/fargate/).
Your pods on AWS Fargate have access to VPC networking with dedicated ENIs in your subnets, public
IP addresses to connect to the internet, private IP addresses to connect to your Kubernetes cluster,
security groups, IAM roles, CloudWatch Logs and many other AWS services. Pods on Fargate can
co-exist with pods on regular worker nodes in the same Kubernetes cluster.

Easy instructions and a sample configuration file is available in the [AWS Fargate provider documentation](providers/aws/README.md).

### HashiCorp Nomad Provider

HashiCorp [Nomad](https://nomadproject.io) provider for Virtual Kubelet connects your Kubernetes cluster
with Nomad cluster by exposing the Nomad cluster as a node in Kubernetes. By
using the provider, pods that are scheduled on the virtual Nomad node
registered on Kubernetes will run as jobs on Nomad clients as they
would on a Kubernetes node.

```bash
./bin/virtual-kubelet --provider="nomad"
```

For detailed instructions, follow the guide [here](providers/nomad/README.md).

### OpenStack Zun Provider

OpenStack [Zun](https://docs.openstack.org/zun/latest/) provider for Virtual Kubelet connects
your Kubernetes cluster with OpenStack in order to run Kubernetes pods on OpenStack Cloud.
Your pods on OpenStack have access to OpenStack tenant networks because they have Neutron ports
in your subnets. Each pod will have private IP addresses to connect to other OpenStack resources
(i.e. VMs) within your tenant, optionally have floating IP addresses to connect to the internet,
and bind-mount Cinder volumes into a path inside a pod's container.

```bash
./bin/virtual-kubelet --provider="openstack"
```

For detailed instructions, follow the guide [here](providers/openstack/README.md).

### Adding a New Provider via the Provider Interface

The structure we chose allows you to have all the power of the Kubernetes API
on top with a pluggable interface.

Create a new directory for your provider under `providers` and implement the
following interface. Then add register your provider in
`providers/register/<provider_name>_provider.go`. Make sure to add a build tag so that
your provider can be excluded from being built. The format for this build tag
should be `no_<provider_name>_provider`. Also make sure your provider has all
necessary platform build tags, e.g. "linux" if your provider only compiles on Linux.

```go
// Provider contains the methods required to implement a virtual-kubelet provider.
type Provider interface {
	// CreatePod takes a Kubernetes Pod and deploys it within the provider.
	CreatePod(ctx context.Context, pod *v1.Pod) error

	// UpdatePod takes a Kubernetes Pod and updates it within the provider.
	UpdatePod(ctx context.Context, pod *v1.Pod) error

	// DeletePod takes a Kubernetes Pod and deletes it from the provider.
	DeletePod(ctx context.Context, pod *v1.Pod) error

	// GetPod retrieves a pod by name from the provider (can be cached).
	GetPod(ctx context.Context, namespace, name string) (*v1.Pod, error)

	// GetContainerLogs retrieves the logs of a container by name from the provider.
	GetContainerLogs(ctx context.Context, namespace, podName, containerName string, tail int) (string, error)

	// ExecInContainer executes a command in a container in the pod, copying data
	// between in/out/err and the container's stdin/stdout/stderr.
	ExecInContainer(name string, uid types.UID, container string, cmd []string, in io.Reader, out, err io.WriteCloser, tty bool, resize <-chan remotecommand.TerminalSize, timeout time.Duration) error

	// GetPodStatus retrieves the status of a pod by name from the provider.
	GetPodStatus(ctx context.Context, namespace, name string) (*v1.PodStatus, error)

	// GetPods retrieves a list of all pods running on the provider (can be cached).
	GetPods(context.Context) ([]*v1.Pod, error)

	// Capacity returns a resource list with the capacity constraints of the provider.
	Capacity(context.Context) v1.ResourceList

	// NodeConditions returns a list of conditions (Ready, OutOfDisk, etc), which is
	// polled periodically to update the node status within Kubernetes.
	NodeConditions(context.Context) []v1.NodeCondition

	// NodeAddresses returns a list of addresses for the node status
	// within Kubernetes.
	NodeAddresses(context.Context) []v1.NodeAddress

	// NodeDaemonEndpoints returns NodeDaemonEndpoints for the node status
	// within Kubernetes.
	NodeDaemonEndpoints(context.Context) *v1.NodeDaemonEndpoints

	// OperatingSystem returns the operating system the provider is for.
	OperatingSystem() string
}

// PodMetricsProvider is an optional interface that providers can implement to expose pod stats
type PodMetricsProvider interface {
	GetStatsSummary(context.Context) (*stats.Summary, error)
}
```

## Testing

### Unit tests

Running the unit tests locally is as simple as `make test`.

### End-to-end tests

Virtual Kubelet includes an end-to-end (e2e) test suite which is used to validate its implementation.
The current e2e suite **does not** run for any providers other than the `mock` provider.

To run the e2e suite, three things are required:
- a local Kubernetes cluster (we have tested with [Docker for Mac](https://docs.docker.com/docker-for-mac/install/) and [Minikube](https://github.com/kubernetes/minikube));
- Your _kubeconfig_ default context points to the local Kubernetes cluster;
- [`skaffold`](https://github.com/GoogleContainerTools/skaffold).

Since our CI uses Minikube, we describe below how to run e2e on top of it.

To create a Minikube cluster, run the following command after [installing Minikube](https://github.com/kubernetes/minikube#installation):

```console
$ minikube start
```

The e2e suite requires Virtual Kubelet to be running as a pod inside the Kubernetes cluster.
In order to make the testing process easier, the build toolchain leverages on `skaffold` to automatically deploy the Virtual Kubelet to the Kubernetes cluster using the mock provider.

To run the e2e test suite, you can run the following command:

```console
$ make e2e
```

When you're done testing, you can run the following command to cleanup the resources created by `skaffold`:

```console
$ make skaffold MODE=delete
```

Please note that this will not unregister the Virtual Kubelet as a node in the Kubernetes cluster.
In order to do so, you should run:

```console
$ kubectl delete node vkubelet-mock-0
```

### Testing the Azure Provider Client

The unit tests for the [`azure`](providers/azure/) provider require a `credentials.json`
file exist in the root of this directory or that you have `AZURE_AUTH_LOCATION`
set to a credentials file.

You can generate this file by following the instructions listed in the
[README](providers/azure/client/README.md) for that package.

## Known quirks and workarounds

### Missing Load Balancer IP addresses for services

#### Providers that do not support service discovery

Kubernetes 1.9 introduces a new flag, `ServiceNodeExclusion`, for the control plane's Controller Manager. Enabling this flag in the Controller Manager's manifest allows Kubernetes to exclude Virtual Kubelet nodes from being added to Load Balancer pools, allowing you to create public facing services with external IPs without issue.

#### Workaround

Cluster requirements: Kubernetes 1.9 or above

Enable the ServiceNodeExclusion flag, by modifying the Controller Manager manifest and adding `--feature-gates=ServiceNodeExclusion=true` to the command line arguments.

## Contributing

Virtual Kubelet follows the [CNCF Code of Conduct](https://github.com/cncf/foundation/blob/master/code-of-conduct.md).
Sign the [CNCF CLA](https://github.com/kubernetes/community/blob/master/CLA.md) to be able to make Pull Requests to this repo. 

Bi-weekly Virtual Kubelet Architecture meetings are held at 11am PST in this [zoom meeting room](https://zoom.us/j/245165908).  Check out the calendar [here](https://calendar.google.com/calendar?cid=bjRtbGMxYWNtNXR0NXQ1a2hqZmRkNTRncGNAZ3JvdXAuY2FsZW5kYXIuZ29vZ2xlLmNvbQ).

Our google drive with design specifications and meeting notes are [here](https://drive.google.com/drive/folders/19Ndu11WBCCBDowo9CrrGUHoIfd2L8Ueg?usp=sharing).


