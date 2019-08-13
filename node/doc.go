// Copyright © 2017 The virtual-kubelet authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

/*
Package node implements the components for operating a node in Kubernetes.
This includes controllers for managing the node object, running scheduled pods,
and exporting HTTP endpoints expected by the Kubernetes API server.

There are two primary controllers, the node runner and the pod runner.

	nodeRunner, _ := node.NewNodeController(...)
		// setup other things
	podRunner, _ := node.NewPodController(...)

	go podRunner.Run(ctx)

	select {
	case <-podRunner.Ready():
	case <-podRunner.Done():
	}
	if podRunner.Err() != nil {
		// handle error
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
