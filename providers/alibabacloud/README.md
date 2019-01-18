# Alibaba Cloud ECI

<img src="eci.svg" width="200" height="200" />

Alibaba Cloud ECI(Elastic Container Instance) is a service that allow you run containers without having to manage servers or clusters.

You can find more infomation via [alibaba cloud ECI web portal](https://www.aliyun.com/product/eci)

## Alibaba Cloud ECI Virtual-Kubelet Provider
Alibaba ECI provider is an adapter to connect between k8s and ECI service to implement pod from k8s cluster on alibaba cloud platform

## Prerequisites
To using ECI service on alibaba cloud, you may need open ECI service on [web portal](https://www.aliyun.com/product/eci), and then the ECI service will be available

## Deployment of the ECI provider in your cluster
configure and launch virtual kubelet
```
export ECI_REGION=cn-hangzhou
export ECI_SECURITY_GROUP=sg-123
export ECI_VSWITCH=vsw-123
export ECI_ACCESS_KEY=123
export ECI_SECRET_KEY=123

VKUBELET_TAINT_KEY=alibabacloud.com/eci virtual-kubelet --provider alibabacloud
```
confirm the virtual kubelet is connected to k8s cluster
```
$kubectl get node
NAME                                 STATUS                     ROLES     AGE       VERSION
cn-shanghai.i-uf69qodr5ntaxleqdhhk   Ready                      <none>    1d        v1.9.3
virtual-kubelet                      Ready                      agent     10s       v1.8.3
```

## Schedule K8s Pod to ECI via virtual kubelet
You can assign pod to virtual kubelet via node-selector and toleration.
```
apiVersion: v1
kind: Pod
metadata:
  name: mypod
spec:
  nodeName: virtual-kubelet
  containers:
  - name: nginx
    image: nginx
  tolerations:
  - key: alibabacloud.com/eci
    operator: "Exists"
    effect: NoSchedule
```

# Alibaba Cloud Serverless Kubernetes
Alibaba Cloud serverless kubernetes allows you to quickly create kubernetes container applications without
having to manage and maintain clusters and servers.  It is based on ECI and fully compatible with the Kuberentes API.

You can find more infomation via [alibaba cloud serverless kubernetes product doc](https://www.alibabacloud.com/help/doc-detail/94078.htm)

