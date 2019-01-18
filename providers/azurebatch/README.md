# Kubernetes Virtual Kubelet with Azure Batch

[Azure Batch](https://docs.microsoft.com/en-us/azure/batch/) provides a HPC Computing environment in Azure for distributed tasks. Azure Batch handles scheduling of discrete jobs and tasks accross pools of VM's. It is commonly used for batch processing tasks such as rendering.

The Virtual kubelet integration allows you to take advantage of this from within Kubernetes. The primary usecase for the provider is to make it easy to use GPU based workload from normal Kubernetes clusters. For example, creating Kubernetes Jobs which train or execute ML models using Nvidia GPU's or using FFMPEG.

Azure Batch allows for [low priority nodes](https://docs.microsoft.com/en-us/azure/batch/batch-low-pri-vms) which can also help to reduce cost for non-time sensitive workloads.

__The [ACI provider](../azure/README.md) is the best option unless you're looking to utilise some specific features of Azure Batch__.

## Status: Experimental

This provider is currently in the exterimental stages. Contributions welcome!

## Quick Start

The following Terraform template deploys an AKS cluster with the Virtual Kubelet, Azure Batch Account and GPU enabled Azure Batch pool. The Batch pool contains 1 Dedicated NC6 Node and 2 Low Priority NC6 Nodes.

1. Setup Terraform for Azure following [this guide here](https://docs.microsoft.com/en-us/azure/virtual-machines/linux/terraform-install-configure)
2. From the commandline move to the deployment folder `cd ./providers/azurebatch/deployment` then edit `vars.example.tfvars` adding in your Service Principal details
3. Download the latest version of the Community Kubernetes Provider for Terraform. Get the correct link [from here](https://github.com/sl1pm4t/terraform-provider-kubernetes/releases) and use it as follows: (Current official Terraform K8s provider doesn't support `Deployments`)

```shell
curl -L -o - PUT_RELASE_BINARY_LINK_YOU_FOUND_HERE | gunzip > terraform-provider-kubernetes
chmod +x ./terraform-provider-kubernetes
```

4. Use `terraform init` to initialize the template
5. Use `terraform plan -var-file=./vars.example.tfvars` and `terraform apply -var-file=./vars.example.tfvars` to deploy the template
6. Run `kubectl describe deployment/vkdeployment` to check the virtual kubelet is running correctly.
7. Run `kubectl create -f examplegpupod.yaml`
8. Run `pods=$(kubectl get pods --selector=app=examplegpupod --show-all --output=jsonpath={.items..metadata.name})` then `kubectl logs $pods` to view the logs. Should see:

```text
	[Vector addition of 50000 elements]
	Copy input data from the host memory to the CUDA device
	CUDA kernel launch with 196 blocks of 256 threads
	Copy output data from the CUDA device to the host memory
	Test PASSED
	Done
```

### Tweaking the Quickstart

You can update [main.tf](./main.tf) to increase the number of nodes allocated to the Azure Batch pool or update [./aks/main.tf](./aks/main.tf) to increase the number of agent nodes allocated to your AKS cluster.

## Advanced Setup

## Prerequistes

1. An Azure Batch Account configurated
2. An Azure Batch Pool created with necessary VM spec. VM's in the pool must have:
    - `docker` installed and correctly configured
    - `nvidia-docker` and `cuda` drivers installed
3. K8s cluster
4. Azure Service Principal with access to the Azure Batch Account

## Setup

The provider expects the following environment variables to be configured:

```
    ClientID:        AZURE_CLIENT_ID
	ClientSecret:    AZURE_CLIENT_SECRET
	ResourceGroup:   AZURE_RESOURCE_GROUP
	SubscriptionID:  AZURE_SUBSCRIPTION_ID
	TenantID:        AZURE_TENANT_ID
	PoolID:          AZURE_BATCH_POOLID
	JobID (optional):AZURE_BATCH_JOBID
	AccountLocation: AZURE_BATCH_ACCOUNT_LOCATION
	AccountName:     AZURE_BATCH_ACCOUNT_NAME
```

## Running

The provider will assign pods to machines in the Azure Batch Pool. Each machine can, by default, process only one pod at a time
running more than 1 pod per machine isn't currently supported and will result in errors.

Azure Batch queues tasks when no machines are available so pods will sit in `podPending` state while waiting for a VM to become available.
