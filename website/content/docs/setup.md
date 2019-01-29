---
title: Setup
description: Install Virtual Kubelet using one of several methods
weight: 1
---

You can install Virtual Kubelet by building it [from source](#source). First, make sure that you have a [`GOPATH`](https://github.com/golang/go/wiki/GOPATH) set. Then clone the Virtual Kubelet repository and run `make build`:

```bash
mkdir -p ${GOPATH}/src/github.com/virtual-kubelet
cd ${GOPATH}/src/github.com/virtual-kubelet
git clone https://github.com/virtual-kubelet/virtual-kubelet
make build
```

This method adds a `virtual-kubelet` executable to the `bin` folder. To run it:

```bash
bin/virtual-kubelet
```

## Using Virtual Kubelet

Once you have Virtual Kubelet installed, you can move on to the [Usage](../usage) documentation.
