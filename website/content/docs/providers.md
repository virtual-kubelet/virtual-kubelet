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

For a basic example, see the [Virtual Kubelet CRI Provider](https://github.com/virtual-kubelet/virtual-kubelet/tree/master/providers/cri).
