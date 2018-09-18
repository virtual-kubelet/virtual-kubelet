package api

import (
	"context"
	"io"
	"net/http"
	"strconv"

	"github.com/cpuguy83/strongerrors"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
)

// ContainerLogsBackend is used in place of backend implementations for getting container logs
type ContainerLogsBackend interface {
	GetContainerLogs(ctx context.Context, namespace, podName, containerName string, tail int) (string, error)
}

// PodLogsHandlerFunc creates an http handler function from a provider to serve logs from a pod
func PodLogsHandlerFunc(p ContainerLogsBackend) http.HandlerFunc {
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

		podsLogs, err := p.GetContainerLogs(ctx, namespace, pod, container, tail)
		if err != nil {
			return errors.Wrap(err, "error getting container logs?)")
		}

		if _, err := io.WriteString(w, podsLogs); err != nil {
			return strongerrors.Unknown(errors.Wrap(err, "error writing response to client"))
		}
		return nil
	})
}
