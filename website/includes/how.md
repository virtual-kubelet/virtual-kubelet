Unlike normal Kubernetes [kubelets](https://kubernetes.io/docs/reference/command-line-tools-reference/kubelet/), **Virtual Kubelets** masquerade as kubelets, allowing Kubernetes [Nodes](https://kubernetes.io/docs/concepts/architecture/nodes/) to be backed by any compute resource.

The primary use case for Virtual Kubelet is enabling the extension of the Kubernetes API into [serverless](https://en.wikipedia.org/wiki/Serverless_computing) container platforms.
