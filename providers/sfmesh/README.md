# Kubernetes Virtual Kubelet with Service Fabric Mesh

[Service Fabric Mesh](https://docs.microsoft.com/en-us/azure/service-fabric-mesh/service-fabric-mesh-overview) is a fully managed service that enables developers to deploy microservices applications without managing virtual machines, storage, or networking. Applications hosted on Service Fabric Mesh run and scale without you worrying about the infrastructure powering them.

The Virtual kubelet integration allows you to use the Kubernetes API to burst out compute to Service Fabric Mesh and schedule pods as Mesh Applications.

## Status: Experimental

This provider is currently in the experimental stages. Contributions are welcome!

## Setup

The provider expects the following environment variables to be configured:

- AZURE_CLIENT_ID
- AZURE_CLIENT_SECRET
- AZURE_SUBSCRIPTION_ID
- AZURE_TENANT_ID
- RESOURCE_GROUP
- REGION

## Quick Start

#### Run the Virtual Kubelet

```
./virtual-kubelet --provider=sfmesh --taint azure.com/sfmesh
```

#### Create pod yaml:

```
$ cat pod-nginx
apiVersion: v1
kind: Pod
metadata:
  name: nginx
spec:
  nodeName: virtual-kubelet
  containers:
  - name: nginx
    image: nginx:latest
    ports:
    - containerPort: 80
  tolerations:
  - key: azure.com/sfmesh
    effect: NoSchedule
```

#### create pod

```
$ kubectl create -f pod-nginx
```

#### list containers on Service Fabric Mesh

```
$ az mesh app list -o table

Name    ResourceGroup    ProvisioningState    Location
------  ---------------  -------------------  ----------
nginx   myResourceGroup  Succeeded            eastus
```
