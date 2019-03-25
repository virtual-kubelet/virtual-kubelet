---
title: Providers
description: Extend the Virtual Kubelet interface
weight: 4
---

The Virtual Kubelet provides a pluggable **provider interface** that developers can implement to define the actions of a typical kubelet.

This enables on-demand and nearly instantaneous container compute, orchestrated by Kubernetes, without needing to manage VM infrastructure.

Each provider may have its own configuration file and required environment variables.

### Provider interface

Virtual Kubelet providers must provide the following functionality to be considered a fully compliant integration:

1. Provide the back-end plumbing necessary to support the lifecycle management of Pods, containers, and supporting resources in the context of Kubernetes.
2. Conform to the current API provided by Virtual Kubelet.
3. Restrict all access to the [Kubernetes API Server](https://kubernetes.io/docs/concepts/overview/kubernetes-api/) and provide a well-defined callback mechanism for retrieving data like [Secrets](https://kubernetes.io/docs/concepts/configuration/secret/) or [ConfigMaps](https://kubernetes.io/docs/tasks/configure-pod-container/configure-pod-configmap/).

### Current providers

Virtual Kubelet currently has a wide variety of providers:

{{< providers >}}

## Adding new providers {#adding}

To add a new Virtual Kubelet provider, create a new directory for your provider in the [`providers`](https://github.com/virtual-kubelet/virtual-kubelet/tree/master/providers) directory.

```shell
git clone https://github.com/virtual-kubelet/virtual-kubelet
cd virtual-kubelet
mkdir providers/my-provider
```

In that created directory, implement the [`Provider`](https://godoc.org/github.com/virtual-kubelet/virtual-kubelet/providers#Provider) interface in [Go](https://golang.org).

> For an example implementation of the Virtual Kubelet `Provider` interface, see the [Virtual Kubelet CRI Provider](https://github.com/virtual-kubelet/virtual-kubelet/tree/master/providers/cri), especially [`cri.go`](https://github.com/virtual-kubelet/virtual-kubelet/blob/master/providers/cri/cri.go).

Each Virtual Kubelet provider can be configured using its own configuration file and environment variables.

You can see the list of required methods, with relevant descriptions of each method, below:

```go
// Provider contains the methods required to implement a Virtual Kubelet provider
type Provider interface {
    // Takes a Kubernetes Pod and deploys it within the provider
    CreatePod(ctx context.Context, pod *v1.Pod) error

    // Takes a Kubernetes Pod and updates it within the provider
    UpdatePod(ctx context.Context, pod *v1.Pod) error

    // Takes a Kubernetes Pod and deletes it from the provider
    DeletePod(ctx context.Context, pod *v1.Pod) error

    // Retrieves a pod by name from the provider (can be cached)
    GetPod(ctx context.Context, namespace, name string) (*v1.Pod, error)

    // Retrieves the logs of a container by name from the provider
    GetContainerLogs(ctx context.Context, namespace, podName, containerName string, tail int) (string, error)

    // Executes a command in a container in the pod, copying data between
    // in/out/err and the container's stdin/stdout/stderr
    ExecInContainer(name string, uid types.UID, container string, cmd []string, in io.Reader, out, err io.WriteCloser, tty bool, resize <-chan remotecommand.TerminalSize, timeout time.Duration) error

    // Retrieves the status of a pod by name from the provider
    GetPodStatus(ctx context.Context, namespace, name string) (*v1.PodStatus, error)

    // Retrieves a list of all pods running on the provider (can be cached)
    GetPods(context.Context) ([]*v1.Pod, error)

    // Returns a resource list with the capacity constraints of the provider
    Capacity(context.Context) v1.ResourceList

    // Returns a list of conditions (Ready, OutOfDisk, etc), which is polled
    // periodically to update the node status within Kubernetes
    NodeConditions(context.Context) []v1.NodeCondition

    // Returns a list of addresses for the node status within Kubernetes
    NodeAddresses(context.Context) []v1.NodeAddress

    // Returns NodeDaemonEndpoints for the node status within Kubernetes.
    NodeDaemonEndpoints(context.Context) *v1.NodeDaemonEndpoints

    // Returns the operating system the provider is for
    OperatingSystem() string
}
```

In addition to `Provider`, there's an optional [`PodMetricsProvider`](https://godoc.org/github.com/virtual-kubelet/virtual-kubelet/providers#PodMetricsProvider) interface that providers can implement to expose Kubernetes Pod stats:

```go
type PodMetricsProvider interface {
    GetStatsSummary(context.Context) (*stats.Summary, error)
}
```

For a Virtual Kubelet provider to be considered viable, it must support the following functionality:

1. It must provide the backend plumbing necessary to support the lifecycle management of Pods, containers, and supporting resources in the Kubernetes context.
1. It must conform to the current API provided by Virtual Kubelet (see [above](#adding))
1. It won't have access to the [Kubernetes API server](https://kubernetes.io/docs/concepts/overview/kubernetes-api/), so it must provide a well-defined callback mechanism for fetching data like [Secrets](https://kubernetes.io/docs/concepts/configuration/secret/) and [ConfigMaps](https://kubernetes.io/docs/tutorials/configuration/).

In addition to implementing the `Provider` interface in `providers/<your provider>`, you also need to add your provider to the [`providers/register`](https://github.com/virtual-kubelet/virtual-kubelet/tree/master/providers/register) directory, in `provider_<your provider>.go`. Current examples include [`provider_azure.go`](https://github.com/virtual-kubelet/virtual-kubelet/blob/master/providers/register/provider_azure.go) and [`provider_aws.go`](https://github.com/virtual-kubelet/virtual-kubelet/blob/master/providers/register/provider_aws.go), which you can use as templates.

## Documentation

No Virtual Kubelet provider is complete without solid documentation. We strongly recommend providing a README for your provider in its directory. The READMEs for the currently existing implementations can provide a blueprint.

You'll also likely want your provider to appear in the [list of current providers](#current-providers). That list is generated from a [`provider.yaml`](https://github.com/virtual-kubelet/virtual-kubelet/blob/master/website/data/providers.yaml) file. Add a `name` field for the displayed name of the provider and the subdirectory as the `tag` field. The `name` field supports Markdown, so feel free to use bold text or a hyperlink.

## Testing

In order to test the provider you're developing, simply run `make test` from the root of the Virtual Kubelet directory.
