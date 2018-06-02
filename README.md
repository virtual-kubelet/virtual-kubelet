# Virtual Kubelet

Virtual Kubelet is an open source [Kubernetes kubelet](https://kubernetes.io/docs/reference/generated/kubelet/)
implementation that masquerades as a kubelet for the purposes of connecting Kubernetes to other APIs.
This allows the nodes to be backed by other services like ACI, AWS Fargate, Hyper.sh, [IoT Edge](https://github.com/Azure/iot-edge-virtual-kubelet-provider) etc. The primary scenario for VK is enabling the extension of the Kubernetes API into serverless container platforms like ACI, Fargate, and Hyper.sh, though we are open to others. However, it should be noted that VK is explicitly not intended to be an alternative to Kubernetes federation.
 
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
    + [Azure Container Instances Provider](#azure-container-instances-provider)
    + [AWS Fargate Provider](#aws-fargate-provider)
    + [Hyper.sh Provider](#hypersh-provider)
    + [Adding a New Provider via the Provider Interface](#adding-a-new-provider-via-the-provider-interface)
* [Testing](#testing)
    + [Testing the Azure Provider Client](#testing-the-azure-provider-client)
* [Known quirks and workarounds](#known-quirks-and-workarounds)
* [Contributing](#contributing)

## How It Works

The diagram below illustrates how Virtual-Kubelet works.

![diagram](diagram.svg)

## Usage

Deploy a Kubernetes cluster and make sure it's reachable.

Run the binary with your chosen provider:

```bash
./bin/virtual-kubelet --provider <your provider>
```

Now that the virtual-kubelet is deployed run `kubectl get nodes` and you should see
a `virtual-kubelet` node.

## Current Features

- create, delete and update pods
- container logs
- get pod, pods and pod status
- capacity 
- node addresses, node capacity, node daemon endpoints
- operating system


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
1. Provides the back-end plumbing necessary to support the lifecycle management of pods, containers and supporting resources in the context of Kubernete.
2. Conforms to the current API provided by Virtual Kubelet.
3. Does not have access to the Kubernetes API Server and has a well-defined callback mechanism for getting data like secrets or configmaps.


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

### Hyper.sh Provider

The Hyper.sh Provider allows Kubernetes clusters to deploy Hyper.sh containers
and manage both typical pods on VMs and Hyper.sh containers in the same
Kubernetes cluster.

```bash
./bin/virtual-kubelet --provider hyper
```

### Adding a New Provider via the Provider Interface

The structure we chose allows you to have all the power of the Kubernetes API
on top with a pluggable interface.

Create a new directory for your provider under `providers` and implement the
following interface. Then add your new provider under the others in the
[`vkubelet/provider.go`](vkubelet/provider.go) file.

```go
// Provider contains the methods required to implement a virtual-kubelet provider.
type Provider interface {
	// CreatePod takes a Kubernetes Pod and deploys it within the provider.
	CreatePod(pod *v1.Pod) error

	// UpdatePod takes a Kubernetes Pod and updates it within the provider.
	UpdatePod(pod *v1.Pod) error

	// DeletePod takes a Kubernetes Pod and deletes it from the provider.
	DeletePod(pod *v1.Pod) error

	// GetPod retrieves a pod by name from the provider (can be cached).
	GetPod(namespace, name string) (*v1.Pod, error)

	// GetPodStatus retrievesthe status of a pod by name from the provider.
	GetPodStatus(namespace, name string) (*v1.PodStatus, error)

	// GetPods retrieves a list of all pods running on the provider (can be cached).
	GetPods() ([]*v1.Pod, error)

	// Capacity returns a resource list with the capacity constraints of the provider.
	Capacity() v1.ResourceList

	// NodeConditions returns a list of conditions (Ready, OutOfDisk, etc), which is polled periodically to update the node status
	// within Kubernetes.
	NodeConditions() []v1.NodeCondition

	// OperatingSystem returns the operating system the provider is for.
	OperatingSystem() string
}
```

## Testing

Running the unit tests locally is as simple as `make test`.

### Testing the Azure Provider Client

The unit tests for the [`azure`](providers/azure/) provider require a `credentials.json`
file exist in the root of this directory or that you have `AZURE_AUTH_LOCATION`
set to a credentials file.

You can generate this file by following the instructions listed in the
[README](providers/azure/client/README.md) for that package.

## Known quirks and workarounds

### Missing Load Balancer IP addresses for services

#### When Virtual Kubelet is installed on a cluster, you cannot create external-IPs for a Service

Kubernetes 1.9 introduces a new flag, `ServiceNodeExclusion`, for the control plane's Controller Manager. Enabling this flag in the Controller Manager's manifest allows Kubernetes to exclude Virtual Kubelet nodes from being added to Load Balancer pools, allowing you to create public facing services with external IPs without issue.

#### Workaround

Cluster requirements: Kubernetes 1.9 or above

Enable the ServiceNodeExclusion flag, by modifying the Controller Manager manifest and adding `--feature-gates=ServiceNodeExclusion=true` to the command line arguments.

## Contributing

Virtual Kubelet follows the [CNCF Code of Conduct](https://github.com/cncf/foundation/blob/master/code-of-conduct.md).
Sign the [CNCF CLA](https://github.com/kubernetes/community/blob/master/CLA.md) to be able to make Pull Requests to this repo. 

Weekly Virtual Kubelet Architecture meetings are held at 3pm PST [here](https://zoom.us/j/5337610301). Our google drive with design specifications and meeting notes are [here](https://drive.google.com/drive/folders/19Ndu11WBCCBDowo9CrrGUHoIfd2L8Ueg?usp=sharing).

