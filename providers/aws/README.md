# AWS Fargate

[AWS Fargate](https://aws.amazon.com/fargate/) is a technology for deploying and managing containers
without having to manage any of the underlying infrastructure. With AWS Fargate, you no longer have
to provision, configure, and scale clusters of virtual machines to run containers. This removes the
need to choose server types, decide when to scale your clusters, or optimize cluster packing.

Fargate makes it easy to scale your applications. You no longer have to worry about provisioning
enough compute resources. You can launch tens or tens of thousands of containers in seconds. Fargate
lets you focus on designing and building your applications instead of managing the infrastructure
that runs them.

With Fargate, billing is at a per second granularity and you only pay for what you use. You pay for
the amount of vCPU and memory resources your containerized application requests. vCPU and memory
resources are calculated from the time your container images are pulled until they terminate,
rounded up to the nearest second.

## Fargate virtual-kubelet provider

Fargate virtual-kubelet provider configures a Fargate cluster in AWS. Fargate resources show as a
node in your Kubernetes cluster. Pods scheduled on the Fargate node are deployed as Fargate
instances as if Fargate is a standard Kubernetes node.

## Configuration

A [sample configuration file](fargate.toml) is available.

## Usage

``
virtual-kubelet --provider aws --provider-config fargate.toml
``
