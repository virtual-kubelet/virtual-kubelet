# Kubernetes Virtual Kubelet with ACI

Azure Container Instances (ACI) provide a hosted environment for running containers in Azure. When using ACI, there is no need to manage the underlying compute infrastructure, Azure handles this management for you. When running containers in ACI, you are charged by the second for each running container.

The Azure Container Instances provider for the Virtual Kubelet configures an ACI instance as a node in any Kubernetes cluster. When using the Virtual Kubelet ACI provider, pods can be scheduled on an ACI instance as if the ACI instance is a standard Kubernetes node. This configuration allows you to take advantage of both the capabilities of Kubernetes and the management value and cost benefit of ACI.

This document details configuring the Virtual Kubelet ACI provider.
#### Table of Contents

* [Prerequiste](#prerequisite)
* [Quick set-up with the ACI Connector](#quick-set-up-with-the-aci-connector)
* [Manual set-up](#manual-set-up)
* [Create a cluster with a Virtual Network](#create-an-AKS-cluster-with-VNet)
* [Validate the Virtual Kubelet ACI provider](#validate-the-virtual-kubelet-aci-provider)
* [Schedule a pod in ACI](#schedule-a-pod-in-aci)
* [Upgrade the ACI Connector ](#upgrade-the-aci-connector)
* [Remove the Virtual Kubelet](#remove-the-virtual-kubelet)
## Prerequisite

This guide assumes that you have a Kubernetes cluster up and running (can be `minikube`) and that `kubectl` is already configured to talk to it.

Other pre-requesites are:

* A [Microsoft Azure account](https://azure.microsoft.com/en-us/free/).
* Install the [Azure CLI](#install-the-azure-cli).
* Install the [Kubernetes CLI](#install-the-kubernetes-cli).
* Install the [Helm CLI](#install-the-helm-cli).

### Install the Azure CLI

Install `az` by following the instructions for your operating system.
See the [full installation instructions](https://docs.microsoft.com/en-us/cli/azure/install-azure-cli?view=azure-cli-latest) if yours isn't listed below.

#### MacOS

```cli
brew install azure-cli
```

#### Windows

Download and run the [Azure CLI Installer (MSI)](https://aka.ms/InstallAzureCliWindows).

#### Ubuntu 64-bit

1. Add the azure-cli repo to your sources:
    ```cli
    echo "deb [arch=amd64] https://packages.microsoft.com/repos/azure-cli/ wheezy main" | \
         sudo tee /etc/apt/sources.list.d/azure-cli.list
    ```
2. Run the following commands to install the Azure CLI and its dependencies:
    ```cli
    sudo apt-key adv --keyserver packages.microsoft.com --recv-keys 52E16F86FEE04B979B07E28DB02C46DF417A0893
    sudo apt-get install apt-transport-https
    sudo apt-get update && sudo apt-get install azure-cli
    ```

### Install the Kubernetes CLI

Install `kubectl` by running the following command:

```cli
az aks install-cli
```

### Install the Helm CLI

[Helm](https://github.com/kubernetes/helm) is a tool for installing pre-configured applications on Kubernetes.
Install `helm` by running the following command:

#### MacOS

```cli
brew install kubernetes-helm
```

#### Windows

1. Download the latest [Helm release](https://storage.googleapis.com/kubernetes-helm/helm-v2.7.2-windows-amd64.tar.gz).
2. Decompress the tar file.
3. Copy **helm.exe** to a directory on your PATH.

#### Linux

```cli
curl https://raw.githubusercontent.com/kubernetes/helm/master/scripts/get | bash
```
---

## Cluster and Azure Account Setup

Now that we have all the tools, we will set up your Azure account to work with ACI.

### Configure your Azure account

First let's identify your Azure subscription and save it for use later on in the quickstart.

1. Run `az login` and follow the instructions in the command output to authorize `az` to use your account
2. List your Azure subscriptions:
    ```cli
    az account list -o table
    ```
3. Copy your subscription ID and save it in an environment variable:

    **Bash**
    ```cli
    export AZURE_SUBSCRIPTION_ID="<SubscriptionId>"
    ```

    **PowerShell**
    ```cli
    $env:AZURE_SUBSCRIPTION_ID = "<SubscriptionId>"
    ```

4. Enable ACI in your subscription:

   ```cli
   az provider register -n Microsoft.ContainerInstance
   ```

## Quick set-up with the ACI Connector

The Azure cli can be used to install the ACI provider. We like to say Azure's provider or implementation for Virtual Kubelet is the ACI Connector. 
For this section Virtual Kubelet's specific ACI provider will be referenced as the the ACI Connector. 
If you continue with this section you can skip sections below up to "Schedule a pod in ACI", as we use Azure Container Service (AKS) to easily deploy and install the connector, thus it is assumed 
that you've created an [AKS cluster](https://docs.microsoft.com/en-us/azure/aks/kubernetes-walkthrough). 

To install the ACI Connector use the az cli and the aks namespace. Make sure to use the resource group of the aks cluster you've created and the name of the aks cluster you've created. You can choose the connector name to be anything. Choose any command below to install the Linux, Windows, or both the Windows and Linux Connector.

Note: You need to specify the --aci-resource-group, due to a bug in the az cli. The resource group is then auto-generated. To find the name navigate to the Azure Portal resource groups, scroll down and find the name that matches MC_aks cluster name_aks rg_location.

1. Install the Linux ACI Connector

   **Bash**
   ```cli
   az aks install-connector --resource-group <aks cluster rg> --name <aks cluster name> --os-type linux --connector-name myaciconnector --aci-resource-group <auto-generated aks cluster rg>
   ```

2. Install the Windows ACI Connector

   **Bash**
   ```cli 
   az aks install-connector --resource-group <aks cluster rg> --name <aks cluster name> --os-type windows --connector-name myaciconnector --aci-resource-group <auto-generated aks cluster rg>
   ```

3. Install both the Windows and Linux ACI Connectors

   **Bash**
   ```cli
   az aks install-connector --resource-group <aks cluster rg> --name <aks cluster name> --os-type both --connector-name myaciconnector --aci-resource-group <auto-generated aks cluster rg>
   ```

Now you are ready to deploy a pod to the connector so skip to the "Schedule a pod in ACI" section. 

## Manual set-up

### Create a Resource Group for ACI

To use Azure Container Instances, you must provide a resource group. Create one with the az cli using the following command.

```cli
export ACI_REGION=eastus
az group create --name aci-group --location "$ACI_REGION"
export AZURE_RG=aci-group
```

### Create a service principal

This creates an identity for the Virtual Kubelet ACI provider to use when provisioning
resources on your account on behalf of Kubernetes.

1. Create a service principal with RBAC enabled for the quickstart:
    ```cli
    az ad sp create-for-rbac --name virtual-kubelet-quickstart -o table
    ```
2. Save the values from the command output in environment variables:

    **Bash**
    ```cli
    export AZURE_TENANT_ID=<Tenant>
    export AZURE_CLIENT_ID=<AppId>
    export AZURE_CLIENT_SECRET=<Password>
    ```

    **PowerShell**
    ```cli
    $env:AZURE_TENANT_ID = "<Tenant>"
    $env:AZURE_CLIENT_ID = "<AppId>"
    $env:AZURE_CLIENT_SECRET = "<Password>"
    ```

## Deployment of the ACI provider in your cluster

Run these commands to deploy the virtual kubelet which connects your Kubernetes cluster to Azure Container Instances.

```cli
export AKS_VK_RELEASE=virtual-kubelet-0.1.7
export VK_RElEASE=virtual-kubelet-0.3.0
```

If your cluster is an AKS cluster:
```cli
RELEASE_NAME=virtual-kubelet
NODE_NAME=virtual-kubelet
CHART_URL=https://github.com/virtual-kubelet/virtual-kubelet/raw/master/charts/$AKS_VK_RELEASE.tgz

curl https://raw.githubusercontent.com/virtual-kubelet/virtual-kubelet/master/scripts/createCertAndKey.sh > createCertAndKey.sh
chmod +x createCertAndKey.sh
. ./createCertAndKey.sh

helm install "$CHART_URL" --name "$RELEASE_NAME" \
  --set provider=azure \
  --set providers.azure.targetAKS=true \
  --set providers.azure.tenantId=$AZURE_TENANT_ID \
  --set providers.azure.subscriptionId=$AZURE_SUBSCRIPTION_ID \
  --set providers.azure.clientId=$AZURE_CLIENT_ID \
  --set apiserverCert=$cert \
  --set apiserverKey=$key
```

For any other type of Kubernetes cluster:
```cli
RELEASE_NAME=virtual-kubelet
NODE_NAME=virtual-kubelet
CHART_URL=https://github.com/virtual-kubelet/virtual-kubelet/raw/master/charts/$VK_RELEASE.tgz

curl https://raw.githubusercontent.com/virtual-kubelet/virtual-kubelet/master/scripts/createCertAndKey.sh > createCertAndKey.sh
chmod +x createCertAndKey.sh
. ./createCertAndKey.sh

helm install "$CHART_URL" --name "$RELEASE_NAME" \
  --set provider=azure \
  --set rbac.install=true \
  --set providers.azure.targetAKS=false \
  --set providers.azure.tenantId=$AZURE_TENANT_ID \
  --set providers.azure.subscriptionId=$AZURE_SUBSCRIPTION_ID \
  --set providers.azure.clientId=$AZURE_CLIENT_ID \
  --set providers.azure.clientKey=$AZURE_CLIENT_SECRET \
  --set providers.azure.aciResourceGroup=$AZURE_RG \
  --set providers.azure.aciRegion=$ACI_REGION \
  --set apiserverCert=$cert \
  --set apiserverKey=$key
```

If your cluster has RBAC enabled set ```rbac.install=true```

Output:

```console
NAME:   virtual-kubelet
LAST DEPLOYED: Thu Feb 15 13:17:01 2018
NAMESPACE: default
STATUS: DEPLOYED

RESOURCES:
==> v1/Secret
NAME                             TYPE    DATA  AGE
virtual-kubelet-virtual-kubelet  Opaque  3     1s

==> v1beta1/Deployment
NAME                             DESIRED  CURRENT  UP-TO-DATE  AVAILABLE  AGE
virtual-kubelet-virtual-kubelet  1        1        1           0          1s

==> v1/Pod(related)
NAME                                              READY  STATUS             RESTARTS  AGE
virtual-kubelet-virtual-kubelet-7bcf5dc749-6mvgp  0/1    ContainerCreating  0         1s


NOTES:
The virtual kubelet is getting deployed on your cluster.

To verify that virtual kubelet has started, run:

```cli
  kubectl --namespace=default get pods -l "app=virtual-kubelet-virtual-kubelet"
```
##  Create an AKS cluster with VNet

  Run the following commands to create an AKS cluster with a new Azure virtual network. Also, create two subnets. One will be delegated to the cluster and the other will be delegated to Azure Container Instances. 

### Create an Azure virtual network and subnets 

  First, set the following variables for your VNet range and two subnet ranges within that VNet. The following ranges are recommended for those just trying out the connector with VNet. 

  **Bash**
  ```cli
    export VNET_RANGE=10.8.0.0/8  
    export CLUSTER_SUBNET_RANGE=10.240.0.0/16 
    export ACI_SUBNET_RANGE=10.241.0.0/16 
    export VNET_NAME=myAKSVNet 
    export CLUSTER_SUBNET_NAME=myAKSSubnet 
    export ACI_SUBNET_NAME=myACISubnet 
    export AKS_CLUSTER_RG=myresourcegroup 
  ```
  Run the following command to create a virtual network within Azure, and a subnet within that VNet. The subnet will be dedicated to the nodes in the AKS cluster.

    ```cli
    az network vnet create \
    --resource-group $AKS_CLUSTER_RG \
    --name $VNET_NAME \
    --address-prefixes $VNET_RANGE \
    --subnet-name $CLUSTER_SUBNET_NAME \
    --subnet-prefix $CLUSTER_SUBNET_RANGE
    ```

Create a subnet that will be delegated to just resources within ACI, note that this needs to be an empty subnet, but within the same VNet that you already created. 

```cli
az network vnet subnet create \
    --resource-group $AKS_CLUSTER_RG \
    --vnet-name $VNET_NAME \
    --name $ACI_SUBNET_NAME \
    --address-prefix $ACI_SUBNET_RANGE
```

### Create a service principal

Create an Azure Active Directory service principal to allow AKS to interact with other Azure resources. You can use a pre-created service principal too. 

```cli
az ad sp create-for-rbac --skip-assignment
```

The output should look similar to the following. 
 
```console
{
  "appId": "bef76eb3-d743-4a97-9534-03e9388811fc",
  "displayName": "azure-cli-2018-08-29-22-29-29",
  "name": "http://azure-cli-2018-08-29-22-29-29",
  "password": "1d257915-8714-4ce7-a7fb-0e5a5411df7f",
  "tenant": "72f988bf-86f1-41af-91ab-2d7cd011db48"
}
```
Save the output values from the command output in enviroment variables. 

```cli
export AZURE_TENANT_ID=<Tenant>
export AZURE_CLIENT_ID=<AppId>
export AZURE_CLIENT_SECRET=<Password>
```

Grab the virtual network resource id with the following command:

```cli
az network vnet show --resource-group $AKS_CLUSTER_RG --name $VNET_NAME --query id -o tsv
```

Grant access to the AKS cluster to use the virtual network by creating a role and assigning it. 

```cli
az role assignment create --assignee $AZURE_CLIENT_ID --scope <vnetId> --role Contributor
```

### Create an AKS cluster with a virtual network

Grab the id of the cluster subnet you created earlier with the following command. 

```cli
az network vnet subnet show --resource-group $AKS_CLUSTER_RG --vnet-name $VNET_NAME --name $CLUSTER_SUBNET_NAME --query id -o tsv
```
Save the output in the following enviorment variable. 
```cli 
export VNET_SUBNET_ID=<subnet-id>
```
Use the following command to create an AKS cluster with the virtual network you've already created. Use the enviorment variables from the service principal output, 
```cli
az aks create \
    --resource-group myResourceGroup \
    --name myAKSCluster \
    --node-count 1 \
    --network-plugin azure \
    --service-cidr 10.0.0.0/16 \
    --dns-service-ip 10.0.0.10 \
    --docker-bridge-address 172.17.0.1/16 \
    --vnet-subnet-id $VNET_SUBNET_ID \
    --service-principal $AZURE_CLIENT_ID \
    --client-secret $AZURE_CLIENT_SECRET
```

### Deploy Virtual Kubelet

Manually deploy the Virtual Kubelet node as the instructions say in the [Manual set-up](#deployment-of-the-aci-provider-in-your-cluster)section but when installing the Helm chart use these updated settings. 

```cli
RELEASE_NAME=virtual-kubelet
NODE_NAME=virtual-kubelet
CHART_URL=https://github.com/virtual-kubelet/virtual-kubelet/raw/master/charts/$AKS_VK_RELEASE.tgz

curl https://raw.githubusercontent.com/virtual-kubelet/virtual-kubelet/master/scripts/createCertAndKey.sh > createCertAndKey.sh
chmod +x createCertAndKey.sh
. ./createCertAndKey.sh

helm install "$CHART_URL" --name "$RELEASE_NAME" \
  --set provider=azure \
  --set providers.azure.targetAKS=true \
  --set providers.azure.tenantId=$AZURE_TENANT_ID \
  --set providers.azure.subscriptionId=$AZURE_SUBSCRIPTION_ID \
  --set providers.azure.clientId=$AZURE_CLIENT_ID \
  --set providers.azure.aciVnetSubnetName=$ACI_SUBNET_NAME
  --set apiserverCert=$cert \
  --set apiserverKey=$key
```

## Validate the Virtual Kubelet ACI provider

To validate that the Virtual Kubelet has been installed, return a list of Kubernetes nodes using the [kubectl get nodes][kubectl-get] command. You should see a node that matches the name given to the ACI connector.

```cli
kubectl get nodes
```

Output:

```console
NAME                                        STATUS    ROLES     AGE       VERSION
virtual-kubelet-myconnector-linux           Ready     <none>    2m        v1.8.3
aks-nodepool1-39289454-0                    Ready     agent     22h       v1.7.7
aks-nodepool1-39289454-1                    Ready     agent     22h       v1.7.7
aks-nodepool1-39289454-2                    Ready     agent     22h       v1.7.7
```

## Schedule a pod in ACI

Create a file named `virtual-kubelet-test.yaml` and copy in the following YAML. Replace the `nodeName` value with the name given to the virtual kubelet node.

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: helloworld
spec:
  containers:
  - image: microsoft/aci-helloworld
    imagePullPolicy: Always
    name: helloworld
    resources:
      requests:
        memory: 1G
        cpu: 1
    ports:
    - containerPort: 80
      name: http
      protocol: TCP
    - containerPort: 443
      name: https
  dnsPolicy: ClusterFirst
  tolerations:
  - key: virtual-kubelet.io/provider
    operator: Exists
  - key: azure.com/aci
    effect: NoSchedule
```

Notice that Virtual-Kubelet nodes are tainted by default to avoid unexpected pods running on them, i.e. kube-proxy, other virtual-kubelet pods, etc. To schedule a pod to them, you need to add the tolerations to your pod spec:

```
  tolerations:
  - key: virtual-kubelet.io/provider
    operator: Exists
  - key: azure.com/aci
    effect: NoSchedule
```

Run the application with the [kubectl create][kubectl-create] command.

```cli
kubectl create -f virtual-kubelet-test.yaml
```

Use the [kubectl get pods][kubectl-get] command with the `-o wide` argument to output a list of pods with the scheduled node.

```cli
kubectl get pods -o wide
```

Notice that the `helloworld` pod is running on the `virtual-kubelet` node.

```console
NAME                                            READY     STATUS    RESTARTS   AGE       IP             NODE
aci-helloworld-2559879000-8vmjw                 1/1       Running   0          39s       52.179.3.180   virtual-kubelet

```

To validate that the container is running in an Azure Container Instance, use the [az container list][az-container-list] Azure CLI command.

```cli
az container list -o table
```

Output:

```console
Name                             ResourceGroup    ProvisioningState    Image                     IP:ports         CPU/Memory       OsType    Location
-------------------------------  ---------------  -------------------  ------------------------  ---------------  ---------------  --------  ----------
helloworld-2559879000-8vmjw  myResourceGroup    Succeeded            microsoft/aci-helloworld  52.179.3.180:80  1.0 core/1.5 gb  Linux     eastus
```
<!--
### Schedule an ACI pod with a DNS Name label

Add an annotation to your Pod manifest, `virtualkubelet.io/dnsnamelabel` keyed to what you'd like the Azure Container Instance to receive as a DNS Name, and deploy it.

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: helloworld
  annotations:
    virtualkubelet.io/dnsnamelabel: "helloworld-aci"
spec:
  containers:
  - image: microsoft/aci-helloworld
    imagePullPolicy: Always
    name: helloworld
    resources:
      requests:
        memory: 1G
        cpu: 1
    ports:
    - containerPort: 80
      name: http
      protocol: TCP
    - containerPort: 443
      name: https
  dnsPolicy: ClusterFirst
  tolerations:
  - key: virtual-kubelet.io/provider
    operator: Exists
  - key: azure.com/aci
    effect: NoSchedule
```

To confirm the Azure Container Instance received and bound the DNS Name specified, use the [az container show][az-container-show] Azure CLI command. Virtual Kubelet's naming
convention will affect how you use this query, with the argument to `-n` broken down as: nameSpace-podName. Unless specified, Kubernetes will assume
the namespace is `default`.

```azurecli-interactive
az container show -g myResourceGroup -n default-helloworld --query ipAddress.fqdn
```

Output:

```console
"helloworld-aci.westus.azurecontainer.io"
```
-->

## Upgrade the ACI Connector 

If you've installed Virtual Kubelet with the Azure cli so you're using the ACI Connector implementation, then you are also able to upgrade the connector to the latest release. 
Run the following command to upgrade your ACI Connector. 

```cli
az aks upgrade-connector --resource-group <aks cluster rg> --name <aks cluster name> --connector-name myconnector --os-type linux
```

## Remove the Virtual Kubelet

You can remove your Virtual Kubelet node by deleting the Helm deployment. Run the following command:

```cli
helm delete virtual-kubelet --purge
```
If you used the ACI Connector installation then use the following command to remove the the ACI Connector from your cluster.

```cli
az aks remove-connector --resource-group <aks cluster rg> --name <aks cluster name> --connector-name myconnector --os-type linux
```

<!-- LINKS -->
[kubectl-create]: https://kubernetes.io/docs/user-guide/kubectl/v1.6/#create
[kubectl-get]: https://kubernetes.io/docs/user-guide/kubectl/v1.8/#get
[az-container-list]: https://docs.microsoft.com/en-us/cli/azure/container?view=azure-cli-latest#az_container_list
[az-container-show]: https://docs.microsoft.com/en-us/cli/azure/container?view=azure-cli-latest#az_container_show
