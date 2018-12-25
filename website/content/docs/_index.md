---
title: Overview
description: The basics of Virtual Kubelet
weight: 1
---

**Virtual Kubelet** is an implementation of the Kubernetes [kubelet](https://kubernetes.io/docs/reference/command-line-tools-reference/kubelet/) that masquerades as a kubelet for the purpose of connecting a Kubernetes cluster to other APIs. This allows Kubernetes [Nodes](https://kubernetes.io/docs/concepts/architecture/nodes/) to be backed by other services, such as serverless container platforms.

## Providers

Virtual Kubelet supports a variety of providers:

{{< providers >}}

You can also [add your own provider](providers#adding).
