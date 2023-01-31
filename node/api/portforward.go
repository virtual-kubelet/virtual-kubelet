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
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/virtual-kubelet/virtual-kubelet/internal/kubernetes/portforward"
	"k8s.io/apimachinery/pkg/types"
)

// PortForwardHandlerFunc defines the handler function used to
// portforward, passing through the original dataStream
type PortForwardHandlerFunc func(ctx context.Context, namespace, pod string, port int32, stream io.ReadWriteCloser) error

// PortForwardHandlerConfig is used to pass options to options to the container exec handler.
type PortForwardHandlerConfig struct {
	// StreamIdleTimeout is the maximum time a streaming connection
	// can be idle before the connection is automatically closed.
	StreamIdleTimeout time.Duration
	// StreamCreationTimeout is the maximum time for streaming connection
	StreamCreationTimeout time.Duration
}

// PortForwardHandlerOption configures a PortForwardHandlerConfig
// It is used as functional options passed to `HandlePortForward`
type PortForwardHandlerOption func(*PortForwardHandlerConfig)

// WithPortForwardStreamIdleTimeout sets the idle timeout for a container port forward streaming
func WithPortForwardStreamIdleTimeout(dur time.Duration) PortForwardHandlerOption {
	return func(cfg *PortForwardHandlerConfig) {
		cfg.StreamIdleTimeout = dur
	}
}

// WithPortForwardCreationTimeout sets the creation timeout for a container exec stream
func WithPortForwardCreationTimeout(dur time.Duration) PortForwardHandlerOption {
	return func(cfg *PortForwardHandlerConfig) {
		cfg.StreamCreationTimeout = dur
	}
}

// HandlePortForward makes an http handler func from a Provider which forward ports to a container
// Note that this handler currently depends on gorrilla/mux to get url parts as variables.
func HandlePortForward(h PortForwardHandlerFunc, opts ...PortForwardHandlerOption) http.HandlerFunc {
	if h == nil {
		return NotImplemented
	}

	var cfg PortForwardHandlerConfig
	for _, o := range opts {
		o(&cfg)
	}

	if cfg.StreamIdleTimeout == 0 {
		cfg.StreamIdleTimeout = 30 * time.Second
	}
	if cfg.StreamCreationTimeout == 0 {
		cfg.StreamCreationTimeout = 30 * time.Second
	}

	return handleError(func(w http.ResponseWriter, req *http.Request) error {
		vars := mux.Vars(req)

		namespace := vars["namespace"]

		pod := vars["pod"]

		supportedStreamProtocols := strings.Split(req.Header.Get("X-Stream-Protocol-Version"), ",")

		portfwd := &portForwardContext{h: h, pod: pod, namespace: namespace}
		portforward.ServePortForward(
			w,
			req,
			portfwd,
			pod,
			"",
			&portforward.V4Options{}, // This is only used for websocket connection
			cfg.StreamIdleTimeout,
			cfg.StreamCreationTimeout,
			supportedStreamProtocols,
		)

		return nil
	})

}

type portForwardContext struct {
	h         PortForwardHandlerFunc
	pod       string
	namespace string
}

// PortForward Implements portforward.Portforwarder
// This is called by portforward.ServePortForward
func (p *portForwardContext) PortForward(ctx context.Context, name string, uid types.UID, port int32, stream io.ReadWriteCloser) error {
	return p.h(ctx, p.namespace, p.pod, port, stream)
}
