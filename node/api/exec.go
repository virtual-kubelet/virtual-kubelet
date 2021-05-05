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
	"github.com/pkg/errors"
	"github.com/virtual-kubelet/virtual-kubelet/errdefs"
	"github.com/virtual-kubelet/virtual-kubelet/internal/kubernetes/remotecommand"
	"k8s.io/apimachinery/pkg/types"
	remoteutils "k8s.io/client-go/tools/remotecommand"
)

// ContainerExecHandlerFunc defines the handler function used for "execing" into a
// container in a pod.
type ContainerExecHandlerFunc func(ctx context.Context, namespace, podName, containerName string, cmd []string, attach AttachIO) error

// AttachIO is used to pass in streams to attach to a container process
type AttachIO interface {
	Stdin() io.Reader
	Stdout() io.WriteCloser
	Stderr() io.WriteCloser
	TTY() bool
	Resize() <-chan TermSize
}

// TermSize is used to set the terminal size from attached clients.
type TermSize struct {
	Width  uint16
	Height uint16
}

// ContainerExecHandlerConfig is used to pass options to options to the container exec handler.
type ContainerExecHandlerConfig struct {
	// StreamIdleTimeout is the maximum time a streaming connection
	// can be idle before the connection is automatically closed.
	StreamIdleTimeout time.Duration
	// StreamCreationTimeout is the maximum time for streaming connection
	StreamCreationTimeout time.Duration
}

// ContainerExecHandlerOption configures a ContainerExecHandlerConfig
// It is used as functional options passed to `HandleContainerExec`
type ContainerExecHandlerOption func(*ContainerExecHandlerConfig)

// WithExecStreamIdleTimeout sets the idle timeout for a container exec stream
func WithExecStreamIdleTimeout(dur time.Duration) ContainerExecHandlerOption {
	return func(cfg *ContainerExecHandlerConfig) {
		cfg.StreamIdleTimeout = dur
	}
}

// WithExecStreamCreationTimeout sets the creation timeout for a container exec stream
func WithExecStreamCreationTimeout(dur time.Duration) ContainerExecHandlerOption {
	return func(cfg *ContainerExecHandlerConfig) {
		cfg.StreamCreationTimeout = dur
	}
}

// HandleContainerExec makes an http handler func from a Provider which execs a command in a pod's container
// Note that this handler currently depends on gorrilla/mux to get url parts as variables.
// TODO(@cpuguy83): don't force gorilla/mux on consumers of this function
func HandleContainerExec(h ContainerExecHandlerFunc, opts ...ContainerExecHandlerOption) http.HandlerFunc {
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

		q := req.URL.Query()
		command := q["command"]

		streamOpts, err := getExecOptions(req)
		if err != nil {
			return errdefs.AsInvalidInput(err)
		}

		// TODO: Why aren't we using req.Context() here?
		ctx, cancel := context.WithCancel(context.TODO())
		defer cancel()

		exec := &containerExecContext{ctx: ctx, h: h, pod: pod, namespace: namespace, container: container}
		remotecommand.ServeExec(
			w,
			req,
			exec,
			"",
			"",
			container,
			command,
			streamOpts,
			cfg.StreamIdleTimeout,
			cfg.StreamCreationTimeout,
			supportedStreamProtocols,
		)

		return nil
	})
}

const (
	execTTYParam    = "tty"
	execStdinParam  = "input"
	execStdoutParam = "output"
	execStderrParam = "error"
)

func getExecOptions(req *http.Request) (*remotecommand.Options, error) {
	tty := req.FormValue(execTTYParam) == "1"
	stdin := req.FormValue(execStdinParam) == "1"
	stdout := req.FormValue(execStdoutParam) == "1"
	stderr := req.FormValue(execStderrParam) == "1"
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

type containerExecContext struct {
	h                         ContainerExecHandlerFunc
	namespace, pod, container string
	ctx                       context.Context
}

// ExecInContainer Implements remotecommand.Executor
// This is called by remotecommand.ServeExec
func (c *containerExecContext) ExecInContainer(name string, uid types.UID, container string, cmd []string, in io.Reader, out, err io.WriteCloser, tty bool, resize <-chan remoteutils.TerminalSize, timeout time.Duration) error {

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

	return c.h(c.ctx, c.namespace, c.pod, c.container, cmd, eio)
}

type execIO struct {
	tty      bool
	stdin    io.Reader
	stdout   io.WriteCloser
	stderr   io.WriteCloser
	chResize chan TermSize
}

func (e *execIO) TTY() bool {
	return e.tty
}

func (e *execIO) Stdin() io.Reader {
	return e.stdin
}

func (e *execIO) Stdout() io.WriteCloser {
	return e.stdout
}

func (e *execIO) Stderr() io.WriteCloser {
	return e.stderr
}

func (e *execIO) Resize() <-chan TermSize {
	return e.chResize
}
