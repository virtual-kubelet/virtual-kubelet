---
title: Usage
description: Run a Virtual Kubelet either on or external to a Kubernetes cluster
weight: 2
---

You can Virtual Kubelet either [on](#on-k8s) or [external](#external-k8s) to a Kubernetes cluster using the [`virtual-kubelet`](#virtual-kubelet-cli) command-line tool. If you run Virtual Kubelet on a Kubernetes cluster, you can also deploy it using [Helm](#helm).

> For `virtual-kubelet` installation instructions, see the [Setup](../setup) guide.

## External to a Kubernetes cluster {#external-k8s}

To run Virtual Kubelet external to a Kubernetes cluster (not on the Kubernetes cluster you are connecting it to), run the [`virtual-kubelet`](#virtual-kubelet-cli) binary with your chosen [provider](../providers). Here's an example:

```bash
virtual-kubelet --provider aws
```

Once Virtual Kubelet is deployed, run `kubectl get nodes` and you should see a `virtual-kubelet` node (unless you've named it something else using the [`--nodename`](#virtual-kubelet-cli) flag).

<!-- The CLI docs are generated using the shortcode in layouts/shortcodes/cli.html
and the YAML config in data/cli.yaml
-->
{{< cli >}}

## On a Kubernetes cluster {#on-k8s}

It's possible to run the Virtual Kubelet as a Kubernetes Pod in a [Minikube](https://kubernetes.io/docs/setup/minikube/) or [Docker for Desktop](https://docs.docker.com/docker-for-windows/kubernetes/) Kubernetes cluster.

> At this time, automation of this deployment is supported only for the [`mock`](https://github.com/virtual-kubelet/virtual-kubelet/tree/master/cmd/virtual-kubelet/internal/provider/mock) provider.

In order to deploy the Virtual Kubelet, you need to install [Skaffold](https://skaffold.dev/), a Kubernetes development tool. You also need to make sure that your current [kubectl context](https://kubernetes.io/docs/tasks/access-application-cluster/configure-access-multiple-clusters/) is either `minikube` or `docker-for-desktop` (depending on which Kubernetes platform you're using).

First, clone the Virtual Kubelet repository:

```bash
git clone https://github.com/virtual-kubelet/virtual-kubelet
cd virtual-kubelet
```

Then:

```bash
make skaffold
```

By default, this will run Skaffold in [development mode](https://github.com/GoogleContainerTools/skaffold#a-glance-at-skaffold-workflow-and-architecture), which will make Skaffold watch [`hack/skaffold/virtual-kubelet/Dockerfile`](https://github.com/virtual-kubelet/virtual-kubelet/blob/master/hack/skaffold/virtual-kubelet/Dockerfile) and its dependencies for changes and re-deploy the Virtual Kubelet when changes happen. It will also make Skaffold stream logs from the Virtual Kubelet Pod.

Alternative, you can run Skaffold outside of development mode—if you aren't concerned about continuous deployment and log streaming—by running:

```bash
make skaffold MODE=run
```

This will build and deploy the Virtual Kubelet and return.

## Helm

{{< info >}}
[Helm](https://helm.sh) is a package manager that enables you to easily deploy complex systems on Kubernetes using configuration bundles called [Charts](https://docs.helm.sh/developing_charts/).
{{< /info >}}

You can use the Virtual Kubelet [Helm chart](https://github.com/virtual-kubelet/virtual-kubelet/tree/master/charts) to deploy Virtual Kubelet on Kubernetes.

First, add the Chart repository (the Chart is currently hosted on [GitHub](https://github.com)):

```bash
helm repo add virtual-kubelet \
  https://raw.githubusercontent.com/virtual-kubelet/virtual-kubelet/master/charts
```

{{< success >}}
You can check to make sure that the repo is listed amongst your current repos using `helm repo list`.
{{< /success >}}

Now you can install Virtual Kubelet using `helm install`. Here's an example command:

```bash
helm install virtual-kubelet/virtual-kubelet \
  --name virtual-kubelet-azure \
  --namespace virtual-kubelet \
  --set provider=azure
```

This would install the [Azure Container Instances Virtual Kubelet](https://github.com/virtual-kubelet/virtual-kubelet/tree/master/providers/azure) in the `virtual-kubelet` namespace.

To verify that Virtual Kubelet has been installed, run this command, which will list the available nodes and watch for changes:

```bash
kubectl get nodes \
  --namespace virtual-kubelet \
  --watch
```
