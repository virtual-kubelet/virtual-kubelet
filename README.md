# Virtual Kubelet

[![Go Reference](https://pkg.go.dev/badge/github.com/virtual-kubelet/virtual-kubelet.svg)](https://pkg.go.dev/github.com/virtual-kubelet/virtual-kubelet)

Virtual Kubelet is an open source [Kubernetes kubelet](https://kubernetes.io/docs/reference/generated/kubelet/)
implementation that masquerades as a kubelet for the purposes of connecting Kubernetes to other APIs.
This allows the nodes to be backed by other services like ACI, AWS Fargate, [IoT Edge](https://github.com/Azure/iot-edge-virtual-kubelet-provider), [Tensile Kube](https://github.com/virtual-kubelet/tensile-kube) etc. The primary scenario for VK is enabling the extension of the Kubernetes API into serverless container platforms like ACI and Fargate, though we are open to others. However, it should be noted that VK is explicitly not intended to be an alternative to Kubernetes federation.

Virtual Kubelet features a pluggable architecture and direct use of Kubernetes primitives, making it much easier to build on.

We invite the Kubernetes ecosystem to join us in empowering developers to build
upon our base. Join our slack channel named, virtual-kubelet, within the [Kubernetes slack group](https://kubernetes.slack.com/).

The best description is "Kubernetes API on top, programmable back."

#### Table of Contents

* [How It Works](#how-it-works)
* [Usage](#usage)
* [Providers](#providers)
    + [Admiralty Multi-Cluster Scheduler](#admiralty-multi-cluster-scheduler)
    + [Alibaba Cloud ECI Provider](#alibaba-cloud-eci-provider)
    + [Azure Container Instances Provider](#azure-container-instances-provider)
	+ [Azure Batch GPU Provider](https://github.com/virtual-kubelet/azure-batch/blob/master/README.md)
    + [AWS Fargate Provider](#aws-fargate-provider)
    + [Elotl Kip](#elotl-kip)
	+ [HashiCorp Nomad](#hashicorp-nomad-provider)
    + [Liqo](#liqo-provider)
    + [OpenStack Zun](#openstack-zun-provider)
    + [Tensile Kube Provider](#tensile-kube-provider)
    + [Adding a New Provider via the Provider Interface](#adding-a-new-provider-via-the-provider-interface)
* [Testing](#testing)
    + [Unit tests](#unit-tests)
    + [End-to-end tests](#end-to-end-tests)
* [Known quirks and workarounds](#known-quirks-and-workarounds)
* [Contributing](#contributing)

## How It Works

The diagram below illustrates how Virtual-Kubelet works.

![diagram](website/static/img/diagram.svg)

## Usage

Virtual Kubelet is focused on providing a library that you can consume in your
project to build a custom Kubernetes node agent.

See godoc for up to date instructions on consuming this project:
https://godoc.org/github.com/virtual-kubelet/virtual-kubelet

There are implementations available for [several providers](#providers), see
those repos for details on how to deploy.

## Current Features

- create, delete and update pods
- container logs, exec, and metrics
- get pod, pods and pod status
- capacity
- node addresses, node capacity, node daemon endpoints
- operating system
- bring your own virtual network


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

### Admiralty Multi-Cluster Scheduler

Admiralty Multi-Cluster Scheduler mutates annotated pods into "proxy pods" scheduled on a virtual-kubelet node and creates corresponding "delegate pods" in remote clusters (actually running the containers). A feedback loop updates the statuses and annotations of the proxy pods to reflect the statuses and annotations of the delegate pods. You can find more details in the [Admiralty Multi-Cluster Scheduler documentation](https://github.com/admiraltyio/multicluster-scheduler).

### Alibaba Cloud ECI Provider

Alibaba Cloud ECI(Elastic Container Instance) is a service that allow you run containers without having to manage servers or clusters.

You can find more details in the [Alibaba Cloud ECI provider documentation](https://github.com/virtual-kubelet/alibabacloud-eci/blob/master/README.md).

#### Configuration File

The alibaba ECI provider will read configuration file specified by the `--provider-config` flag.

The example configure file is in the [ECI provider repository](https://github.com/virtual-kubelet/alibabacloud-eci/blob/master/eci.toml).

### Azure Container Instances Provider

The Azure Container Instances Provider allows you to utilize both
typical pods on VMs and Azure Container instances simultaneously in the
same Kubernetes cluster.

You can find detailed instructions on how to set it up and how to test it in the [Azure Container Instances Provider documentation](https://github.com/virtual-kubelet/azure-aci/blob/master/README.md).

#### Configuration File

The Azure connector can use a configuration file specified by the `--provider-config` flag.
The config file is in TOML format, and an example lives in `providers/azure/example.toml`.

### AWS Fargate Provider

[AWS Fargate](https://aws.amazon.com/fargate/) is a technology that allows you to run containers
without having to manage servers or clusters.

The AWS Fargate provider allows you to deploy pods to [AWS Fargate](https://aws.amazon.com/fargate/).
Your pods on AWS Fargate have access to VPC networking with dedicated ENIs in your subnets, public
IP addresses to connect to the internet, private IP addresses to connect to your Kubernetes cluster,
security groups, IAM roles, CloudWatch Logs and many other AWS services. Pods on Fargate can
co-exist with pods on regular worker nodes in the same Kubernetes cluster.

Easy instructions and a sample configuration file is available in the [AWS Fargate provider documentation](https://github.com/virtual-kubelet/aws-fargate). Please note that this provider is not currently supported. 

### Elotl Kip

[Kip](https://github.com/elotl/kip) is a provider that runs pods in cloud instances, allowing a Kubernetes cluster to transparently scale workloads into a cloud. When a pod is scheduled onto the virtual node, Kip starts a right-sized cloud instance for the pod's workload and dispatches the pod onto the instance. When the pod is finished running, the cloud instance is terminated.

When workloads run on Kip, your cluster size naturally scales with the cluster workload, pods are strongly isolated from each other and the user is freed from managing worker nodes and strategically packing pods onto nodes.

### HashiCorp Nomad Provider

HashiCorp [Nomad](https://nomadproject.io) provider for Virtual Kubelet connects your Kubernetes cluster
with Nomad cluster by exposing the Nomad cluster as a node in Kubernetes. By
using the provider, pods that are scheduled on the virtual Nomad node
registered on Kubernetes will run as jobs on Nomad clients as they
would on a Kubernetes node.

For detailed instructions, follow the guide [here](https://github.com/virtual-kubelet/nomad/blob/master/README.md).

### Liqo Provider

[Liqo](https://liqo.io) implements a provider for Virtual Kubelet designed to transparently offload pods and services to "peered" Kubernetes remote cluster. Liqo is capable of discovering neighbor clusters (using DNS, mDNS) and "peer" with them, or in other words, establish a relationship to share part of the cluster resources. When a cluster has established a peering, a new instance of the Liqo Virtual Kubelet is spawned to seamlessly extend the capacity of the cluster, by providing an abstraction of the resources of the remote cluster. The provider combined with the Liqo network fabric extends the cluster networking by enabling Pod-to-Pod traffic and multi-cluster east-west services, supporting endpoints on both clusters.

For detailed instruction, follow the guide [here](https://github.com/liqotech/liqo/blob/master/README.md)

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

For detailed instructions, follow the guide [here](https://github.com/virtual-kubelet/openstack-zun/blob/master/README.md).

### Tensile Kube Provider

[Tensile kube](https://github.com/virtual-kubelet/tensile-kube/blob/master/README.md) is contributed by [tencent
 games](https://game.qq.com), which is provider for Virtual Kubelet connects your Kubernetes cluster with other Kubernetes clusters. This provider enables us extending Kubernetes to an unlimited one. By using the provider, pods that are scheduled on the virtual node registered on Kubernetes will run as jobs on other Kubernetes clusters' nodes.

### Adding a New Provider via the Provider Interface

Providers consume this project as a library which implements the core logic of
a Kubernetes node agent (Kubelet), and wire up their implementation for
performing the neccessary actions.

There are 3 main interfaces:

#### PodLifecycleHandler

When pods are created, updated, or deleted from Kubernetes, these methods are
called to handle those actions.

[godoc#PodLifecylceHandler](https://godoc.org/github.com/virtual-kubelet/virtual-kubelet/node#PodLifecycleHandler)

```go
type PodLifecycleHandler interface {
    // CreatePod takes a Kubernetes Pod and deploys it within the provider.
    CreatePod(ctx context.Context, pod *corev1.Pod) error

    // UpdatePod takes a Kubernetes Pod and updates it within the provider.
    UpdatePod(ctx context.Context, pod *corev1.Pod) error

    // DeletePod takes a Kubernetes Pod and deletes it from the provider.
    DeletePod(ctx context.Context, pod *corev1.Pod) error

    // GetPod retrieves a pod by name from the provider (can be cached).
    GetPod(ctx context.Context, namespace, name string) (*corev1.Pod, error)

    // GetPodStatus retrieves the status of a pod by name from the provider.
    GetPodStatus(ctx context.Context, namespace, name string) (*corev1.PodStatus, error)

    // GetPods retrieves a list of all pods running on the provider (can be cached).
    GetPods(context.Context) ([]*corev1.Pod, error)
}
```

There is also an optional interface `PodNotifier` which enables the provider to
asynchronously notify the virtual-kubelet about pod status changes. If this
interface is not implemented, virtual-kubelet will periodically check the status
of all pods.

It is highly recommended to implement `PodNotifier`, especially if you plan
to run a large number of pods.

[godoc#PodNotifier](https://godoc.org/github.com/virtual-kubelet/virtual-kubelet/node#PodNotifier)

```go
type PodNotifier interface {
    // NotifyPods instructs the notifier to call the passed in function when
    // the pod status changes.
    //
    // NotifyPods should not block callers.
    NotifyPods(context.Context, func(*corev1.Pod))
}
```

`PodLifecycleHandler` is consumed by the `PodController` which is the core
logic for managing pods assigned to the node.

```go
	pc, _ := node.NewPodController(podControllerConfig) // <-- instatiates the pod controller
	pc.Run(ctx) // <-- starts watching for pods to be scheduled on the node
```

#### NodeProvider

NodeProvider is responsible for notifying the virtual-kubelet about node status
updates. Virtual-Kubelet will periodically check the status of the node and
update Kubernetes accordingly.

[godoc#NodeProvider](https://godoc.org/github.com/virtual-kubelet/virtual-kubelet/node#NodeProvider)

```go
type NodeProvider interface {
    // Ping checks if the node is still active.
    // This is intended to be lightweight as it will be called periodically as a
    // heartbeat to keep the node marked as ready in Kubernetes.
    Ping(context.Context) error

    // NotifyNodeStatus is used to asynchronously monitor the node.
    // The passed in callback should be called any time there is a change to the
    // node's status.
    // This will generally trigger a call to the Kubernetes API server to update
    // the status.
    //
    // NotifyNodeStatus should not block callers.
    NotifyNodeStatus(ctx context.Context, cb func(*corev1.Node))
}
```

Virtual Kubelet provides a `NaiveNodeProvider` that you can use if you do not
plan to have custom node behavior.

[godoc#NaiveNodeProvider](https://godoc.org/github.com/virtual-kubelet/virtual-kubelet/node#NaiveNodeProvider)

`NodeProvider` gets consumed by the `NodeController`, which is core logic for
managing the node object in Kubernetes.

```go
	nc, _ := node.NewNodeController(nodeProvider, nodeSpec) // <-- instantiate a node controller from a node provider and a kubernetes node spec
	nc.Run(ctx) // <-- creates the node in kubernetes and starts up he controller
```

#### API endpoints

One of the roles of a Kubelet is to accept requests from the API server for
things like `kubectl logs` and  `kubectl exec`. Helpers for setting this up are
provided [here](https://godoc.org/github.com/virtual-kubelet/virtual-kubelet/node/api)

#### Scrape Pod metrics

If you want to use HPA(Horizontal Pod Autoscaler) in your cluster, the provider should implement the `GetStatsSummary` function. Then metrics-server will be able to get the metrics of the pods on virtual-kubelet. Otherwise, you may see `No metrics for pod ` on metrics-server, which means the metrics of the pods on virtual-kubelet are not collected.


## Testing

### Unit tests

Running the unit tests locally is as simple as `make test`.

### End-to-end tests

Check out [`test/e2e`](./test/e2e) for more details.

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

Monthly Virtual Kubelet Office Hours are held at 10am PST on the second Thursday of every month in this [zoom meeting room](https://zoom.us/j/94701509915). Check out the calendar [here](https://calendar.google.com/calendar/embed?src=b119ced62134053de07d6c261b50d21ebde0da54f4163f5771b60ecf906e8b90%40group.calendar.google.com&ctz=America%2FLos_Angeles).

Our google drive with design specifications and meeting notes are [here](https://drive.google.com/drive/folders/19Ndu11WBCCBDowo9CrrGUHoIfd2L8Ueg?usp=sharing).

We also have a community slack channel named virtual-kubelet in the Kubernetes slack. You can also connect with the Virtual Kubelet community via the [mailing list](https://lists.cncf.io/g/virtualkubelet-dev).
