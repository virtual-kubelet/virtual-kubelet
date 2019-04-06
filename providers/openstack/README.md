# OpenStack Zun

[OpenStack Zun](https://docs.openstack.org/zun/latest/) is an OpenStack Container service.
It aims to provide an API service for running application containers without the need to
manage servers or clusters.

## OpenStack Zun virtual-kubelet provider

OpenStack Zun virtual-kubelet provider connects your Kubernetes cluster to an OpenStack Cloud.
Your pods on OpenStack have access to OpenStack tenant networks since each pod is given
dedicated Neutron ports in your tenant subnets.

## Prerequisites

You need to have an OpenStack cloud with Zun service installed.
The quickest way to get everything setup is using
[Devstack](https://docs.openstack.org/zun/latest/contributor/quickstart.html).
If it is for production purpose, you follow the
[Zun installation guide](https://docs.openstack.org/zun/latest/install/index.html).
Another alternative is using
[Kolla](https://docs.openstack.org/kolla-ansible/latest/reference/compute/zun-guide.html).

## Authentication via Keystone

Virtual-kubelet needs permission to schedule pods on OpenStack Zun on your behalf.
You will need to retrieve your OpenStack credentials and store them as environment variables.

```console
export OS_DOMAIN_ID=default
export OS_REGION_NAME=RegionOne
export OS_PROJECT_NAME=demo
export OS_IDENTITY_API_VERSION=3
export OS_AUTH_URL=http://10.0.2.15/identity/v3
export OS_USERNAME=demo
export OS_PASSWORD=password
```

For users that have the OpenStack dashboard installed, there's a shortcut. If you visit the
project/access_and_security path in Horizon and click on the "Download OpenStack RC File" button
at the top right hand corner, you will download a bash file that exports all of your access details
to environment variables. To execute the file, run source admin-openrc.sh and you will be prompted
for your password.

## Connecting virtual-kubelet to your Kubernetes cluster

Start the virtual-kubelet process.

```console
virtual-kubelet --provider openstack
```

In your Kubernetes cluster, confirm that the virtual-kubelet shows up as a node.
```console
kubectl get nodes

NAME              STATUS     ROLES    AGE   VERSION
virtual-kubelet   Ready      agent    20d   v1.13.1-vk-N/A
...
```

To disconnect, stop the virtual-kubelet process.

## Deploying Kubernetes pods in OpenStack Zun

In order to not break existing pod deployments, the OpenStack virtual node is given a taint.
Pods that are to be deployed on OpenStack require an explicit toleration that tolerates the
taint of the virtual node.

```
apiVersion: v1
kind: Pod
metadata:
  name: myapp-pod
  labels:
    app: myapp
spec:
  tolerations:
  - key: "virtual-kubelet.io/provider"
    operator: "Equal"
    value: "openstack"
    effect: "NoSchedule"
  containers:
  - name: myapp-container
    image: busybox
    command: ['sh', '-c', 'echo Hello Kubernetes! && sleep 3600']
```
