# Kubernetes initial testing notes
## Required HW setup:
 - Ubuntu 16.04
 - 4 CPU
 - 16GB memory
 - 80GB disk 
 - (might need 8CPU/32GB of memory, as the above recommended was quite slow still)

## Initial install:
  - `sudo apt-get update`
  - `sudo apt-add-repository ppa:juju/stable`
  - `sudo apt-add-repository ppa:conjure-up/next`
  - `sudo apt update`
  - `sudo apt install conjure-up`

## Configure container hypervisor:
  - `newgrp lxd`
  - `sudo lxd init`

Walkthrough the config (just hit enter) - select NO when asked to setup IPv6

## Point juju at the new hypervisor:
  - `juju bootstrap localhost lxd-test`

## Start the k8s cluster:
  - `conjure-up canonical-kubernetes`

Just hit enter a few times until you get to the summary and hit Q

  - `watch -c juju status --color`

Wait for quite a while until the cluster is completely up/active/idle.  Can take upwards of an hour!

## Finalize setup:
  - `mkdir -p ~/.kube`
  - `juju scp kubernetes-master/0:config ~/.kube/config`
  - `juju scp kubernetes-master/0:kubectl ~/bin/kubectl`

## Verify it is working and the cluster is up:
  - `kubectl cluster-info`

## Example commands:
  - `kubectl run -i -t busybox --image=busybox --restart=Never`
  - `kubectl run nginx --image=nginx`

### Show the running pods:
  - `kubectl get pods`

### To scale up the cluster:
  - `juju add-unit kubernetes-worker`

### To show the controller:
  - `juju switch`

### To destroy the cluster:
  - `juju destroy-controller lxd-test --destroy-all-models`
