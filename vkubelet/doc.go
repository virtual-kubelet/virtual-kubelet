/*
Package vkubelet implements the core virtual-kubelet framework.
It contains everything reuired to implement a virtuak-kubelet, including the
core controller which reconciles pod states and API endpoints for things like
pod logs, exec, attach, etc.

To get started, call the `New` with the appropriate config. When you are ready
to start the controller, which registers the node and starts watching for pod
changes, call `Run`. Taints can be used ensure the sceduler only schedules
certain workloads to your virtual-kubelet.

	vk := vkubelet.New(...)
	// setup other things
	...
	vk.Run(ctx, ...)

After calling start, cancelling the passed in context will shutdown the
controller.

Up to this point you have a running virtual kubelet controller, but no HTTP
handlers to deal with requests forwarded from the API server for things like
pod logs (e.g. user calls `kubectl logs myVKPod`).  The api package provides some
helpers for this: `api.AttachPodRoutes` and `api.AttachMetricsRoutes`.

	mux := http.NewServeMux()
	api.AttachPodRoutes(provider, mux)

You must configure your own HTTP server, but these helpers will add handlers at
the correct URI paths to your serve mux. You are not required to use go's
built-in `*http.ServeMux`, but it does implement the `ServeMux` interface
defined in this package which is used for these helpers.

Note: The metrics routes may need to be attached to a different HTTP server,
depending on your configuration.

For more fine-grained control over the API, see the `vkubelet/api` package which
only implements the HTTP handlers that you can use in whatever way you want.

This uses open-cenesus to implement tracing (but no internal metrics yet) which
is propagated through the context. This is passed on even to the providers. We
may look at supporting custom propagaters for providers who would like to use a
different tracing format.
*/
package vkubelet
