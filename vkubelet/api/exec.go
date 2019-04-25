package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/cpuguy83/strongerrors"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	api "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/kubelet/server/remotecommand"
)

// PodExecHandlerFunc makes an http handler func from a Provider which execs a command in a pod's container
// Note that this handler currently depends on gorrilla/mux to get url parts as variables.
// TODO(@cpuguy83): don't force gorilla/mux on consumers of this function
func PodExecHandlerFunc(backend remotecommand.Executor) http.HandlerFunc {
	return handleError(func(w http.ResponseWriter, req *http.Request) error {
		vars := mux.Vars(req)

		namespace := vars["namespace"]
		pod := vars["pod"]
		container := vars["container"]

		supportedStreamProtocols := strings.Split(req.Header.Get("X-Stream-Protocol-Version"), ",")

		q := req.URL.Query()
		command := q["command"]

		streamOpts, err := getExecOptions(req)
		if err != nil {
			return strongerrors.InvalidArgument(err)
		}

		idleTimeout := time.Second * 30
		streamCreationTimeout := time.Second * 30

		remotecommand.ServeExec(w, req, backend, fmt.Sprintf("%s-%s", namespace, pod), "", container, command, streamOpts, idleTimeout, streamCreationTimeout, supportedStreamProtocols)
		return nil
	})
}

func getExecOptions(req *http.Request) (*remotecommand.Options, error) {
	tty := req.FormValue(api.ExecTTYParam) == "1"
	stdin := req.FormValue(api.ExecStdinParam) == "1"
	stdout := req.FormValue(api.ExecStdoutParam) == "1"
	stderr := req.FormValue(api.ExecStderrParam) == "1"
	if tty && stderr {
		return nil, errors.New("cannot exec with tty and stderr")
	}

	if !stdin && !stdout && !stderr {
		return nil, errors.New("you must specify at least one of stdin, stdout, stderr")
	}
	return &remotecommand.Options{
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
		TTY:    tty,
	}, nil

}
