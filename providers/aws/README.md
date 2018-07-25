# AWS Fargate

[AWS Fargate](https://aws.amazon.com/fargate/) is a technology that allows you to run containers
without having to manage servers or clusters. With AWS Fargate, you no longer have to provision,
configure and scale clusters of virtual machines to run containers. This removes the need to choose
server types, decide when to scale your clusters, or optimize cluster packing. Fargate lets you
focus on designing and building your applications instead of managing the infrastructure that runs
them.

Fargate makes it easy to scale your applications. You no longer have to worry about provisioning
enough compute resources. You can launch tens or tens of thousands of containers in seconds. 

With Fargate, billing is at a per second granularity and you only pay for what you use. You pay for
the amount of vCPU and memory resources your containerized application requests. vCPU and memory
resources are calculated from the time your container images are pulled until they terminate,
rounded up to the nearest second.

## AWS Fargate virtual-kubelet provider

> Virtual-kubelet and the AWS Fargate virtual-kubelet provider are in very early stages of development.<br>
> DO NOT run them in any Kubernetes production environment or connect to any Fargate production cluster.

AWS Fargate virtual-kubelet provider connects your Kubernetes cluster to a Fargate cluster in AWS.
The Fargate cluster is exposed as a virtual node with the CPU and memory capacity that you choose.
Pods scheduled on the virtual node run on Fargate like they would run on a standard Kubernetes node.

See our [AWS Open Source Blog post](https://aws.amazon.com/blogs/opensource/aws-fargate-virtual-kubelet/) for detailed step-by-step instructions on how to run virtual-kubelet with AWS Fargate. If you are already familiar with virtual-kubelet, the rest of this README contains an overview of how to setup AWS Fargate.

## Prerequisites

If you have never used Fargate before, the easiest way to get started is to run Fargate's
[First run experience](https://console.aws.amazon.com/ecs/home?region=us-east-1#/firstRun). This
will setup Fargate in your AWS account with the default settings. It will create a default Fargate
cluster, IAM roles, a default VPC with an internet gateway and a default security group. You can
always fine-tune individual settings later.

Once you have your first application on Fargate running, move on to the next section below.

You may also want to install the
[AWS CLI](https://docs.aws.amazon.com/cli/latest/userguide/installing.html)
and visit the [AWS ECS console](https://console.aws.amazon.com/ecs) to take a closer look at your
Fargate resources.

## Configuration

In order to run virtual-kubelet for AWS Fargate, you need a simple configuration file. We have
provided a [sample configuration file](fargate.toml) for you that contains reasonable defaults and
brief descriptions for each field.

Create a copy of the sample configuration file and customize it.

If you ran the first-run experience, you only need to provide a subnet and set
AssignPublicIPv4Address to true. You can leave the security groups list blank to use the default
security group. You can learn your subnet ID in
[AWS console VPC subnets page](https://console.aws.amazon.com/vpc/home?#subnets). You
also need to update your [security group](https://console.aws.amazon.com/vpc/home?#securityGroups)
to allow traffic to your pods.

## Authentication via IAM

Virtual-kubelet needs permission to schedule pods on Fargate on your behalf. The easiest way to do
so is to run virtual-kubelet on a worker node in your Kubernetes cluster in EC2. Attach an IAM role
to the worker node EC2 instance and give it permission to your Fargate cluster.

## Connecting virtual-kubelet to your Kubernetes cluster

Copy the virtual-kubelet binary and your configuration file to your Kubernetes worker node in EC2.

```console
virtual-kubelet --provider aws --provider-config fargate.toml
```

In your Kubernetes cluster, confirm that the virtual-kubelet shows up as a node.
```console
kubectl get nodes

NAME                            STATUS    ROLES     AGE       VERSION
virtual-kubelet                 Ready     agent     5s        v1.8.3
```

To disconnect, stop the virtual-kubelet process.

## Deploying Kubernetes pods in AWS Fargate

Virtual-kubelet currently supports only a subset of regular kubelet functionality. In order to not
break existing pod deployments, pods that are to be deployed on Fargate require an explicit node
selector that points to the virtual node.
