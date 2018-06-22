// Copyright 2016 VMware, Inc. All Rights Reserved.
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

package trace

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"sync/atomic"
	"time"

	"github.com/Sirupsen/logrus"

	"github.com/vmware/govmomi/vim25/types"
)

type OperationKey string

const OpTraceKey OperationKey = "traceKey"

var opIDPrefix = os.Getpid()

// opCount is a monotonic counter which increments on Start()
var opCount uint64

type Operation struct {
	context.Context
	operation

	// Logger is used to configure an Operation-specific destination for log messages, in addition
	// to the global logger. This logger is passed to any children which are created.
	Logger *logrus.Logger
}

type operation struct {
	t  []Message
	id string
}

func newOperation(ctx context.Context, id string, skip int, msg string) Operation {
	op := operation{

		// Can be used to trace based on this number which is unique per chain
		// of operations
		id: id,

		// Start the trace.
		t: []Message{*newTrace(msg, skip, id)},
	}

	// We need to be able to identify this operation across API (and process)
	// boundaries.  So add the trace as a value to the embedded context.  We
	// stash the values individually in the context because we can't assign
	// the operation itself as a value to the embedded context (it's circular)
	ctx = context.WithValue(ctx, OpTraceKey, op)

	// By adding the op.id any operations passed to govmomi will result
	// in the op.id being logged in vSphere (vpxa / hostd) as the prefix to opID
	// For example if the op.id was 299.16 hostd would show
	// verbose hostd[12281B70] [Originator@6876 sub=PropertyProvider opID=299.16-5b05 user=root]
	ctx = context.WithValue(ctx, types.ID{}, op.id)

	o := Operation{
		Context:   ctx,
		operation: op,
	}

	return o
}

// Creates a header string to be printed.
func (o *Operation) header() string {
	return fmt.Sprintf("op=%s", o.id)
}

// Err returns a non-nil error value after Done is closed.  Err returns
// Canceled if the context was canceled or DeadlineExceeded if the
// context's deadline passed.  No other values for Err are defined.
// After Done is closed, successive calls to Err return the same value.
func (o Operation) Err() error {

	// Walk up the contexts from which this context was created and get their errors
	if err := o.Context.Err(); err != nil {
		buf := &bytes.Buffer{}

		// Add a frame for this Err call, then walk the stack
		currFrame := newTrace("Err", 2, o.id)
		fmt.Fprintf(buf, "%s: %s error: %s\n", currFrame.funcName, o.t[0].msg, err)

		// handle the carriage return
		numFrames := len(o.t)

		for i, t := range o.t {
			fmt.Fprintf(buf, "%-15s:%d %s", t.funcName, t.lineNo, t.msg)

			// don't add a cr on the last frame
			if i != numFrames-1 {
				buf.WriteByte('\n')
			}
		}

		// Print the error
		o.Errorf(buf.String())

		return err
	}

	return nil
}

func (o Operation) String() string {
	return o.header()
}

func (o *Operation) ID() string {
	return o.id
}

func (o *Operation) Infof(format string, args ...interface{}) {
	o.Info(fmt.Sprintf(format, args...))
}

func (o *Operation) Info(args ...interface{}) {
	msg := fmt.Sprint(args...)

	Logger.Infof("%s: %s", o.header(), msg)

	if o.Logger != nil {
		o.Logger.Info(msg)
	}
}

func (o *Operation) Debugf(format string, args ...interface{}) {
	o.Debug(fmt.Sprintf(format, args...))
}

func (o *Operation) Debug(args ...interface{}) {
	msg := fmt.Sprint(args...)

	Logger.Debugf("%s: %s", o.header(), msg)

	if o.Logger != nil {
		o.Logger.Debug(msg)
	}
}

func (o *Operation) Warnf(format string, args ...interface{}) {
	o.Warn(fmt.Sprintf(format, args...))
}

func (o *Operation) Warn(args ...interface{}) {
	msg := fmt.Sprint(args...)

	Logger.Warnf("%s: %s", o.header(), msg)

	if o.Logger != nil {
		o.Logger.Warn(msg)
	}
}

func (o *Operation) Errorf(format string, args ...interface{}) {
	o.Error(fmt.Sprintf(format, args...))
}

func (o *Operation) Error(args ...interface{}) {
	msg := fmt.Sprint(args...)

	Logger.Errorf("%s: %s", o.header(), msg)

	if o.Logger != nil {
		o.Logger.Error(msg)
	}
}

func (o *Operation) Panicf(format string, args ...interface{}) {
	o.Panic(fmt.Sprintf(format, args...))
}

func (o *Operation) Panic(args ...interface{}) {
	msg := fmt.Sprint(args...)

	Logger.Panicf("%s: %s", o.header(), msg)

	if o.Logger != nil {
		o.Logger.Panic(msg)
	}
}

func (o *Operation) Fatalf(format string, args ...interface{}) {
	o.Fatal(fmt.Sprintf(format, args...))
}

func (o *Operation) Fatal(args ...interface{}) {
	msg := fmt.Sprint(args...)

	Logger.Fatalf("%s: %s", o.header(), msg)

	if o.Logger != nil {
		o.Logger.Fatal(msg)
	}
}

func (o *Operation) newChild(ctx context.Context, msg string) Operation {
	child := newOperation(ctx, o.id, 4, msg)
	child.t = append(child.t, o.t...)
	child.Logger = o.Logger
	return child
}

func opID(opNum uint64) string {
	return fmt.Sprintf("%d.%d", opIDPrefix, opNum)
}

// NewOperation will return a new operation with operationID added as a value to the context
func NewOperation(ctx context.Context, format string, args ...interface{}) Operation {
	o := newOperation(ctx, opID(atomic.AddUint64(&opCount, 1)), 3, fmt.Sprintf(format, args...))

	frame := o.t[0]
	o.Debugf("[NewOperation] %s [%s:%d]", o.header(), frame.funcName, frame.lineNo)
	return o
}

// NewOperationWithLoggerFrom will return a new operation with operationID added as a value to the
// context and logging settings copied from the supplied operation.
//
// Deprecated: This method was added to aid in converting old code to use operation-based logging.
//             Its use almost always indicates a broken context/operation model (e.g., a context
//             being improperly stored in a struct instead of being passed between functions).
func NewOperationWithLoggerFrom(ctx context.Context, oldOp Operation, format string, args ...interface{}) Operation {
	op := NewOperation(ctx, format, args...)
	op.Logger = oldOp.Logger
	return op
}

// WithTimeout creates a new operation from parent with context.WithTimeout
func WithTimeout(parent *Operation, timeout time.Duration, format string, args ...interface{}) (Operation, context.CancelFunc) {
	ctx, cancelFunc := context.WithTimeout(parent.Context, timeout)
	op := parent.newChild(ctx, fmt.Sprintf(format, args...))

	return op, cancelFunc
}

// WithDeadline creates a new operation from parent with context.WithDeadline
func WithDeadline(parent *Operation, expiration time.Time, format string, args ...interface{}) (Operation, context.CancelFunc) {
	ctx, cancelFunc := context.WithDeadline(parent.Context, expiration)
	op := parent.newChild(ctx, fmt.Sprintf(format, args...))

	return op, cancelFunc
}

// WithCancel creates a new operation from parent with context.WithCancel
func WithCancel(parent *Operation, format string, args ...interface{}) (Operation, context.CancelFunc) {
	ctx, cancelFunc := context.WithCancel(parent.Context)
	op := parent.newChild(ctx, fmt.Sprintf(format, args...))

	return op, cancelFunc
}

// WithValue creates a new operation from parent with context.WithValue
func WithValue(parent *Operation, key, val interface{}, format string, args ...interface{}) Operation {
	ctx := context.WithValue(parent.Context, key, val)
	op := parent.newChild(ctx, fmt.Sprintf(format, args...))

	return op
}

// FromOperation creates a child operation from the one supplied
// uses the same context as the parent
func FromOperation(parent Operation, format string, args ...interface{}) Operation {
	return parent.newChild(parent.Context, fmt.Sprintf(format, args...))
}

// FromContext will return an Operation
//
// The Operation returned will be one of the following:
//   The operation in the context value
//   The operation passed as the context param
//   A new operation
func FromContext(ctx context.Context, message string, args ...interface{}) Operation {

	// do we have an operation
	if op, ok := ctx.(Operation); ok {
		return op
	}

	// do we have a context w/the op added as a value
	if op, ok := ctx.Value(OpTraceKey).(operation); ok {
		// ensure we have an initialized operation
		if op.id == "" {
			return NewOperation(ctx, message, args...)
		}
		// return an operation based off the context value
		return Operation{
			Context:   ctx,
			operation: op,
		}
	}

	op := newOperation(ctx, opID(atomic.AddUint64(&opCount, 1)), 3, fmt.Sprintf(message, args...))
	frame := op.t[0]
	Logger.Debugf("%s: [OperationFromContext] [%s:%d]", op.id, frame.funcName, frame.lineNo)

	// return the new operation
	return op
}
