---
title: Setup
description: Install Virtual Kubelet using one of a variety of methods
weight: 1
---

You can install Virtual Kubelet in one of several ways:

* Using [`go install`](#go-install)
* By building [from source](#source)

## Using `go install` {#go-install}

You can install Virtual Kubelet using [`go install`](https://golang.org/cmd/go/#hdr-Compile_and_install_packages_and_dependencies) if you have [Go](https://golang.org) installed (instructions [here](https://golang.org/doc/install)).

You can `go install` from inside the Virtual Kubelet repository (if you have a [`GOPATH`](https://github.com/golang/go/wiki/GOPATH) set):

```bash
mkdir -p ${GOPATH}/src/github.com/virtual-kubelet
cd ${GOPATH}/src/github.com/virtual-kubelet
git clone https://github.com/virtual-kubelet/virtual-kubelet
go install
```

Both methods install a `virtual-kubelet` executable in `${GOPATH}/bin`.

To run the executable:

```bash
${GOPATH}/bin
```

Or if `${GOPATH}/bin` is in your path:

```bash
virtual-kubelet
```

## Building from source {#source}

To build Virtual Kubelet from source, first make sure that you have a [`GOPATH`](https://github.com/golang/go/wiki/GOPATH) set. Then clone the Virtual Kubelet repository and run `make build`:

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
