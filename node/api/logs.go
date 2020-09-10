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

package api

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"github.com/virtual-kubelet/virtual-kubelet/errdefs"
	"github.com/virtual-kubelet/virtual-kubelet/log"
)

// ContainerLogsHandlerFunc is used in place of backend implementations for getting container logs
type ContainerLogsHandlerFunc func(ctx context.Context, namespace, podName, containerName string, opts ContainerLogOpts) (io.ReadCloser, error)

// ContainerLogOpts are used to pass along options to be set on the container
// log stream.
type ContainerLogOpts struct {
	Tail         int
	LimitBytes   int
	Timestamps   bool
	Follow       bool
	Previous     bool
	SinceSeconds int
	SinceTime    time.Time
}

func parseLogOptions(q url.Values) (opts ContainerLogOpts, err error) {
	if tailLines := q.Get("tailLines"); tailLines != "" {
		opts.Tail, err = strconv.Atoi(tailLines)
		if err != nil {
			return opts, errdefs.AsInvalidInput(errors.Wrap(err, "could not parse \"tailLines\""))
		}
		if opts.Tail < 0 {
			return opts, errdefs.InvalidInputf("\"tailLines\" is %d", opts.Tail)
		}
	}
	if follow := q.Get("follow"); follow != "" {
		opts.Follow, err = strconv.ParseBool(follow)
		if err != nil {
			return opts, errdefs.AsInvalidInput(errors.Wrap(err, "could not parse \"follow\""))
		}
	}
	if limitBytes := q.Get("limitBytes"); limitBytes != "" {
		opts.LimitBytes, err = strconv.Atoi(limitBytes)
		if err != nil {
			return opts, errdefs.AsInvalidInput(errors.Wrap(err, "could not parse \"limitBytes\""))
		}
		if opts.LimitBytes < 1 {
			return opts, errdefs.InvalidInputf("\"limitBytes\" is %d", opts.LimitBytes)
		}
	}
	if previous := q.Get("previous"); previous != "" {
		opts.Previous, err = strconv.ParseBool(previous)
		if err != nil {
			return opts, errdefs.AsInvalidInput(errors.Wrap(err, "could not parse \"previous\""))
		}
	}
	if sinceSeconds := q.Get("sinceSeconds"); sinceSeconds != "" {
		opts.SinceSeconds, err = strconv.Atoi(sinceSeconds)
		if err != nil {
			return opts, errdefs.AsInvalidInput(errors.Wrap(err, "could not parse \"sinceSeconds\""))
		}
		if opts.SinceSeconds < 1 {
			return opts, errdefs.InvalidInputf("\"sinceSeconds\" is %d", opts.SinceSeconds)
		}
	}
	if sinceTime := q.Get("sinceTime"); sinceTime != "" {
		opts.SinceTime, err = time.Parse(time.RFC3339, sinceTime)
		if err != nil {
			return opts, errdefs.AsInvalidInput(errors.Wrap(err, "could not parse \"sinceTime\""))
		}
		if opts.SinceSeconds > 0 {
			return opts, errdefs.InvalidInput("both \"sinceSeconds\" and \"sinceTime\" are set")
		}
	}
	if timestamps := q.Get("timestamps"); timestamps != "" {
		opts.Timestamps, err = strconv.ParseBool(timestamps)
		if err != nil {
			return opts, errdefs.AsInvalidInput(errors.Wrap(err, "could not parse \"timestamps\""))
		}
	}
	return opts, nil
}

// HandleContainerLogs creates an http handler function from a provider to serve logs from a pod
func HandleContainerLogs(h ContainerLogsHandlerFunc) http.HandlerFunc {
	if h == nil {
		return NotImplemented
	}
	return handleError(func(w http.ResponseWriter, req *http.Request) error {
		vars := mux.Vars(req)
		if len(vars) != 3 {
			return errdefs.NotFound("not found")
		}

		ctx := req.Context()

		namespace := vars["namespace"]
		pod := vars["pod"]
		container := vars["container"]

		query := req.URL.Query()
		opts, err := parseLogOptions(query)
		if err != nil {
			return err
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
			return errors.Wrap(err, "error writing response to client")
		}
		return nil
	})
}
