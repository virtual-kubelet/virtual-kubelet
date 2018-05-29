# Kubernetes Virtual Kubelet with Azure Batch

[Azure Batch](https://docs.microsoft.com/en-us/azure/batch/) provide an HPC Computing environment in Azure for distributed tasks. Azure handles scheduling decrete jobs and tasks accross pools of VM's. It is commonly used for batch processing tasks such as rendering.

The Virtual kubelet integration allows you to take advantage of this from within Kubernetes. The primary usecase for the provider is the simple executate of GPU based batch processes from normal Kubernetes clusters. For example, creating Kubernetes Jobs which train or execute ML models using Nvidia GPU's or using FFMPEG. As such there are some limitation to this provider. 

__The [ACI provider](../azure/README.MD) is the best option unless you're looking to utilise some specific features of Azure Batch__.

## Quick Start

The following Terraform template deploys an AKS cluster, Service Principal, Azure Batch Account and GPU enabled Azure Batch pool then runs a sample job which uses `nvidia-smi` to list out the driver details of the GPU.

1. Setup Terraform for Azure following [this guide here](https://docs.microsoft.com/en-us/azure/virtual-machines/linux/terraform-install-configure)
2. From the commandline move to the deployment folder `cd ./providers/azurebatch/deployment`
3. Use `terraform plan` and `terraform apply` to deploy the template
4. Run `kubectl create -f examplegpujob.yaml`
5. Run `pods=$(kubectl get pods --selector=job-name=examplegpujob --output=jsonpath={.items..metadata.name})` then `kubectl logs $pods` to view the logs

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
	JobID:           AZURE_BATCH_JOBID
	AccountLocation: AZURE_BATCH_ACCOUNT_LOCATION
	AccountName:     AZURE_BATCH_ACCOUNT_NAME
```

## Running

The provider will assign pods to machines in the Azure Batch Pool. Each machine can, by default, process only one pod at a time
running more than 1 pod per machine isn't currently supported and will result in errors.

Azure Batch queues tasks when no machines are available so pods will sit in `podPending` state while waiting for a VM to become available.