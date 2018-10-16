package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"k8s.io/kubernetes/pkg/kubelet/server/remotecommand"
)

// PodExecHandlerFunc makes an http handler func from a Provider which execs a command in a pod's container
// Note that this handler currently depends on gorrilla/mux to get url parts as variables.
// TODO(@cpuguy83): don't force gorilla/mux on consumers of this function
func PodExecHandlerFunc(backend remotecommand.Executor) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)

		namespace := vars["namespace"]
		pod := vars["pod"]
		container := vars["container"]

		supportedStreamProtocols := strings.Split(req.Header.Get("X-Stream-Protocol-Version"), ",")

		q := req.URL.Query()
		command := q["command"]

		// TODO: tty flag causes remotecommand.createStreams to wait for the wrong number of streams
		streamOpts := &remotecommand.Options{
			Stdin:  true,
			Stdout: true,
			Stderr: true,
			TTY:    false,
		}

		idleTimeout := time.Second * 30
		streamCreationTimeout := time.Second * 30

		remotecommand.ServeExec(w, req, backend, fmt.Sprintf("%s-%s", namespace, pod), "", container, command, streamOpts, idleTimeout, streamCreationTimeout, supportedStreamProtocols)
	}
}
