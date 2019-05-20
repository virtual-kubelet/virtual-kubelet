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
	"k8s.io/apimachinery/pkg/types"
	remoteutils "k8s.io/client-go/tools/remotecommand"
	api "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/kubelet/server/remotecommand"
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

// HandleContainerExec makes an http handler func from a Provider which execs a command in a pod's container
// Note that this handler currently depends on gorrilla/mux to get url parts as variables.
// TODO(@cpuguy83): don't force gorilla/mux on consumers of this function
func HandleContainerExec(h ContainerExecHandlerFunc) http.HandlerFunc {
	if h == nil {
		return NotImplemented
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

		idleTimeout := time.Second * 30
		streamCreationTimeout := time.Second * 30

		ctx, cancel := context.WithCancel(context.TODO())
		defer cancel()

		exec := &containerExecContext{ctx: ctx, h: h, pod: pod, namespace: namespace, container: container}
		remotecommand.ServeExec(w, req, exec, "", "", container, command, streamOpts, idleTimeout, streamCreationTimeout, supportedStreamProtocols)

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

type containerExecContext struct {
	h                         ContainerExecHandlerFunc
	eio                       *execIO
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
