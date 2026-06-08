/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package remotecommand

import (
	"net/http"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/util/httpstream"
)

// waitTimeout bounds how long a test waits for a goroutine signal before
// declaring failure. It is only ever hit on a genuine bug (a never-fired
// cancel or a leaked goroutine), so it can be generous without slowing the
// happy path.
const waitTimeout = time.Second

// TestWatchClose_CancelOnClose verifies that watchClose calls cancel exactly
// once and returns when the connection close channel fires. This is the
// load-bearing case: a hard client drop closes closeCh, and the server-side
// exec/attach context must be cancelled so the provider can reap the remote
// process.
func TestWatchClose_CancelOnClose(t *testing.T) {
	for name, closer := range map[string]func(ch chan bool){
		"closeCh closed":         func(ch chan bool) { close(ch) },
		"closeCh receives value": func(ch chan bool) { ch <- true },
	} {
		t.Run(name, func(t *testing.T) {
			closeCh := make(chan bool)
			execDone := make(chan struct{})

			// cancelled is closed by the cancel func, giving a race-safe
			// happens-before edge instead of a shared bool.
			cancelled := make(chan struct{})
			cancelCount := make(chan struct{}, 8)
			cancel := func() {
				cancelCount <- struct{}{}
				select {
				case <-cancelled:
					// already closed by an earlier call; closing again would panic.
				default:
					close(cancelled)
				}
			}

			returned := make(chan struct{})
			go func() {
				watchClose(closeCh, execDone, cancel)
				close(returned)
			}()

			closer(closeCh)

			select {
			case <-cancelled:
			case <-time.After(waitTimeout):
				t.Fatal("cancel was not called after closeCh fired")
			}

			// watchClose must return after cancelling so its goroutine does not leak.
			select {
			case <-returned:
			case <-time.After(waitTimeout):
				t.Fatal("watchClose did not return after cancelling")
			}

			// cancel must fire exactly once.
			if got := len(cancelCount); got != 1 {
				t.Errorf("expected cancel to be called exactly once, got %d calls", got)
			}
		})
	}
}

// TestWatchClose_ExecDoneFirst_NoCancel verifies that when the exec/attach
// finishes normally (execDone closes first), watchClose returns WITHOUT
// calling cancel. Cancelling here would be a spurious cancellation of a
// connection that is shutting down cleanly.
func TestWatchClose_ExecDoneFirst_NoCancel(t *testing.T) {
	closeCh := make(chan bool)
	execDone := make(chan struct{})

	cancelCount := make(chan struct{}, 8)
	cancel := func() { cancelCount <- struct{}{} }

	returned := make(chan struct{})
	go func() {
		watchClose(closeCh, execDone, cancel)
		close(returned)
	}()

	close(execDone)

	// watchClose must return promptly once execDone fires (no goroutine leak).
	select {
	case <-returned:
	case <-time.After(waitTimeout):
		t.Fatal("watchClose did not return after execDone closed")
	}

	// After the goroutine has provably exited, the cancel count is stable.
	if got := len(cancelCount); got != 0 {
		t.Errorf("expected cancel not to be called when execDone closes first, got %d calls", got)
	}
}

// TestWatchClose_NilCloseCh verifies the websocket-safe path: when closeCh is
// nil (the websocket conn has no CloseChan), watchClose must never select the
// nil channel — it blocks on it harmlessly — and returns only when execDone
// closes, without calling cancel or panicking.
func TestWatchClose_NilCloseCh(t *testing.T) {
	var closeCh chan bool // nil
	execDone := make(chan struct{})

	cancelCount := make(chan struct{}, 8)
	cancel := func() { cancelCount <- struct{}{} }

	returned := make(chan struct{})
	go func() {
		watchClose(closeCh, execDone, cancel)
		close(returned)
	}()

	close(execDone)

	select {
	case <-returned:
	case <-time.After(waitTimeout):
		t.Fatal("watchClose did not return after execDone closed (nil closeCh)")
	}

	if got := len(cancelCount); got != 0 {
		t.Errorf("expected cancel not to be called with a nil closeCh, got %d calls", got)
	}
}

// TestCloseChan verifies the conn -> close-channel extraction gate.
func TestCloseChan(t *testing.T) {
	t.Run("CloseChan-capable conn returns its channel", func(t *testing.T) {
		// fc.CloseChan() returns fc.closeCh; closeChan must hand back THAT
		// channel (identity), so the watcher observes the real connection close.
		fc := newFakeConn()
		got := closeChan(fc)
		if got == nil {
			t.Fatal("expected a non-nil channel from a CloseChan-capable conn")
		}

		// Prove identity: closing the conn's channel must be observable through
		// the channel closeChan returned.
		close(fc.closeCh)
		select {
		case _, ok := <-got:
			if ok {
				t.Error("expected the returned channel to be the conn's CloseChan (closed), but it carried a value")
			}
		case <-time.After(waitTimeout):
			t.Fatal("returned channel is not the conn's CloseChan: it did not observe the close")
		}
	})

	t.Run("plain io.Closer returns nil", func(t *testing.T) {
		// A websocket-style conn that is only an io.Closer (no CloseChan) must
		// yield a nil channel so the watcher never spuriously cancels.
		if got := closeChan(nopCloser{}); got != nil {
			t.Errorf("expected nil channel for a plain io.Closer, got %v", got)
		}
	})
}

// nopCloser is a minimal io.Closer that does NOT implement CloseChan, modeling
// the websocket transport path.
type nopCloser struct{}

func (nopCloser) Close() error { return nil }

// fakeConn implements the full httpstream.Connection interface (including a
// controllable CloseChan) so closeChan's type assertion succeeds against it.
// The template matches internal/kubernetes/portforward/httpstream_test.go.
type fakeConn struct {
	closeCh chan bool
}

func newFakeConn() *fakeConn {
	return &fakeConn{closeCh: make(chan bool)}
}

var _ httpstream.Connection = &fakeConn{}

func (*fakeConn) CreateStream(headers http.Header) (httpstream.Stream, error) { return nil, nil }
func (*fakeConn) Close() error                                                { return nil }
func (f *fakeConn) CloseChan() <-chan bool                                    { return f.closeCh }
func (*fakeConn) SetIdleTimeout(timeout time.Duration)                        {}
func (*fakeConn) RemoveStreams(streams ...httpstream.Stream)                  {}
