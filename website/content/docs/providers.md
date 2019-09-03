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

To add a new Virtual Kubelet provider, create a new directory for your provider.

In that created directory, implement [`PodLifecycleHandler`](https://godoc.org/github.com/virtual-kubelet/virtual-kubelet/node#PodLifecycleHandler) interface in [Go](https://golang.org).

> For an example implementation of the Virtual Kubelet `PodLifecycleHandler` interface, see the [Virtual Kubelet CRI Provider](https://github.com/virtual-kubelet/cri), especially [`cri.go`](https://github.com/virtual-kubelet/cri/blob/master/cri.go).

Each Virtual Kubelet provider can be configured using its own configuration file and environment variables.

You can see the list of required methods, with relevant descriptions of each method, below:

```go
// PodLifecycleHandler defines the interface used by the PodController to react
// to new and changed pods scheduled to the node that is being managed.
//
// Errors produced by these methods should implement an interface from
// github.com/virtual-kubelet/virtual-kubelet/errdefs package in order for the
// core logic to be able to understand the type of failure.
type PodLifecycleHandler interface {
    // CreatePod takes a Kubernetes Pod and deploys it within the provider.
    CreatePod(ctx context.Context, pod *corev1.Pod) error

    // UpdatePod takes a Kubernetes Pod and updates it within the provider.
    UpdatePod(ctx context.Context, pod *corev1.Pod) error

    // DeletePod takes a Kubernetes Pod and deletes it from the provider.
    DeletePod(ctx context.Context, pod *corev1.Pod) error

    // GetPod retrieves a pod by name from the provider (can be cached).
    // The Pod returned is expected to be immutable, and may be accessed
    // concurrently outside of the calling goroutine. Therefore it is recommended
    // to return a version after DeepCopy.
    GetPod(ctx context.Context, namespace, name string) (*corev1.Pod, error)

    // GetPodStatus retrieves the status of a pod by name from the provider.
    // The PodStatus returned is expected to be immutable, and may be accessed
    // concurrently outside of the calling goroutine. Therefore it is recommended
    // to return a version after DeepCopy.
    GetPodStatus(ctx context.Context, namespace, name string) (*corev1.PodStatus, error)

    // GetPods retrieves a list of all pods running on the provider (can be cached).
    // The Pods returned are expected to be immutable, and may be accessed
    // concurrently outside of the calling goroutine. Therefore it is recommended
    // to return a version after DeepCopy.
    GetPods(context.Context) ([]*corev1.Pod, error)
}
```

In addition to `PodLifecycleHandler`, there's an optional [`PodMetricsProvider`](https://godoc.org/github.com/virtual-kubelet/virtual-kubelet/cmd/virtual-kubelet/internal/provider#PodMetricsProvider) interface that providers can implement to expose Kubernetes Pod stats:

```go
type PodMetricsProvider interface {
    GetStatsSummary(context.Context) (*stats.Summary, error)
}
```

For a Virtual Kubelet provider to be considered viable, it must support the following functionality:

1. It must provide the backend plumbing necessary to support the lifecycle management of Pods, containers, and supporting resources in the Kubernetes context.
1. It must conform to the current API provided by Virtual Kubelet (see [above](#adding))
1. It won't have access to the [Kubernetes API server](https://kubernetes.io/docs/concepts/overview/kubernetes-api/), so it must provide a well-defined callback mechanism for fetching data like [Secrets](https://kubernetes.io/docs/concepts/configuration/secret/) and [ConfigMaps](https://kubernetes.io/docs/tutorials/configuration/).

## Documentation

No Virtual Kubelet provider is complete without solid documentation. We strongly recommend providing a README for your provider in its directory. The READMEs for the currently existing implementations can provide a blueprint.

You'll also likely want your provider to appear in the [list of current providers](#current-providers). That list is generated from a [`provider.yaml`](https://github.com/virtual-kubelet/virtual-kubelet/blob/master/website/data/providers.yaml) file. Add a `name` field for the displayed name of the provider and the subdirectory as the `tag` field. The `name` field supports Markdown, so feel free to use bold text or a hyperlink.

## Testing

In order to test the provider you're developing, simply run `make test` from the root of the Virtual Kubelet directory.
