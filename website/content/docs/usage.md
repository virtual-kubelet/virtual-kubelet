---
title: Usage
description: Run a Virtual Kubelet inside or outside of your Kubernetes cluster
weight: 2
---

Virtual Kubelet is run via the `virtual-kubelet` command-line tool (documented [below](#virtual-kubelet-cli)). You can run Virtual Kubelet either [outside](#outside-k8s) or [inside](#inside-k8s) of a Kubernetes cluster.

## Outside of a Kubernetes cluster {#outside-k8s}

> Before you go through this section, make sure to [install Virtual Kubelet](../setup) first.

To run Virtual Kubelet outside of a Kubernetes cluster, run the [`virtual-kubelet`](#virtual-kubelet-cli) binary with your chosen [provider](../providers). Here's an example:

```bash
virtual-kubelet --provider aws
```

Once the Virtual Kubelet is deployed, run `kubectl get nodes` and you should see a `virtual-kubelet` node (unless you've named it something else using the [`--nodename`](#virtual-kubelet-cli) flag).

<!-- The CLI docs are generated using the shortcode in layouts/shortcodes/cli.html
and the YAML config in data/cli.yaml
-->
{{< cli >}}

## Inside a Kubernetes cluster {#inside-k8s}
