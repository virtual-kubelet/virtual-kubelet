/*
Package virtualkubelet is currently just for providing docs for godoc.

Virtual Kubelet is a project which aims to provide a library that can be
consumed by other projects to build a Kubernetes node agent that performs the
same basic role as the Kubelet, but fully customize the behavior.

*Note*: Virtual Kubelet is not the Kubelet.

All of the business logic for virtual-kubelet is in the `node` package. The
node package has controllers for managing the node in Kubernetes and running
scheduled pods against a backend service. The backend service along with the
code wrapping what is provided in the node package is what consumers of this
project would implement. In the interest of not duplicating examples, please
see that package on how to get started using virtual kubelet.

Virtual Kubelet supports propagation of logging and traces through a context.
See the "log" and "trace" packages for how to use this.

Errors produced by and consumed from the node package are expected to conform to
error types defined in the "errdefs" package in order to be able to understand
the kind of failure that occurred and react accordingly.
*/
package virtualkubelet
