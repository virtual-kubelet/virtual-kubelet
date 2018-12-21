---
title: Architecture
description: How Virtual Kubelet works
weight: 3
---

This document provides a high-level overview of how Virtual Kubelet works. It begins by explaining how normal---i.e. non-virtual---[kubelets work](#kubelets) and then [explains Virtual Kubelet](#virtual-kubelet) by way of contrast.

## How kubelets usually work {#kubelets}

Ordinarily, Kubernetes [kubelets](https://kubernetes.io/docs/reference/command-line-tools-reference/kubelet/) implement Pod and container operations for each Kubernetes Node. They run as an agent on each Node, whether that Node is a physical server or a virtual machine, and handles Pod/container operations on that Node. kubelets take a configuration called a **PodSpec** as input and work to ensure that containers specified in the PodSpec are running and healthy.

## How Virtual Kubelet works {#virtual-kubelet}

From the standpoint of the Kubernetes API server, Virtual Kubelets *seem* like normal kubelets, but with the crucial difference that they scheduler containers elsewhere, for example in a cloud serverless API, and not on the Node.

[Figure 1](#figure-1) below shows a Kubernetes cluster with a series of standard kubelets and one Virtual Kubelet:

{{< svg src="img/diagram.svg" caption="Standard vs. Virtual Kubelets" >}}
