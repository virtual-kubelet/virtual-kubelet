Web provider for Virtual Kubelet
================================

Virtual Kubelet providers are written using the Go programming language. While
Go is a great general purpose programming language, it is however a fact that
other programming languages exist. The problem that Virtual Kubelet solves is as
applicable to applications written in those languages as it is for those written
using Go. This provider aims to serve as a bridge between technology stacks and
programming languages, as it were, by adapting the Virtual Kubelet provider
interface using a web endpoint, i.e., this provider is a thin layer that
forwards all calls that Kubernetes makes to the virtual kubelet to a
pre-configured HTTP endpoint. This frees the provider's implementor to write
their code in any programming language and technology stack as they see fit.

The `providers/web/web-rust` folder contains a sample provider implemented in
the Rust programming language. Here's a diagram that depicts the interaction
between Kubernetes, the virtual kubelet web provider and the Rust app.

    +----------------+         +---------------------------+          +------------------------------+
    |                |         |                           |   HTTP   |                              |
    |   Kubernetes   | <-----> |   Virtual Kubelet: Web    | <------> |   Provider written in Rust   |
    |                |         |                           |          |                              |
    +----------------+         +---------------------------+          +------------------------------+

Provider interface
------------------

The web provider uses an environment variable to determine the endpoint to use
for forwarding requests. The environment variable must be named
`WEB_ENDPOINT_URL` and must implement the following HTTP API:

|       Path        |  Verb  |                  Query                  | Request  |                     Response                      |                                Description                                |
|-------------------|--------|-----------------------------------------|----------|---------------------------------------------------|---------------------------------------------------------------------------|
| /createPod        | POST   | -                                       | Pod JSON | HTTP status code                                  | Create a new pod                                                          |
| /updatePod        | PUT    | -                                       | Pod JSON | HTTP status code                                  | Update pod spec                                                           |
| /deletePod        | DELETE | -                                       | Pod JSON | HTTP status code                                  | Delete an existing pod                                                    |
| /getPod           | GET    | namespace, name                         | -        | Pod JSON                                          | Given a pod namespace and name, return the pod JSON                       |
| /getContainerLogs | GET    | namespace, podName, containerName, tail | -        | Container logs                                    | Given the namespace, pod name and container name, return `tail` log lines |
| /getPodStatus     | GET    | namespace, name                         | -        | Pod status JSON                                   | Given a pod namespace and name, return the pod's status JSON              |
| /getPods          | GET    | -                                       | -        | Array of pod JSON strings                         | Fetch list of created pods                                                |
| /capacity         | GET    | -                                       | -        | JSON map containing resource name and values      | Fetch resource capacity values                                            |
| /nodeConditions   | GET    | -                                       | -        | Array of node condition JSON strings              | Get list of node conditions (Ready, OutOfDisk etc)                        |
| /nodeAddresses    | GET    | -                                       | -        | Array of node address values (type/address pairs) | Fetch a list of addresses for the node status                             |

A typical deployment configuration for this setup would be to have the provider
implementation be deployed as a container in the same pod as the virtual kubelet
itself (as a "sidecar").

Take her for a spin
-------------------

A sample web provider implementation is included in this repo in order to
showcase what this enables. The sample has been implemented in
[Rust](http://rust-lang.org). The easiest way to get this up and running is
to use the Helm chart available at `providers/web/charts/web-rust`. Open a
terminal and install the chart like so:

    $ cd providers/web/charts
    $ helm install -n web-provider ./web-rust

    $ kubectl get pods
    NAME                                                READY     STATUS    RESTARTS   AGE
    web-provider-virtual-kubelet-web-6b5b7446f6-279xl   2/2       Running   0          3h

If you list the nodes in the cluster after this you should see something that
looks like this:

    $ kubectl get nodes
    NAME                       STATUS    ROLES     AGE       VERSION
    aks-nodepool1-35187879-0   Ready     agent     37d       v1.8.2
    aks-nodepool1-35187879-1   Ready     agent     37d       v1.8.2
    aks-nodepool1-35187879-3   Ready     agent     37d       v1.8.2
    virtual-kubelet-web        Ready     agent     3h        v1.8.3

In case the name of the node didn't give it away, the last entry in the output
above is the virtual kubelet. If you try to list the containers in the pod that
represents the virtual kubelet you should be able to see the sidecar Rust
container:

    $ kubectl get pods -o=custom-columns=NAME:.metadata.name,CONTAINERS:.spec.containers[*].name
    NAME                                                CONTAINERS
    web-provider-virtual-kubelet-web-6b5b7446f6-279xl   webrust,virtualkubelet

In the output above, `webrust` is the sidecar container and `virtualkubelet` is
the broker that forwards requests to `webrust`. You can run a query on the
`/getPods` HTTP endpoint on the `webrust` container to see a list of the pods
that it has been asked to create. To do this we first use `kubectl` to setup a
port forwarding server like so:

    $ kubectl port-forward web-provider-virtual-kubelet-web-6b5b7446f6-279xl 3000:3000

Now if we run `curl` on the `http://localhost:3000/getPods` URL you should see
the pod JSON getting dumped to the terminal. I ran my test on a Kubernetes
cluster deployed on [Azure](https://docs.microsoft.com/en-us/azure/aks/) which
happens to deploy a daemonset with some, what I imagine are "system" pods to
every node in the cluster. You can filter the output to see just the pod names
using the [jq](https://stedolan.github.io/jq/) tool like so:

    $ curl -s http://localhost:3000/getPods | jq '.[] | { name: .metadata.name }'
    {
        "name": "kube-proxy-czz57"
    }
    {
        "name": "kube-svc-redirect-7qlpd"
    }

You can deploy workloads to the virtual kubelet as you normally do. Here's a
sample pod spec that uses `nodeSelector` to cause the deployment to be scheduled
on the virtual kubelet.

    apiVersion: v1
    kind: Pod
    metadata:
            name: vk-pod
            labels:
                    foo: bar
    spec:
            containers:
            - name: web1
            image: nginx
            nodeSelector:
                    type: virtual-kubelet

Let's go ahead and deploy the pod and run our `/getPods` query again:

    $ kubectl apply -f ~/tmp/pod1.yaml
    pod "vk-pod" created

    $ curl -s http://localhost:3000/getPods | jq '.[] | { name: .metadata.name }'
    {
        "name": "kube-proxy-czz57"
    }
    {
        "name": "kube-svc-redirect-7qlpd"
    }
    {
        "name": "vk-pod"
    }

    $ kubectl get pods
    NAME                                                READY     STATUS    RESTARTS   AGE
    vk-pod                                              0/1       Running   0          1m
    web-provider-virtual-kubelet-web-6b5b7446f6-279xl   2/2       Running   6          4h

As you can tell, a new pod has been scheduled to run on our virtual kubelet
instance. Deleting pods works as one would expect:

    $ kubectl delete -f ~/tmp/pod1.yaml
    pod "vk-pod" deleted

    $ curl -s http://localhost:3000/getPods | jq '.[] | { name: .metadata.name }'
    {
        "name": "kube-proxy-czz57"
    }
    {
        "name": "kube-svc-redirect-7qlpd"
    }
