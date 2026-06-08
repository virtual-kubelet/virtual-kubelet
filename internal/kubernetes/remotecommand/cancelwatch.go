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

import "io"

// watchClose cancels an exec/attach context when the underlying connection's
// close channel fires (a client disconnect or the stream idle-timeout), and
// returns without cancelling when the exec/attach completes first (execDone).
// A nil closeCh (e.g. the websocket transport, which has no CloseChan) is never
// selected, so the watcher simply waits for execDone — never spuriously
// cancelling. The goroutine always terminates once either channel fires.
func watchClose(closeCh <-chan bool, execDone <-chan struct{}, cancel func()) {
	select {
	case <-closeCh:
		cancel()
	case <-execDone:
	}
}

// closeChan returns the connection's close channel when conn exposes one (the
// SPDY httpstream.Connection), or nil otherwise (the websocket conn is only an
// io.Closer). Returning nil keeps the watcher inert on transports that cannot
// report a mid-stream client drop, and avoids widening the context.conn field
// (which would break the websocket assignment at compile time).
func closeChan(conn io.Closer) <-chan bool {
	if c, ok := conn.(interface{ CloseChan() <-chan bool }); ok {
		return c.CloseChan()
	}
	return nil
}
