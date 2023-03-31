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
	"github.com/virtual-kubelet/virtual-kubelet/errdefs"
	"github.com/virtual-kubelet/virtual-kubelet/internal/kubernetes/remotecommand"
	"k8s.io/apimachinery/pkg/types"
	remoteutils "k8s.io/client-go/tools/remotecommand"
)

// ContainerAttachHandlerFunc defines the handler function used for "execing" into a
// container in a pod.
type ContainerAttachHandlerFunc func(ctx context.Context, namespace, podName, containerName string, attach AttachIO) error

// HandleContainerAttach makes an http handler func from a Provider which execs a command in a pod's container
// Note that this handler currently depends on gorrilla/mux to get url parts as variables.
// TODO(@cpuguy83): don't force gorilla/mux on consumers of this function
func HandleContainerAttach(h ContainerAttachHandlerFunc, opts ...ContainerExecHandlerOption) http.HandlerFunc {
	if h == nil {
		return NotImplemented
	}

	var cfg ContainerExecHandlerConfig
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
		container := vars["container"]

		supportedStreamProtocols := strings.Split(req.Header.Get("X-Stream-Protocol-Version"), ",")

		streamOpts, err := getExecOptions(req)
		if err != nil {
			return errdefs.AsInvalidInput(err)
		}

		ctx, cancel := context.WithCancel(req.Context())
		defer cancel()

		attach := &containerAttachContext{ctx: ctx, h: h, pod: pod, namespace: namespace, container: container}
		remotecommand.ServeAttach(
			w,
			req,
			attach,
			"",
			"",
			container,
			streamOpts,
			cfg.StreamIdleTimeout,
			cfg.StreamCreationTimeout,
			supportedStreamProtocols,
		)

		return nil
	})
}

type containerAttachContext struct {
	h                         ContainerAttachHandlerFunc
	namespace, pod, container string
	ctx                       context.Context
}

// AttachToContainer Implements remotecommand.Attacher
// This is called by remotecommand.ServeAttach
func (c *containerAttachContext) AttachToContainer(name string, uid types.UID, container string, in io.Reader, out, err io.WriteCloser, tty bool, resize <-chan remoteutils.TerminalSize, timeout time.Duration) error {

	eio := &execIO{
		tty:    tty,
		stdin:  in,
		stdout: out,
		stderr: err,
	}

	if tty {
		eio.chResize = make(chan TermSize)
	}

	ctx, cancel := context.WithCancel(c.ctx)
	defer cancel()

	if tty {
		go func() {
			send := func(s remoteutils.TerminalSize) bool {
				select {
				case eio.chResize <- TermSize{Width: s.Width, Height: s.Height}:
					return false
				case <-ctx.Done():
					return true
				}
			}

			for {
				select {
				case s := <-resize:
					if send(s) {
						return
					}
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	return c.h(c.ctx, c.namespace, c.pod, c.container, eio)
}
