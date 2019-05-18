package api

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/cpuguy83/strongerrors"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"github.com/virtual-kubelet/virtual-kubelet/log"
)

// ContainerLogsHandlerFunc is used in place of backend implementations for getting container logs
type ContainerLogsHandlerFunc func(ctx context.Context, namespace, podName, containerName string, opts ContainerLogOpts) (io.ReadCloser, error)

// ContainerLogOpts are used to pass along options to be set on the container
// log stream.
type ContainerLogOpts struct {
	Tail       int
	Since      time.Duration
	LimitBytes int
	Timestamps bool
}

// HandleContainerLogs creates an http handler function from a provider to serve logs from a pod
func HandleContainerLogs(h ContainerLogsHandlerFunc) http.HandlerFunc {
	if h == nil {
		return NotImplemented
	}
	return handleError(func(w http.ResponseWriter, req *http.Request) error {
		vars := mux.Vars(req)
		if len(vars) != 3 {
			return strongerrors.NotFound(errors.New("not found"))
		}

		ctx := req.Context()

		namespace := vars["namespace"]
		pod := vars["pod"]
		container := vars["container"]
		tail := 10
		q := req.URL.Query()

		if queryTail := q.Get("tailLines"); queryTail != "" {
			t, err := strconv.Atoi(queryTail)
			if err != nil {
				return strongerrors.InvalidArgument(errors.Wrap(err, "could not parse \"tailLines\""))
			}
			tail = t
		}

		// TODO(@cpuguy83): support v1.PodLogOptions
		// The kubelet decoding here is not straight forward, so this needs to be disected

		opts := ContainerLogOpts{
			Tail: tail,
		}

		logs, err := h(ctx, namespace, pod, container, opts)
		if err != nil {
			return errors.Wrap(err, "error getting container logs?)")
		}

		defer logs.Close()

		req.Header.Set("Transfer-Encoding", "chunked")

		if _, ok := w.(writeFlusher); !ok {
			log.G(ctx).Debug("http response writer does not support flushes")
		}

		if _, err := io.Copy(flushOnWrite(w), logs); err != nil {
			return strongerrors.Unknown(errors.Wrap(err, "error writing response to client"))
		}
		return nil
	})
}
