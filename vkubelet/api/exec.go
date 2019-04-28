package api

import (
	"context"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/cpuguy83/strongerrors"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"github.com/virtual-kubelet/virtual-kubelet/providers"
	"k8s.io/apimachinery/pkg/types"
	remoteutils "k8s.io/client-go/tools/remotecommand"
	api "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/kubelet/server/remotecommand"
)

type ExecBackend interface {
	RunInContainer(ctx context.Context, namespace, podName, containerName string, cmd []string, attach providers.AttachIO) error
}

// PodExecHandlerFunc makes an http handler func from a Provider which execs a command in a pod's container
// Note that this handler currently depends on gorrilla/mux to get url parts as variables.
// TODO(@cpuguy83): don't force gorilla/mux on consumers of this function
func PodExecHandlerFunc(backend ExecBackend) http.HandlerFunc {
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

		ctx, cancel := context.WithCancel(context.TODO())
		defer cancel()

		exec := &containerExecContext{ctx: ctx, b: backend, pod: pod, namespace: namespace, container: container}
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
	b                         ExecBackend
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
		eio.chResize = make(chan providers.TermSize)
	}

	ctx, cancel := context.WithCancel(c.ctx)
	defer cancel()

	if tty {
		go func() {
			send := func(s remoteutils.TerminalSize) bool {
				select {
				case eio.chResize <- providers.TermSize{Width: s.Width, Height: s.Height}:
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

	return c.b.RunInContainer(c.ctx, c.namespace, c.pod, c.container, cmd, eio)
}

type execIO struct {
	tty      bool
	stdin    io.Reader
	stdout   io.WriteCloser
	stderr   io.WriteCloser
	chResize chan providers.TermSize
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

func (e *execIO) Resize() <-chan providers.TermSize {
	return e.chResize
}
