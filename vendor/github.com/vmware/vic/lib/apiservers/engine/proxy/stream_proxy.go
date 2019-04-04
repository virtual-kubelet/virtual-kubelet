// Copyright 2016-2018 VMware, Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package proxy

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/go-openapi/strfmt"

	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/pkg/term"

	"github.com/vmware/vic/lib/apiservers/engine/backends/convert"
	"github.com/vmware/vic/lib/apiservers/engine/errors"
	"github.com/vmware/vic/lib/apiservers/portlayer/client"
	"github.com/vmware/vic/lib/apiservers/portlayer/client/containers"
	"github.com/vmware/vic/lib/apiservers/portlayer/client/interaction"
	"github.com/vmware/vic/lib/apiservers/portlayer/client/events"
	"github.com/vmware/vic/pkg/trace"
)

type StreamProxy interface {
	AttachStreams(ctx context.Context, ac *AttachConfig, stdin io.ReadCloser, stdout, stderr io.Writer, autoclose bool) error
	StreamContainerLogs(ctx context.Context, name string, out io.Writer, started chan struct{}, showTimestamps bool, followLogs bool, since int64, tailLines int64) error
	StreamContainerStats(ctx context.Context, config *convert.ContainerStatsConfig) error
	StreamEvents(ctx context.Context, out io.Writer) error
}

type VicStreamProxy struct {
	client *client.PortLayer
}

const (
	attachConnectTimeout  time.Duration = 15 * time.Second //timeout for the connection
	attachAttemptTimeout  time.Duration = 60 * time.Second //timeout before we ditch an attach attempt
	attachPLAttemptDiff   time.Duration = 10 * time.Second
	attachStdinInitString               = "v1c#>"
	archiveStreamBufSize                = 64 * 1024
)

// AttachConfig wraps backend.ContainerAttachConfig and adds other required fields
// Similar to https://github.com/docker/docker/blob/master/container/stream/attach.go
type AttachConfig struct {
	*backend.ContainerAttachConfig

	// ID of the session
	ID string
	// Tells the attach copier that the stream's stdin is a TTY and to look for
	// escape sequences in stdin to detach from the stream.
	// When true the escape sequence is not passed to the underlying stream
	UseTty bool
	// CloseStdin signals that once done, stdin for the attached stream should be closed
	// For example, this would close the attached container's stdin.
	CloseStdin bool
}

func NewStreamProxy(client *client.PortLayer) *VicStreamProxy {
	return &VicStreamProxy{client: client}
}

// AttachStreams takes the the hijacked connections from the calling client and attaches
// them to the 3 streams from the portlayer's rest server.
// stdin, stdout, stderr are the hijacked connection
// autoclose controls whether the underlying client transport will be closed when stdout/stderr
func (s *VicStreamProxy) AttachStreams(ctx context.Context, ac *AttachConfig, stdin io.ReadCloser, stdout, stderr io.Writer, autoclose bool) error {
	op := trace.FromContext(ctx, "")
	defer trace.End(trace.Begin("", op))

	// Cancel will close the child connections.
	var wg, outWg sync.WaitGroup

	if s.client == nil {
		return errors.NillPortlayerClientError("StreamProxy")
	}

	errChan := make(chan error, 3)

	var keys []byte
	var err error
	if ac.DetachKeys != "" {
		keys, err = term.ToBytes(ac.DetachKeys)
		if err != nil {
			return fmt.Errorf("Invalid escape keys (%s) provided", ac.DetachKeys)
		}
	}

	ctx, cancel := context.WithCancel(op)
	defer cancel()

	if ac.UseStdin && autoclose {
		// if we're not autoclosing then we don't want to block waiting for copyStdin to exit
		wg.Add(1)
	}

	if ac.UseStdout {
		wg.Add(1)
		outWg.Add(1)
	}

	if ac.UseStderr {
		wg.Add(1)
		outWg.Add(1)
	}

	// cancel stdin if all output streams are complete
	go func() {
		outWg.Wait()
		cancel()
	}()

	EOForCanceled := func(err error) bool {
		return err != nil && ctx.Err() != context.Canceled && !strings.HasSuffix(err.Error(), SwaggerSubstringEOF)
	}

	if ac.UseStdin {
		go func() {
			if autoclose {
				defer wg.Done()
			}
			err := copyStdIn(ctx, s.client, ac, stdin, keys, autoclose)
			if err != nil {
				op.Errorf("container attach: stdin (%s): %s", ac.ID, err)
			} else {
				op.Infof("container attach: stdin (%s) done", ac.ID)
			}

			// Check for EOF or canceled context. We can only detect EOF by checking the error string returned by swagger :/
			// We check this before calling cancel so that we will be sure to return detach errors before stdout/err exits,
			// even in the !autoclose case.
			if EOForCanceled(err) {
				errChan <- err
			}

			if !ac.CloseStdin || ac.UseTty {
				cancel()
			}
		}()
	}

	if ac.UseStdout {
		go func() {
			defer outWg.Done()
			defer wg.Done()

			err := copyStdOut(ctx, s.client, ac, stdout, attachAttemptTimeout)
			if err != nil {
				op.Errorf("container attach: stdout (%s): %s", ac.ID, err)
			} else {
				op.Infof("container attach: stdout (%s) done", ac.ID)
			}

			// Check for EOF or canceled context. We can only detect EOF by checking the error string returned by swagger :/
			if EOForCanceled(err) {
				errChan <- err
			}
		}()
	}

	if ac.UseStderr {
		go func() {
			defer outWg.Done()
			defer wg.Done()

			err := copyStdErr(ctx, s.client, ac, stderr)
			if err != nil {
				op.Errorf("container attach: stderr (%s): %s", ac.ID, err)
			} else {
				op.Infof("container attach: stderr (%s) done", ac.ID)
			}

			// Check for EOF or canceled context. We can only detect EOF by checking the error string returned by swagger :/
			if EOForCanceled(err) {
				errChan <- err
			}
		}()
	}

	// Wait for all stream copy to exit
	wg.Wait()

	// close the channel so that we don't leak (if there is an error)/or get blocked (if there are no errors)
	close(errChan)

	op.Infof("cleaned up connections to %s", ac.ID)
	for err := range errChan {
		if err != nil {
			// check if we got DetachError
			if _, ok := err.(errors.DetachError); ok {
				op.Infof("Detached from container detected")
				return err
			}

			// If we get here, most likely something went wrong with the port layer API server
			// These errors originate within the go-swagger client itself.
			// Go-swagger returns untyped errors to us if the error is not one that we define
			// in the swagger spec.  Even EOF.  Therefore, we must scan the error string (if there
			// is an error string in the untyped error) for the term EOF.
			op.Errorf("container attach error: %s", err)

			return err
		}
	}

	return nil
}

// StreamContainerLogs reads the log stream from the portlayer rest server and writes
// it directly to the io.Writer that is passed in.
func (s *VicStreamProxy) StreamContainerLogs(ctx context.Context, name string, out io.Writer, started chan struct{}, showTimestamps bool, followLogs bool, since int64, tailLines int64) error {
	op := trace.FromContext(ctx, "")
	defer trace.End(trace.Begin("", op))
	opID := op.ID()

	if s.client == nil {
		return errors.NillPortlayerClientError("StreamProxy")
	}

	close(started)

	params := containers.NewGetContainerLogsParamsWithContext(op).
		WithID(name).
		WithFollow(&followLogs).
		WithTimestamp(&showTimestamps).
		WithSince(&since).
		WithTaillines(&tailLines).
		WithOpID(&opID)
	_, err := s.client.Containers.GetContainerLogs(params, out)
	if err != nil {
		switch err := err.(type) {
		case *containers.GetContainerLogsNotFound:
			return errors.NotFoundError(name)
		case *containers.GetContainerLogsInternalServerError:
			return errors.InternalServerError("Server error from the interaction port layer")
		default:
			//Check for EOF.  Since the connection, transport, and data handling are
			//encapsulated inside of Swagger, we can only detect EOF by checking the
			//error string
			if strings.Contains(err.Error(), SwaggerSubstringEOF) {
				return nil
			}
			return errors.InternalServerError(fmt.Sprintf("Unknown error from the interaction port layer: %s", err))
		}
	}

	return nil
}

// StreamContainerStats will provide a stream of container stats written to the provided
// io.Writer.  Prior to writing to the provided io.Writer there will be a transformation
// from the portLayer representation of stats to the docker format
func (s *VicStreamProxy) StreamContainerStats(ctx context.Context, config *convert.ContainerStatsConfig) error {
	op := trace.FromContext(ctx, "")
	defer trace.End(trace.Begin(config.ContainerID, op))
	opID := op.ID()

	if s.client == nil {
		return errors.NillPortlayerClientError("StreamProxy")
	}

	// create a child context that we control
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	params := containers.NewGetContainerStatsParamsWithContext(op).WithOpID(&opID)
	params.ID = config.ContainerID
	params.Stream = config.Stream

	config.Ctx = ctx
	config.Cancel = cancel

	// create our converter
	containerConverter := convert.NewContainerStats(config)
	// provide the writer for the portLayer and start listening for metrics
	writer := containerConverter.Listen()
	if writer == nil {
		// problem with the listener
		return errors.InternalServerError(fmt.Sprintf("unable to gather container(%s) statistics", config.ContainerID))
	}

	_, err := s.client.Containers.GetContainerStats(params, writer)
	if err != nil {
		switch err := err.(type) {
		case *containers.GetContainerStatsNotFound:
			return errors.NotFoundError(config.ContainerID)
		case *containers.GetContainerStatsInternalServerError:
			return errors.InternalServerError("Server error from the interaction port layer")
		default:
			if ctx.Err() == context.Canceled {
				return nil
			}
			//Check for EOF.  Since the connection, transport, and data handling are
			//encapsulated inside of Swagger, we can only detect EOF by checking the
			//error string
			if strings.Contains(err.Error(), SwaggerSubstringEOF) {
				return nil
			}
			return errors.InternalServerError(fmt.Sprintf("Unknown error from the interaction port layer: %s", err))
		}
	}
	return nil
}

// StreamEvents() handles all swagger interaction to the Portlayer's event manager
//
// Input:
//	context and a io.Writer
func (s *VicStreamProxy) StreamEvents(ctx context.Context, out io.Writer) error {
	op := trace.FromContext(ctx, "")
	defer trace.End(trace.Begin("", op))
	opID := op.ID()

	if s.client == nil {
		return errors.NillPortlayerClientError("StreamProxy")
	}

	params := events.NewGetEventsParamsWithContext(ctx).WithOpID(&opID)
	if _, err := s.client.Events.GetEvents(params, out); err != nil {
		switch err := err.(type) {
		case *events.GetEventsInternalServerError:
			return errors.InternalServerError("Server error from the events port layer")
		default:
			//Check for EOF.  Since the connection, transport, and data handling are
			//encapsulated inside of Swagger, we can only detect EOF by checking the
			//error string
			if strings.Contains(err.Error(), SwaggerSubstringEOF) {
				return nil
			}
			return errors.InternalServerError(fmt.Sprintf("Unknown error from the interaction port layer: %s", err))
		}
	}

	return nil
}


//------------------------------------
// ContainerAttach() Utility Functions
//------------------------------------

func copyStdIn(ctx context.Context, pl *client.PortLayer, ac *AttachConfig, stdin io.ReadCloser, keys []byte, autoclose bool) error {
	op := trace.FromContext(ctx, "")
	defer trace.End(trace.Begin("", op))
	opID := op.ID()

	// Pipe for stdin so we can interject and watch the input streams for detach keys.
	stdinReader, stdinWriter := io.Pipe()
	defer stdinReader.Close()

	var detach bool

	done := make(chan struct{})
	if autoclose {
		go func() {
			// make sure we get out of io.Copy if context is canceled
			select {
			case <-ctx.Done():
				// This will cause the transport to the API client to be shut down, so all output
				// streams will get closed as well.
				// See the closer in container_routes.go:postContainersAttach

				// We're closing this here to disrupt the io.Copy below
				// TODO: seems like we should be providing an io.Copy impl with ctx argument that honors
				// cancelation with the amount of code dedicated to working around it

				// TODO: I think this still leaves a race between closing of the API client transport and
				// copying of the output streams, it's just likely the error will be dropped as the transport is
				// closed when it occurs.
				// We should move away from needing to close transports to interrupt reads.
				stdin.Close()
			case <-done:
			}
		}()
	}

	go func() {
		defer close(done)
		defer stdinWriter.Close()

		// Copy the stdin from the CLI and write to a pipe.  We need to do this so we can
		// watch the stdin stream for the detach keys.
		var err error

		// Write some init bytes into the pipe to force Swagger to make the initial
		// call to the portlayer, prior to any user input in whatever attach client
		// he/she is using.
		op.Debugf("copyStdIn writing primer bytes")
		stdinWriter.Write([]byte(attachStdinInitString))
		if ac.UseTty {
			_, err = copyEscapable(stdinWriter, stdin, keys)
		} else {
			_, err = io.Copy(stdinWriter, stdin)
		}

		if err != nil {
			if _, ok := err.(errors.DetachError); ok {
				op.Infof("stdin detach detected")
				detach = true
			} else {
				op.Errorf("stdin err: %s", err)
			}
		}
	}()

	id := ac.ID

	// Swagger wants an io.reader so give it the reader pipe.  Also, the swagger call
	// to set the stdin is synchronous so we need to run in a goroutine
	setStdinParams := interaction.NewContainerSetStdinParamsWithContext(op).
		WithID(id).
		WithRawStream(stdinReader).
		WithOpID(&opID)

	_, err := pl.Interaction.ContainerSetStdin(setStdinParams)
	<-done

	if ac.CloseStdin && !ac.UseTty {
		// Close the stdin connection.  Mimicing Docker's behavior.
		op.Errorf("Attach stream has stdinOnce set.  Closing the stdin.")
		params := interaction.NewContainerCloseStdinParamsWithContext(ctx).WithID(id)
		_, err := pl.Interaction.ContainerCloseStdin(params)
		if err != nil {
			op.Errorf("CloseStdin failed with %s", err)
		}
	}

	// ignore the portlayer error when it is DetachError as that is what we should return to the caller when we detach
	if detach {
		return errors.DetachError{}
	}

	return err
}

func copyStdOut(ctx context.Context, pl *client.PortLayer, ac *AttachConfig, stdout io.Writer, attemptTimeout time.Duration) error {
	op := trace.FromContext(ctx, "")
	defer trace.End(trace.Begin("", op))
	id := ac.ID

	//Calculate how much time to let portlayer attempt
	plAttemptTimeout := attemptTimeout - attachPLAttemptDiff //assumes personality deadline longer than portlayer's deadline
	plAttemptDeadline := time.Now().Add(plAttemptTimeout)
	swaggerDeadline := strfmt.DateTime(plAttemptDeadline)
	op.Debugf("* stdout portlayer deadline: %s", plAttemptDeadline.Format(time.UnixDate))
	op.Debugf("* stdout personality deadline: %s", time.Now().Add(attemptTimeout).Format(time.UnixDate))

	op.Debugf("* stdout attach start %s", time.Now().Format(time.UnixDate))
	getStdoutParams := interaction.NewContainerGetStdoutParamsWithContext(ctx).WithID(id).WithDeadline(&swaggerDeadline)
	_, err := pl.Interaction.ContainerGetStdout(getStdoutParams, stdout)
	op.Debugf("* stdout attach end %s", time.Now().Format(time.UnixDate))
	if err != nil {
		if _, ok := err.(*interaction.ContainerGetStdoutNotFound); ok {
			return errors.ContainerResourceNotFoundError(id, "interaction connection")
		}

		return errors.InternalServerError(err.Error())
	}

	return nil
}

func copyStdErr(ctx context.Context, pl *client.PortLayer, ac *AttachConfig, stderr io.Writer) error {
	id := ac.ID

	getStderrParams := interaction.NewContainerGetStderrParamsWithContext(ctx).WithID(id)
	_, err := pl.Interaction.ContainerGetStderr(getStderrParams, stderr)
	if err != nil {
		if _, ok := err.(*interaction.ContainerGetStderrNotFound); ok {
			errors.ContainerResourceNotFoundError(id, "interaction connection")
		}

		return errors.InternalServerError(err.Error())
	}

	return nil
}

// FIXME: Move this function to a pkg to show it's origination from Docker once
// we have ignore capabilities in our header-check.sh that checks for copyright
// header.
// Code c/c from io.Copy() modified by Docker to handle escape sequence
// Begin

func copyEscapable(dst io.Writer, src io.ReadCloser, keys []byte) (written int64, err error) {
	if len(keys) == 0 {
		// Default keys : ctrl-p ctrl-q
		keys = []byte{16, 17}
	}
	buf := make([]byte, 32*1024)
	for {
		nr, er := src.Read(buf)
		if nr > 0 {
			// ---- Docker addition
			preservBuf := []byte{}
			for i, key := range keys {
				preservBuf = append(preservBuf, buf[0:nr]...)
				if nr != 1 || buf[0] != key {
					break
				}
				if i == len(keys)-1 {
					src.Close()
					return 0, errors.DetachError{}
				}
				nr, er = src.Read(buf)
			}
			var nw int
			var ew error
			if len(preservBuf) > 0 {
				nw, ew = dst.Write(preservBuf)
				nr = len(preservBuf)
			} else {
				// ---- End of docker
				nw, ew = dst.Write(buf[0:nr])
			}
			if nw > 0 {
				written += int64(nw)
			}
			if ew != nil {
				err = ew
				break
			}
			if nr != nw {
				err = io.ErrShortWrite
				break
			}
		}
		if er == io.EOF {
			break
		}
		if er != nil {
			err = er
			break
		}
	}
	return written, err
}
