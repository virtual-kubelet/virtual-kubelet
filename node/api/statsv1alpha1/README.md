These types are copied from the [k8s.io/kubelet](https://pkg.go.dev/k8s.io/kubelet@v0.21.0/pkg/apis/stats/v1alpha1) module.
They are used from a type alias in the API package.

We want to stop importing k8s.io/kubernetes (where the older type def is) since this transatively imports all of kubernetes.

After the min version is v1.20 we can update the type alias to point the the module and remove these type definitions.