/*
Package node implements the components for operating a node in Kubernetes.
This includes controllers for managin the node object, running scheduled pods,
and exporting HTTP endpoints expected by the Kubernets API server.

There are two primary controllers, the node runner and the pod runner.

	nodeRunner, _ := node.NewNodeController(...)
		// setup other things
	podRunner, _ := node.NewPodController(...)

	go podRunner.Run(ctx)

	select {
	case <-podRunner.Ready():
		go nodeRunner.Run(ctx)
	case <-ctx.Done()
		return ctx.Err()
	}

After calling start, cancelling the passed in context will shutdown the
controller.
Note this example elides error handling.

Up to this point you have an active node in Kubernetes which can have pods scheduled
to it. However the API server expects nodes to implement API endpoints in order
to support certain features such as fetching logs or execing a new process.
The api package provides some helpers for this:
`api.AttachPodRoutes` and `api.AttachMetricsRoutes`.

	mux := http.NewServeMux()
	api.AttachPodRoutes(provider, mux)

You must configure your own HTTP server, but these helpers will add handlers at
the correct URI paths to your serve mux. You are not required to use go's
built-in `*http.ServeMux`, but it does implement the `ServeMux` interface
defined in this package which is used for these helpers.

Note: The metrics routes may need to be attached to a different HTTP server,
depending on your configuration.

For more fine-grained control over the API, see the `node/api` package which
only implements the HTTP handlers that you can use in whatever way you want.

This uses open-cenesus to implement tracing (but no internal metrics yet) which
is propagated through the context. This is passed on even to the providers.
*/
package node
