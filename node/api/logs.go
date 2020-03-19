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
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"github.com/virtual-kubelet/virtual-kubelet/errdefs"
	"github.com/virtual-kubelet/virtual-kubelet/log"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	"k8s.io/kubernetes/pkg/apis/core/v1/validation"
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

func init() {
	legacyscheme.Scheme.AddKnownTypes(v1.SchemeGroupVersion, &v1.PodLogOptions{})
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
		q := req.URL.Query()

		var (
			opts *ContainerLogOpts
			err  error
		)
		if opts, err = formatContainerLogOpts(q); err != nil {
			return err
		}

		logs, err := h(ctx, namespace, pod, container, *opts)
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

// formatContainerLogOpts formats the logs options
func formatContainerLogOpts(query url.Values) (*ContainerLogOpts, error) {

	// container logs on the kubelet are locked to the v1 API version of PodLogOptions
	logOptions := &v1.PodLogOptions{}
	if err := legacyscheme.ParameterCodec.DecodeParameters(query, v1.SchemeGroupVersion, logOptions); err != nil {
		return nil, fmt.Errorf("unable to decode query, error: %v", err)
	}

	logOptions.TypeMeta = metav1.TypeMeta{}
	if errs := validation.ValidatePodLogOptions(logOptions); len(errs) > 0 {
		return nil, fmt.Errorf("invalid request, error: %v", errs.ToAggregate())
	}

	opts := &ContainerLogOpts{
		Timestamps: logOptions.Timestamps,
	}
	if logOptions.TailLines != nil {
		opts.Tail = int(*logOptions.TailLines)
	}
	if logOptions.SinceSeconds != nil {
		opts.Since = time.Duration(*logOptions.SinceSeconds) * time.Second
	}
	if logOptions.LimitBytes != nil {
		opts.LimitBytes = int(*logOptions.LimitBytes)
	}

	return opts, nil
}
