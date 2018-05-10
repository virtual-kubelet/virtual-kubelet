# Virtual Kubelet CRI Provider

This is a Virtual Kubelet Provider implementation that manages real pods and containers in a CRI-based container runtime. 

## Purpose

The purpose of the CRI Provider is for testing and prototyping ONLY. It is not to be used for any other purpose!

The whole point of the Virtual Kubelet project is to provide an interface for container runtimes that don't conform to the standard node-based model. The [Kubelet](https://github.com/kubernetes/kubernetes/tree/master/pkg/kubelet) codebase is the comprehensive standard CRI node agent and this Provider is not attempting to recreate that.

This Provider implementation should be seen as a bare-bones minimum implementation for making it easier to test the core of the Virtual Kubelet project against real pods and containers - in other words, more comprehensive than MockProvider.

This Provider implementation is also designed such that it can be used for prototyping new architectural features which can be developed against local Linux infrastructure. If the CRI provider can be shown to work successfully within a Linux guest, there can be a much higher degree of confidence that the abstraction should work for other Providers.

## Dependencies

The simplest way to run the CRI provider is to install [containerd 1.1](https://github.com/containerd/containerd/releases), which already has the CRI plugin installed.

## Configuring

* Copy `/etc/kubernetes/admin.conf` from your master node and place it somewhere local to Virtual Kubelet
* Find a `client.crt` and `client.key` that will allow you to authenticate with the API server and copy them somewhere local

## Running

Start containerd
```cli
sudo nohup containerd > /tmp/containerd.out 2>&1 &
```
Create a script that will set up the environment and run Virtual Kubelet with the correct provider
```
#!/bin/bash
export VKUBELET_POD_IP=<IP of the Linux node>
export APISERVER_CERT_LOCATION="/etc/virtual-kubelet/client.crt"
export APISERVER_KEY_LOCATION="/etc/virtual-kubelet/client.key"
export KUBELET_PORT="10250"
cd bin
./virtual-kubelet --provider cri --kubeconfig admin.conf
```
The Provider assumes that the containerd socket is available at `/run/containerd/containerd.sock` which is the default location. It will write container logs at `/var/log/vk-cri/` and mount volumes at `/run/vk-cri/volumes/`. You need to make sure that you run as a user that has permissions to read and write to these locations.

## Limitations

* The CRI provider does everything that the Provider interface currently allows it to do, principally managing the lifecycle of pods, returning logs and very little else.
* It will create emptyDir, configmap and secret volumes as necessary, but won't update configmaps or secrets if they change as this has yet to be implemented in the base
* It does not support any kind of persistent volumes
* It will try to run kube-proxy when it starts and can successfully do that. However, as we transition VK to a model in which it treats services and routing in the abstract, this capability will be refactored as a means of testing that feature.
* Networking should currently be considered non-functional
